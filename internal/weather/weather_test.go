package weather

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWeatherAPICurrentSucesso(t *testing.T) {
	var recebido struct {
		key string
		q   string
		aqi string
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recebido.key = r.URL.Query().Get("key")
		recebido.q = r.URL.Query().Get("q")
		recebido.aqi = r.URL.Query().Get("aqi")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"location":{"name":"Sao Paulo"},"current":{"temp_c":28.5}}`))
	}))
	defer srv.Close()

	client := NewWeatherAPI("chave-secreta", WithBaseURL(srv.URL))

	got, err := client.Current(context.Background(), "Sao Paulo,SP,Brazil")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if got.TempC != 28.5 {
		t.Errorf("TempC = %v, quer 28.5", got.TempC)
	}

	if recebido.key != "chave-secreta" {
		t.Errorf("key = %q, quer chave-secreta", recebido.key)
	}

	if recebido.q != "Sao Paulo,SP,Brazil" {
		t.Errorf("q = %q, quer Sao Paulo,SP,Brazil", recebido.q)
	}

	if recebido.aqi != "no" {
		t.Errorf("aqi = %q, quer no", recebido.aqi)
	}
}

func TestWeatherAPICurrentLocalidadeNaoEncontrada(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":1006,"message":"No matching location found."}}`))
	}))
	defer srv.Close()

	_, err := NewWeatherAPI("k", WithBaseURL(srv.URL)).Current(context.Background(), "Nowhere")
	if !errors.Is(err, ErrLocationNotFound) {
		t.Errorf("erro = %v, quer ErrLocationNotFound", err)
	}
}

func TestWeatherAPICurrentChaveRejeitada(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"code":2006,"message":"API key is invalid."}}`))
		}))

		_, err := NewWeatherAPI("ruim", WithBaseURL(srv.URL)).Current(context.Background(), "Sao Paulo")
		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("status %d: erro = %v, quer ErrUnauthorized", status, err)
		}

		srv.Close()
	}
}

func TestWeatherAPICurrentFalhasDeInfra(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "erro 500 do upstream",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":{"code":9999,"message":"internal"}}`))
			},
		},
		{
			name: "json malformado",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"current":`))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			_, err := NewWeatherAPI("k", WithBaseURL(srv.URL)).Current(context.Background(), "Sao Paulo")
			if err == nil {
				t.Fatal("esperava erro, obteve nil")
			}

			if errors.Is(err, ErrLocationNotFound) {
				t.Errorf("erro = %v, não deveria ser ErrLocationNotFound", err)
			}
		})
	}
}

func TestWeatherAPICurrentRespeitaContextoCancelado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := NewWeatherAPI("k", WithBaseURL(srv.URL)).Current(ctx, "Sao Paulo"); err == nil {
		t.Fatal("esperava erro com contexto cancelado")
	}
}

func TestBuildQuery(t *testing.T) {
	tests := []struct {
		name  string
		city  string
		state string
		want  string
	}{
		{"remove acentos da cidade", "São Paulo", "São Paulo", "Sao Paulo,Sao Paulo,Brazil"},
		// Sem remover o acento do estado, a WeatherAPI resolve para Brazil, Indiana.
		{"remove acentos do estado", "Belém", "Pará", "Belem,Para,Brazil"},
		{"cedilha", "Poções", "Bahia", "Pocoes,Bahia,Brazil"},
		{"acento no estado e na cidade", "Goiânia", "Goiás", "Goiania,Goias,Brazil"},
		{"estado composto", "Vitória", "Espírito Santo", "Vitoria,Espirito Santo,Brazil"},
		{"til no estado", "São Luís", "Maranhão", "Sao Luis,Maranhao,Brazil"},
		{"sem acento passa intacto", "Curitiba", "Paraná", "Curitiba,Parana,Brazil"},
		// O fallback da ViaCEP entrega a sigla; precisa continuar montando query.
		{"estado como sigla", "Recife", "PE", "Recife,PE,Brazil"},
		{"estado ausente", "Recife", "", "Recife,Brazil"},
		{"espacos ao redor", "  Belém  ", "  Pará  ", "Belem,Para,Brazil"},
		{"cidade vazia", "", "Bahia", "Bahia,Brazil"},
		{"tudo vazio", "", "", "Brazil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildQuery(tt.city, tt.state); got != tt.want {
				t.Errorf("BuildQuery(%q, %q) = %q, quer %q", tt.city, tt.state, got, tt.want)
			}
		})
	}
}
