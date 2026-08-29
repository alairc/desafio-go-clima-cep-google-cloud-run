# Clima por CEP — Go + Google Cloud Run

API em Go que recebe um CEP brasileiro, identifica a cidade e devolve a
temperatura atual em Celsius, Fahrenheit e Kelvin.

**Aplicação no ar:** https://clima-cep-1016698499649.us-central1.run.app

```bash
curl https://clima-cep-1016698499649.us-central1.run.app/weather/01310100
# {"temp_C":31.1,"temp_F":87.98,"temp_K":304.1}
```

---

## Contrato

`GET /weather/{cep}` — o CEP deve ter exatamente 8 dígitos, sem hífen.

| Situação | Status | Corpo |
|---|---|---|
| Sucesso | `200` | `{"temp_C":31.1,"temp_F":87.98,"temp_K":304.1}` |
| CEP fora do formato `^[0-9]{8}$` | `422` | `{"message":"invalid zipcode"}` |
| CEP bem formado mas inexistente | `404` | `{"message":"can not find zipcode"}` |

Há também `GET /health`, que responde `{"status":"ok"}` — usado como
health check.

### Exemplos

```bash
BASE=https://clima-cep-1016698499649.us-central1.run.app

# 200 — CEP válido
curl -i $BASE/weather/01310100

# 422 — formato inválido
curl -i $BASE/weather/123

# 404 — bem formado, mas não existe
curl -i $BASE/weather/99999999
```

---

## Rodando localmente

Requisito: uma chave gratuita da [WeatherAPI](https://www.weatherapi.com/).

```bash
cp .env.example .env
# edite .env e preencha WEATHER_API_KEY
```

### Docker Compose

```bash
docker compose up --build
# ou: make docker-up
```

A API sobe em http://localhost:8080.

### Docker sem compose

```bash
docker build -t clima-cep:latest .

docker run --rm -p 8080:8080 \
  -e WEATHER_API_KEY=sua-chave \
  clima-cep:latest
```

### Direto com Go

```bash
WEATHER_API_KEY=sua-chave go run ./cmd/server
```

### Variáveis de ambiente

| Variável | Obrigatória | Default |
|---|---|---|
| `WEATHER_API_KEY` | **sim** — a aplicação falha no boot sem ela | — |
| `PORT` | não (o Cloud Run injeta) | `8080` |
| `VIACEP_BASE_URL` | não | `https://viacep.com.br` |
| `WEATHERAPI_BASE_URL` | não | `https://api.weatherapi.com/v1` |

---

## Testes

```bash
go test ./...
# ou: make test
```

Com cobertura:

```bash
make cover        # relatório por função no terminal
make cover-html   # gera coverage.html
```

Os testes não fazem chamadas de rede: as APIs externas são substituídas por
servidores `httptest`, e as camadas superiores usam dublês das interfaces
`cep.Locator` e `weather.Provider`. Cobrem as três conversões de temperatura,
o arredondamento, o contrato HTTP completo e o tratamento de erro dos dois
clientes.

---

## Como funciona

```
GET /weather/{cep}
  -> valida o formato (8 dígitos)
  -> ViaCEP:     CEP  -> cidade + estado
  -> WeatherAPI: cidade -> temperatura em Celsius
  -> converte para Fahrenheit e Kelvin
```

Sem framework: roteamento com o `ServeMux` da stdlib (padrões do Go 1.22+) e
**zero dependências externas**. A imagem final é `scratch` com 7.7 MB.

### Duas decisões que não são óbvias

**O estado vai para a WeatherAPI por extenso, não pela sigla.** A API ignora a
UF, e o resultado é silenciosamente errado:

```
Bom Jesus,RS,Brazil                 -> Bom Jesus do Acre
Bom Jesus,Rio Grande do Sul,Brazil  -> Bom Jesus do RS      (correto)
Florianopolis,SC,Brazil             -> Brazil, Indiana, EUA
Florianopolis,Santa Catarina,Brazil -> Florianópolis/SC     (correto)
```

Por isso a aplicação usa o campo `estado` da ViaCEP, e não `uf`. Os acentos
também são removidos de ambos: `Belém,Pará,Brazil` resolve para Brazil,
Indiana.

**Kelvin usa `C + 273`, não `C + 273.15`.** O enunciado especifica a fórmula
`K = C + 273`, mas o JSON de exemplo mostra `28.5 °C -> 301.65 K`, que
corresponderia a `273.15`. O próprio requisito se contradiz. A implementação
segue a fórmula textual; trocar a constante `kelvinOffset` em
`internal/temperature/temperature.go` é suficiente para adotar a outra.

---

## Deploy

A aplicação roda no Cloud Run, com a chave da WeatherAPI no Secret Manager —
não como variável de ambiente em texto claro.

```bash
gcloud run deploy clima-cep \
  --source . \
  --region us-central1 \
  --allow-unauthenticated \
  --set-secrets=WEATHER_API_KEY=weather-api-key:latest
```
