// Package weather consulta a temperatura atual de uma localidade na WeatherAPI.
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

// DefaultBaseURL é o endpoint público da WeatherAPI.
const DefaultBaseURL = "https://api.weatherapi.com/v1"

// codeLocationNotFound é o código da WeatherAPI para "No matching location found".
const codeLocationNotFound = 1006

var (
	// ErrLocationNotFound indica que a WeatherAPI não reconheceu a localidade.
	ErrLocationNotFound = errors.New("localidade não encontrada na weatherapi")

	// ErrUnauthorized indica chave de API ausente, inválida ou sem cota.
	ErrUnauthorized = errors.New("chave da weatherapi rejeitada")
)

// Current é a medição atual de uma localidade.
type Current struct {
	TempC float64
}

// Provider consulta a temperatura atual de uma localidade. Existe para permitir
// substituição por dublê nos testes das camadas superiores.
type Provider interface {
	Current(ctx context.Context, query string) (Current, error)
}

// WeatherAPI é a implementação de Provider sobre a WeatherAPI.
type WeatherAPI struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Option customiza a construção do cliente.
type Option func(*WeatherAPI)

// WithBaseURL troca o endpoint da WeatherAPI (usado nos testes).
func WithBaseURL(baseURL string) Option {
	return func(w *WeatherAPI) { w.baseURL = strings.TrimSuffix(baseURL, "/") }
}

// NewWeatherAPI constrói o cliente com timeout padrão de 5s.
func NewWeatherAPI(apiKey string, opts ...Option) *WeatherAPI {
	client := &WeatherAPI{
		baseURL:    DefaultBaseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

type weatherAPIResponse struct {
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Current consulta a temperatura atual. Retorna ErrLocationNotFound quando a
// WeatherAPI não reconhece a localidade informada.
func (w *WeatherAPI) Current(ctx context.Context, query string) (Current, error) {
	params := url.Values{}
	params.Set("key", w.apiKey)
	params.Set("q", query)
	params.Set("aqi", "no")

	endpoint := fmt.Sprintf("%s/current.json?%s", w.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Current{}, fmt.Errorf("weatherapi: montando requisição: %w", err)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return Current{}, fmt.Errorf("weatherapi: chamando api: %w", err)
	}
	defer resp.Body.Close()

	var body weatherAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Current{}, fmt.Errorf("weatherapi: decodificando resposta (status %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode != http.StatusOK {
		switch {
		case body.Error.Code == codeLocationNotFound:
			return Current{}, ErrLocationNotFound
		case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
			return Current{}, fmt.Errorf("%w: %s", ErrUnauthorized, body.Error.Message)
		default:
			return Current{}, fmt.Errorf("weatherapi: status %d: %s", resp.StatusCode, body.Error.Message)
		}
	}

	return Current{TempC: body.Current.TempC}, nil
}

// BuildQuery monta o termo de busca da WeatherAPI a partir de uma localidade
// brasileira, no formato "Cidade,Estado,Brazil".
//
// Duas exigências da API, ambas verificadas contra ela:
//
//   - Acentos são removidos. Com eles o match degrada de formas surpreendentes:
//     "Belém,Pará,Brazil" resolve para Brazil, Indiana (EUA).
//   - O estado vai por extenso, não pela sigla. A WeatherAPI ignora a UF, então
//     "Bom Jesus,RS,Brazil" cai em Bom Jesus do Acre; já
//     "Bom Jesus,Rio Grande do Sul,Brazil" acerta. Por isso o chamador passa
//     cep.Address.StateName, não .State.
func BuildQuery(city, state string) string {
	parts := make([]string, 0, 3)

	if city = strings.TrimSpace(city); city != "" {
		parts = append(parts, removeDiacritics(city))
	}

	if state = strings.TrimSpace(state); state != "" {
		parts = append(parts, removeDiacritics(state))
	}

	parts = append(parts, "Brazil")

	return strings.Join(parts, ",")
}

// diacritics mapeia as vogais acentuadas e cedilha usadas em português (mais
// ñ) para o equivalente ASCII. Mantido como tabela local para o projeto não
// depender de golang.org/x/text por causa de uma única função.
var diacritics = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n',
}

func removeDiacritics(text string) string {
	var out strings.Builder
	out.Grow(len(text))

	for _, r := range text {
		lower := unicode.ToLower(r)

		replacement, ok := diacritics[lower]
		if !ok {
			out.WriteRune(r)
			continue
		}

		if lower != r {
			replacement = unicode.ToUpper(replacement)
		}

		out.WriteRune(replacement)
	}

	return out.String()
}

// compile-time: WeatherAPI satisfaz Provider.
var _ Provider = (*WeatherAPI)(nil)
