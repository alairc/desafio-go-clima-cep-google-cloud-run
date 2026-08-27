package main

import (
	"testing"

	"github.com/alairc/desafio-go-clima-cep-google-cloud-run/internal/cep"
	"github.com/alairc/desafio-go-clima-cep-google-cloud-run/internal/weather"
)

func TestLoadConfigExigeChaveDaWeatherAPI(t *testing.T) {
	t.Setenv("WEATHER_API_KEY", "")

	if _, err := loadConfig(); err == nil {
		t.Fatal("esperava erro sem WEATHER_API_KEY")
	}
}

func TestLoadConfigAplicaDefaults(t *testing.T) {
	t.Setenv("WEATHER_API_KEY", "chave")
	t.Setenv("PORT", "")
	t.Setenv("VIACEP_BASE_URL", "")
	t.Setenv("WEATHERAPI_BASE_URL", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if cfg.port != "8080" {
		t.Errorf("port = %q, quer 8080", cfg.port)
	}

	if cfg.viaCEPBaseURL != cep.DefaultBaseURL {
		t.Errorf("viaCEPBaseURL = %q, quer %q", cfg.viaCEPBaseURL, cep.DefaultBaseURL)
	}

	if cfg.weatherBaseURL != weather.DefaultBaseURL {
		t.Errorf("weatherBaseURL = %q, quer %q", cfg.weatherBaseURL, weather.DefaultBaseURL)
	}
}

// O Cloud Run injeta PORT e o container precisa obedecer.
func TestLoadConfigRespeitaPortDoAmbiente(t *testing.T) {
	t.Setenv("WEATHER_API_KEY", "chave")
	t.Setenv("PORT", "9090")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if cfg.port != "9090" {
		t.Errorf("port = %q, quer 9090", cfg.port)
	}
}
