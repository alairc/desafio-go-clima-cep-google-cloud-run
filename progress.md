# Progresso — Desafio Clima por CEP (Go + Cloud Run)

> Documento de handoff para continuar o trabalho em outra sessão.
> Requisitos originais em [desafio.md](desafio.md).

**Status:** Fases 1–6 concluídas e verificadas. Faltam Fase 7 (deploy no Cloud Run) e Fase 8 (README).

---

## Estado atual do código

Módulo: `github.com/alairc/desafio-go-clima-cep-google-cloud-run`
Go 1.27 · **zero dependências externas** (apenas stdlib) · ~1.290 linhas

```
cmd/server/main.go              config, timeouts, graceful shutdown
internal/apperr/errors.go       ErrInvalidZipcode / ErrZipcodeNotFound
internal/temperature/           conversões puras + round2
internal/cep/                   client ViaCEP + interface Locator
internal/weather/               client WeatherAPI + interface Provider + BuildQuery
internal/handler/               GET /weather/{cep}, GET /health, mapa erro->status
Dockerfile                      multi-stage, imagem final scratch (7.7 MB)
docker-compose.yml  Makefile  .env.example  .dockerignore  .gitignore
```

Roteamento com `net/http` puro (padrões do `ServeMux` do Go 1.22+), sem framework.
As interfaces `cep.Locator` e `weather.Provider` existem para o handler ser testável com dublês, sem rede.

### Contrato implementado

| Situação | Status | Body |
|---|---|---|
| Sucesso | 200 | `{"temp_C":28.5,"temp_F":83.3,"temp_K":301.5}` |
| CEP não bate `^[0-9]{8}$` | 422 | `{"message":"invalid zipcode"}` |
| ViaCEP com `erro:true` ou HTTP 400 | 404 | `{"message":"can not find zipcode"}` |
| WeatherAPI código 1006 | 404 | `{"message":"can not find zipcode"}` |
| Falha de infra / chave rejeitada | 500 | `{"message":"internal server error"}` |

### Variáveis de ambiente

| Variável | Obrigatória | Default |
|---|---|---|
| `WEATHER_API_KEY` | **sim** (falha rápida no boot) | — |
| `PORT` | não (Cloud Run injeta) | `8080` |
| `VIACEP_BASE_URL` | não | `https://viacep.com.br` |
| `WEATHERAPI_BASE_URL` | não | `https://api.weatherapi.com/v1` |

---

## Verificação já realizada

`gofmt` e `go vet` limpos. `go test ./...` todo verde.

Cobertura: 100% em `Handle`, `FromCelsius`, `BuildQuery`, `RegisterRoutes`;
`Locate` 95.7%; `Current` 97.4%; total 65.2% (puxado para baixo por `run()`, que é só wiring).

**End-to-end dentro do container**, com as duas APIs apontadas para um stub local
(`--network host`, `VIACEP_BASE_URL`/`WEATHERAPI_BASE_URL` sobrescritas):

```
GET  /weather/01310100  -> 200 {"temp_C":28.5,"temp_F":83.3,"temp_K":301.5}
GET  /weather/123       -> 422 {"message":"invalid zipcode"}
GET  /weather/99999999  -> 404 {"message":"can not find zipcode"}
POST /weather/01310100  -> 405
GET  /health            -> 200 {"status":"ok"}
```

Também confirmado: a query enviada à WeatherAPI saiu como `Sao Paulo,SP,Brazil`
(acento removido); CEP inválido **não** gera chamada externa; `docker stop`
produz shutdown limpo com exit code 0.

**Nunca foi testado contra as APIs reais** — falta a chave da WeatherAPI.

---

## Decisões tomadas (e como revertê-las)

**Kelvin = C + 273.** O enunciado diz `K = C + 273`, mas o JSON de exemplo mostra
`28.5 -> 301.65`, que seria `C + 273.15`. Contradição no próprio desafio.
Seguimos a fórmula textual. Por isso nosso 200 devolve `301.5`, não `301.65`.
Trocar: constante `kelvinOffset` em `internal/temperature/temperature.go`.

**Arredondamento em 2 casas.** `28.5 * 1.8 + 32` em float64 produz
`83.30000000000001`, que vazaria cru no JSON. Função `round2`, com teste dedicado.

**Campo `erro` da ViaCEP aceita string e booleano.** A API já respondeu
`"true"` e `true` ao longo do tempo. Tipo `flexBool` com `UnmarshalJSON` custom,
com teste para cada formato.

**ViaCEP não retorna 404.** CEP inexistente vem como **HTTP 200** com
`{"erro":"true"}`; formato quebrado vem como 400. O mapeamento para 404 depende
de inspecionar o corpo, não o status.

**Acentos removidos antes de chamar a WeatherAPI.** A API erra o match com nomes
acentuados. `BuildQuery` monta `cidade-sem-acento,UF,Brazil` — a UF desambigua
homônimos entre estados (ex.: "Bom Jesus"). Tabela de diacríticos local, para não
depender de `golang.org/x/text` por causa de uma função.

**WeatherAPI 1006 vira 404, não 500.** O contrato não prevê esse caso.
Racional: o cliente pediu clima por CEP e não conseguimos entregar.
Se preferir 500, é trocar o branch em `internal/handler/weather.go` (~linha 82).

**Imagem `scratch` com `ca-certificates` copiado do builder.** Sem a store de
certificados o HTTPS para as APIs externas falha. Usuário não-root (65534).

---

## O que falta

### Fase 7 — Deploy no Cloud Run

Bloqueadores:
- **gcloud CLI não está instalado** neste ambiente (`which gcloud` -> vazio).
- **Chave da WeatherAPI ainda não obtida** (gratuita em https://www.weatherapi.com/).

Passos previstos:
1. Instalar o gcloud CLI e autenticar (`gcloud auth login`, `gcloud config set project`).
2. Habilitar as APIs `run.googleapis.com` e `cloudbuild.googleapis.com`.
3. Guardar a chave no Secret Manager (preferível) ou passar via `--set-env-vars`.
4. `gcloud run deploy clima-cep --source . --region us-central1 --allow-unauthenticated`
   (região do free tier).
5. Validar os três cenários do contrato contra a URL pública, agora com APIs reais.

### Fase 8 — README

Precisa conter, por exigência do enunciado:
- URL do sistema rodando no Cloud Run (só existe depois da Fase 7).
- Instruções para rodar a aplicação localmente via Docker.
- Instruções para rodar os testes.
- Recomendo incluir também: exemplos de `curl` dos três cenários e a nota sobre
  a divergência do Kelvin.

### Pendências menores

- `README.md` hoje tem só duas linhas de placeholder.
- O enunciado pede **repositório exclusivo do projeto**. Antes de entregar,
  avaliar remover `desafio.md` e este `progress.md`.
- Nada foi commitado além do commit inicial (`410b2c7`). Todo o código está
  no working tree, não versionado.

---

## Comandos úteis

```bash
make test          # go test ./...
make cover         # cobertura por função
make lint          # gofmt -l . && go vet ./...
make run           # go run ./cmd/server (exige WEATHER_API_KEY)
make docker-build  # docker build -t clima-cep:latest .
make docker-up     # docker compose up --build (lê .env)
```

Para rodar local sem chave real, subindo um stub das APIs externas:

```bash
docker run -d --name clima-test --network host \
  -e PORT=8081 -e WEATHER_API_KEY=fake \
  -e VIACEP_BASE_URL=http://127.0.0.1:9999 \
  -e WEATHERAPI_BASE_URL=http://127.0.0.1:9999/v1 \
  clima-cep:latest
```
