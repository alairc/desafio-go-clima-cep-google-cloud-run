// Command server sobe a API de clima por CEP.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alairc/desafio-go-clima-cep-google-cloud-run/internal/cep"
	"github.com/alairc/desafio-go-clima-cep-google-cloud-run/internal/handler"
	"github.com/alairc/desafio-go-clima-cep-google-cloud-run/internal/weather"
)

// shutdownTimeout é a janela para drenar requisições em voo após o SIGTERM.
// O Cloud Run concede ~10s entre o sinal e o encerramento da instância.
const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("aplicação encerrada com erro", "erro", err)
		os.Exit(1)
	}
}

type config struct {
	port           string
	weatherAPIKey  string
	viaCEPBaseURL  string
	weatherBaseURL string
}

func loadConfig() (config, error) {
	cfg := config{
		// O Cloud Run injeta PORT e exige que o container escute nela.
		port:           envOrDefault("PORT", "8080"),
		weatherAPIKey:  os.Getenv("WEATHER_API_KEY"),
		viaCEPBaseURL:  envOrDefault("VIACEP_BASE_URL", cep.DefaultBaseURL),
		weatherBaseURL: envOrDefault("WEATHERAPI_BASE_URL", weather.DefaultBaseURL),
	}

	// Falha rápida: sem a chave, todo request viraria 500.
	if cfg.weatherAPIKey == "" {
		return config{}, errors.New("variável de ambiente WEATHER_API_KEY é obrigatória")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func run(logger *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	locator := cep.NewViaCEP(cep.WithBaseURL(cfg.viaCEPBaseURL))
	provider := weather.NewWeatherAPI(cfg.weatherAPIKey, weather.WithBaseURL(cfg.weatherBaseURL))

	mux := http.NewServeMux()
	handler.NewWeather(locator, provider, logger).RegisterRoutes(mux)

	server := &http.Server{
		Addr:              ":" + cfg.port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Cancela ao receber SIGINT (Ctrl+C local) ou SIGTERM (Cloud Run).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	erros := make(chan error, 1)

	go func() {
		logger.Info("servidor escutando", "porta", cfg.port)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			erros <- fmt.Errorf("listen: %w", err)
		}
	}()

	select {
	case err := <-erros:
		return err
	case <-ctx.Done():
		logger.Info("sinal recebido, encerrando com graça", "timeout", shutdownTimeout.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	logger.Info("servidor encerrado")

	return nil
}
