package cep

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alairc/desafio-go-clima-cep-google-cloud-run/internal/apperr"
)

func TestViaCEPLocateSucesso(t *testing.T) {
	var caminhoRecebido string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caminhoRecebido = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"cep":"01310-100","localidade":"São Paulo","uf":"SP","estado":"São Paulo"}`))
	}))
	defer srv.Close()

	client := NewViaCEP(WithBaseURL(srv.URL))

	got, err := client.Locate(context.Background(), "01310100")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	want := Address{Zipcode: "01310-100", City: "São Paulo", State: "SP", StateName: "São Paulo"}
	if got != want {
		t.Errorf("Locate = %+v, quer %+v", got, want)
	}

	if caminhoRecebido != "/ws/01310100/json/" {
		t.Errorf("caminho chamado = %q, quer /ws/01310100/json/", caminhoRecebido)
	}
}

// O campo "estado" é uma adição recente da ViaCEP. Sem ele, StateName cai na
// sigla — a WeatherAPI erra mais, mas o request não fica sem estado nenhum.
func TestViaCEPLocateSemCampoEstado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"cep":"01310-100","localidade":"São Paulo","uf":"SP"}`))
	}))
	defer srv.Close()

	got, err := NewViaCEP(WithBaseURL(srv.URL)).Locate(context.Background(), "01310100")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if got.StateName != "SP" {
		t.Errorf("StateName = %q, quer SP (fallback para a sigla)", got.StateName)
	}
}

// A ViaCEP responde 200 com {"erro": ...} para CEP inexistente, e já usou
// tanto string quanto booleano nesse campo.
func TestViaCEPLocateNaoEncontrado(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"erro como string", `{"erro": "true"}`},
		{"erro como booleano", `{"erro": true}`},
		{"resposta sem localidade", `{"cep":"99999-999","localidade":"","uf":""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			_, err := NewViaCEP(WithBaseURL(srv.URL)).Locate(context.Background(), "99999999")
			if !errors.Is(err, apperr.ErrZipcodeNotFound) {
				t.Errorf("erro = %v, quer ErrZipcodeNotFound", err)
			}
		})
	}
}

func TestViaCEPLocateStatus400ViraNaoEncontrado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := NewViaCEP(WithBaseURL(srv.URL)).Locate(context.Background(), "00000000")
	if !errors.Is(err, apperr.ErrZipcodeNotFound) {
		t.Errorf("erro = %v, quer ErrZipcodeNotFound", err)
	}
}

func TestViaCEPLocateFalhasDeInfra(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "erro 500 do upstream",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
		{
			name: "json malformado",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"localidade":`))
			},
		},
		{
			name: "campo erro com valor inesperado",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"erro":"talvez"}`))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			_, err := NewViaCEP(WithBaseURL(srv.URL)).Locate(context.Background(), "01310100")
			if err == nil {
				t.Fatal("esperava erro, obteve nil")
			}

			// Falha de infraestrutura não deve ser confundida com "não encontrado".
			if errors.Is(err, apperr.ErrZipcodeNotFound) {
				t.Errorf("erro = %v, não deveria ser ErrZipcodeNotFound", err)
			}
		})
	}
}

func TestViaCEPLocateRespeitaContextoCancelado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := NewViaCEP(WithBaseURL(srv.URL)).Locate(ctx, "01310100"); err == nil {
		t.Fatal("esperava erro com contexto cancelado")
	}
}
