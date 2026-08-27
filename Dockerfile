# ---------- build ----------
FROM golang:1.27-alpine AS builder

# ca-certificates é copiado para a imagem final: a chamada HTTPS às APIs
# externas falha sem a store de certificados.
RUN apk add --no-cache ca-certificates

WORKDIR /src

# O projeto não tem dependências externas, mas a cópia separada mantém o
# cache de camadas caso alguma seja adicionada.
COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/server ./cmd/server

# ---------- runtime ----------
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/server /server

# O Cloud Run sobrescreve PORT; o default cobre a execução local.
ENV PORT=8080
EXPOSE 8080

USER 65534:65534

ENTRYPOINT ["/server"]
