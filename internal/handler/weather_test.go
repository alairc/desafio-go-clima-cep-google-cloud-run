package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alairc/desafio-go-clima-cep-google-cloud-run/internal/apperr"
	"github.com/alairc/desafio-go-clima-cep-google-cloud-run/internal/cep"
	"github.com/alairc/desafio-go-clima-cep-google-cloud-run/internal/weather"
)

// locatorFake substitui a ViaCEP nos testes do handler.
type locatorFake struct {
	address        cep.Address
	err            error
	zipcodeChamado string
}

func (l *locatorFake) Locate(_ context.Context, zipcode string) (cep.Address, error) {
	l.zipcodeChamado = zipcode
	return l.address, l.err
}

// providerFake substitui a WeatherAPI nos testes do handler.
type providerFake struct {
	current      weather.Current
	err          error
	queryChamada string
	chamado      bool
}

func (p *providerFake) Current(_ context.Context, query string) (weather.Current, error) {
	p.chamado = true
	p.queryChamada = query

	return p.current, p.err
}

// newTestServer monta um mux com o handler e dublês, silenciando o log.
func newTestServer(locator cep.Locator, provider weather.Provider) *http.ServeMux {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	NewWeather(locator, provider, logger).RegisterRoutes(mux)

	return mux
}

func do(t *testing.T, mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	return rec
}

// Cenário 1 do contrato: 200 OK com as três escalas.
func TestHandleSucesso(t *testing.T) {
	locator := &locatorFake{address: cep.Address{Zipcode: "01310-100", City: "São Paulo", State: "SP"}}
	provider := &providerFake{current: weather.Current{TempC: 28.5}}

	rec := do(t, newTestServer(locator, provider), "/weather/01310100")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, quer 200. corpo: %s", rec.Code, rec.Body.String())
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}

	var got temperatureResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("corpo não é o JSON esperado: %v (%s)", err, rec.Body.String())
	}

	want := temperatureResponse{TempC: 28.5, TempF: 83.3, TempK: 301.5}
	if got != want {
		t.Errorf("corpo = %+v, quer %+v", got, want)
	}

	if locator.zipcodeChamado != "01310100" {
		t.Errorf("cep repassado = %q, quer 01310100", locator.zipcodeChamado)
	}

	// Confirma que a localidade acentuada foi normalizada antes da consulta.
	if provider.queryChamada != "Sao Paulo,SP,Brazil" {
		t.Errorf("query = %q, quer Sao Paulo,SP,Brazil", provider.queryChamada)
	}
}

// Cenário 2 do contrato: 422 invalid zipcode.
func TestHandleCepInvalido(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"menos de 8 digitos", "/weather/1234567"},
		{"mais de 8 digitos", "/weather/123456789"},
		{"com hifen", "/weather/01310-10"},
		{"com letras", "/weather/0131010a"},
		{"tudo letras", "/weather/abcdefgh"},
		{"com espaco codificado", "/weather/0131010%20"},
		{"apenas um digito", "/weather/0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locator := &locatorFake{}
			provider := &providerFake{}

			rec := do(t, newTestServer(locator, provider), tt.path)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, quer 422", rec.Code)
			}

			if msg := decodeMessage(t, rec.Body.Bytes()); msg != "invalid zipcode" {
				t.Errorf("mensagem = %q, quer invalid zipcode", msg)
			}

			// Formato inválido não deve gerar chamada externa.
			if locator.zipcodeChamado != "" {
				t.Errorf("locator foi chamado com %q; não deveria ter sido chamado", locator.zipcodeChamado)
			}

			if provider.chamado {
				t.Error("provider foi chamado; não deveria ter sido")
			}
		})
	}
}

// Cenário 3 do contrato: 404 can not find zipcode.
func TestHandleCepNaoEncontrado(t *testing.T) {
	locator := &locatorFake{err: apperr.ErrZipcodeNotFound}
	provider := &providerFake{}

	rec := do(t, newTestServer(locator, provider), "/weather/99999999")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quer 404", rec.Code)
	}

	if msg := decodeMessage(t, rec.Body.Bytes()); msg != "can not find zipcode" {
		t.Errorf("mensagem = %q, quer can not find zipcode", msg)
	}

	if provider.chamado {
		t.Error("provider foi chamado apesar do cep não existir")
	}
}

// A WeatherAPI não reconhecer a cidade resolvida também vira 404.
func TestHandleLocalidadeSemClima(t *testing.T) {
	locator := &locatorFake{address: cep.Address{City: "Cidade Fantasma", State: "XX"}}
	provider := &providerFake{err: weather.ErrLocationNotFound}

	rec := do(t, newTestServer(locator, provider), "/weather/01310100")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, quer 404", rec.Code)
	}

	if msg := decodeMessage(t, rec.Body.Bytes()); msg != "can not find zipcode" {
		t.Errorf("mensagem = %q, quer can not find zipcode", msg)
	}
}

// Falhas de infraestrutura viram 500, não 404/422.
func TestHandleFalhasDeInfra(t *testing.T) {
	falhaGenerica := errors.New("conexão recusada")

	tests := []struct {
		name     string
		locator  *locatorFake
		provider *providerFake
	}{
		{
			name:     "viacep indisponivel",
			locator:  &locatorFake{err: falhaGenerica},
			provider: &providerFake{},
		},
		{
			name:     "weatherapi indisponivel",
			locator:  &locatorFake{address: cep.Address{City: "São Paulo", State: "SP"}},
			provider: &providerFake{err: falhaGenerica},
		},
		{
			name:     "chave da weatherapi rejeitada",
			locator:  &locatorFake{address: cep.Address{City: "São Paulo", State: "SP"}},
			provider: &providerFake{err: weather.ErrUnauthorized},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newTestServer(tt.locator, tt.provider), "/weather/01310100")

			if rec.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, quer 500", rec.Code)
			}

			// A mensagem interna não deve vazar para o cliente.
			if body := rec.Body.String(); body == "" {
				t.Error("corpo vazio no 500")
			} else if msg := decodeMessage(t, rec.Body.Bytes()); msg != "internal server error" {
				t.Errorf("mensagem = %q, quer internal server error", msg)
			}
		})
	}
}

func TestHandleTemperaturaNegativa(t *testing.T) {
	locator := &locatorFake{address: cep.Address{City: "Urupema", State: "SC"}}
	provider := &providerFake{current: weather.Current{TempC: -5.2}}

	rec := do(t, newTestServer(locator, provider), "/weather/88625000")

	var got temperatureResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("corpo inválido: %v", err)
	}

	want := temperatureResponse{TempC: -5.2, TempF: 22.64, TempK: 267.8}
	if got != want {
		t.Errorf("corpo = %+v, quer %+v", got, want)
	}
}

func TestHealth(t *testing.T) {
	rec := do(t, newTestServer(&locatorFake{}, &providerFake{}), "/health")

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, quer 200", rec.Code)
	}
}

func TestRotaInexistente(t *testing.T) {
	rec := do(t, newTestServer(&locatorFake{}, &providerFake{}), "/nao-existe")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, quer 404", rec.Code)
	}
}

func TestMetodoNaoPermitido(t *testing.T) {
	mux := newTestServer(&locatorFake{}, &providerFake{})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/weather/01310100", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, quer 405", rec.Code)
	}
}

func decodeMessage(t *testing.T, body []byte) string {
	t.Helper()

	var resp errorResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("corpo de erro inválido: %v (%s)", err, body)
	}

	return resp.Message
}
