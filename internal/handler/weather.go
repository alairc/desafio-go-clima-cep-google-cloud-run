// Package handler expõe o endpoint HTTP do desafio e traduz os erros de
// domínio para os status codes do contrato.
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/alairc/desafio-go-clima-cep-google-cloud-run/internal/apperr"
	"github.com/alairc/desafio-go-clima-cep-google-cloud-run/internal/cep"
	"github.com/alairc/desafio-go-clima-cep-google-cloud-run/internal/temperature"
	"github.com/alairc/desafio-go-clima-cep-google-cloud-run/internal/weather"
)

// zipcodePattern aceita exatamente 8 dígitos, conforme o requisito de entrada.
var zipcodePattern = regexp.MustCompile(`^[0-9]{8}$`)

// Weather orquestra CEP -> localidade -> temperatura -> conversão.
type Weather struct {
	locator  cep.Locator
	provider weather.Provider
	logger   *slog.Logger
}

// NewWeather constrói o handler. Um logger nil cai no default do slog.
func NewWeather(locator cep.Locator, provider weather.Provider, logger *slog.Logger) *Weather {
	if logger == nil {
		logger = slog.Default()
	}

	return &Weather{locator: locator, provider: provider, logger: logger}
}

// temperatureResponse é o corpo de sucesso definido no contrato.
type temperatureResponse struct {
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

// errorResponse é o corpo dos cenários de falha.
type errorResponse struct {
	Message string `json:"message"`
}

// RegisterRoutes registra os endpoints da aplicação no mux.
func (h *Weather) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /weather/{cep}", h.Handle)
	mux.HandleFunc("GET /health", handleHealth)
}

// Handle responde GET /weather/{cep}.
func (h *Weather) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	zipcode := r.PathValue("cep")

	if !zipcodePattern.MatchString(zipcode) {
		writeError(w, http.StatusUnprocessableEntity, apperr.ErrInvalidZipcode.Error())
		return
	}

	address, err := h.locator.Locate(ctx, zipcode)
	if err != nil {
		if errors.Is(err, apperr.ErrZipcodeNotFound) {
			writeError(w, http.StatusNotFound, apperr.ErrZipcodeNotFound.Error())
			return
		}

		h.logger.ErrorContext(ctx, "falha ao localizar cep", "zipcode", zipcode, "erro", err)
		writeError(w, http.StatusInternalServerError, "internal server error")

		return
	}

	current, err := h.provider.Current(ctx, weather.BuildQuery(address.City, address.State))
	if err != nil {
		// A WeatherAPI não reconhecer a localidade resolvida é, na prática, um
		// CEP para o qual não conseguimos entregar clima. O contrato só prevê
		// 404 para esse caso, então reaproveitamos a mensagem.
		if errors.Is(err, weather.ErrLocationNotFound) {
			h.logger.WarnContext(ctx, "localidade sem clima na weatherapi", "zipcode", zipcode, "cidade", address.City)
			writeError(w, http.StatusNotFound, apperr.ErrZipcodeNotFound.Error())

			return
		}

		h.logger.ErrorContext(ctx, "falha ao consultar clima", "cidade", address.City, "erro", err)
		writeError(w, http.StatusInternalServerError, "internal server error")

		return
	}

	temps := temperature.FromCelsius(current.TempC)

	writeJSON(w, http.StatusOK, temperatureResponse{
		TempC: temps.Celsius,
		TempF: temps.Fahrenheit,
		TempK: temps.Kelvin,
	})
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// O status já foi escrito; só resta registrar.
		slog.Error("falha ao serializar resposta", "erro", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Message: message})
}
