// Package cep resolve um CEP em uma localidade usando a API ViaCEP.
package cep

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alairc/desafio-go-clima-cep-google-cloud-run/internal/apperr"
)

// DefaultBaseURL é o endpoint público da ViaCEP.
const DefaultBaseURL = "https://viacep.com.br"

// Address é a localidade resolvida a partir de um CEP.
//
// State e StateName carregam a mesma informação em formatos diferentes porque
// cada consumidor precisa de um: a sigla é o identificador canônico, mas a
// WeatherAPI só desambigua cidades homônimas com o nome por extenso.
type Address struct {
	Zipcode   string
	City      string
	State     string
	StateName string
}

// Locator resolve um CEP em uma localidade. Existe para permitir substituição
// por dublê nos testes das camadas superiores.
type Locator interface {
	Locate(ctx context.Context, zipcode string) (Address, error)
}

// ViaCEP é a implementação de Locator sobre a API ViaCEP.
type ViaCEP struct {
	baseURL    string
	httpClient *http.Client
}

// Option customiza a construção do cliente.
type Option func(*ViaCEP)

// WithBaseURL troca o endpoint da ViaCEP (usado nos testes).
func WithBaseURL(baseURL string) Option {
	return func(v *ViaCEP) { v.baseURL = strings.TrimSuffix(baseURL, "/") }
}

// NewViaCEP constrói o cliente com timeout padrão de 5s.
func NewViaCEP(opts ...Option) *ViaCEP {
	client := &ViaCEP{
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// viaCEPResponse é o subconjunto da resposta da ViaCEP que nos interessa.
type viaCEPResponse struct {
	Cep        string   `json:"cep"`
	Localidade string   `json:"localidade"`
	UF         string   `json:"uf"`
	Estado     string   `json:"estado"`
	Erro       flexBool `json:"erro"`
}

// flexBool aceita tanto `true` quanto `"true"`: a ViaCEP já respondeu nos dois
// formatos para o campo `erro` ao longo do tempo.
type flexBool bool

func (f *flexBool) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(string(data), `"`)
	if raw == "" || raw == "null" {
		*f = false
		return nil
	}

	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return fmt.Errorf("campo erro inesperado %q: %w", raw, err)
	}

	*f = flexBool(parsed)

	return nil
}

// Locate consulta a ViaCEP. Retorna apperr.ErrZipcodeNotFound quando o CEP
// não existe na base.
func (v *ViaCEP) Locate(ctx context.Context, zipcode string) (Address, error) {
	url := fmt.Sprintf("%s/ws/%s/json/", v.baseURL, zipcode)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Address{}, fmt.Errorf("viacep: montando requisição: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return Address{}, fmt.Errorf("viacep: chamando api: %w", err)
	}
	defer resp.Body.Close()

	// A ViaCEP responde 400 para CEP malformado. O handler já validou o
	// formato, então aqui isso equivale a "não encontrado".
	if resp.StatusCode == http.StatusBadRequest {
		return Address{}, apperr.ErrZipcodeNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return Address{}, fmt.Errorf("viacep: status inesperado %d", resp.StatusCode)
	}

	var body viaCEPResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Address{}, fmt.Errorf("viacep: decodificando resposta: %w", err)
	}

	// CEP inexistente vem como HTTP 200 com {"erro": "true"} — o status code
	// sozinho não distingue sucesso de falha.
	if bool(body.Erro) || body.Localidade == "" {
		return Address{}, apperr.ErrZipcodeNotFound
	}

	// O campo "estado" é uma adição recente da ViaCEP. Se vier vazio, a sigla
	// é o melhor que temos — pior para o match, mas melhor que nada.
	stateName := body.Estado
	if stateName == "" {
		stateName = body.UF
	}

	return Address{
		Zipcode:   body.Cep,
		City:      body.Localidade,
		State:     body.UF,
		StateName: stateName,
	}, nil
}

// compile-time: ViaCEP satisfaz Locator.
var _ Locator = (*ViaCEP)(nil)
