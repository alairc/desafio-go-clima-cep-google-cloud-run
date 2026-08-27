.PHONY: run test test-verbose cover lint build docker-build docker-up docker-down

run:
	go run ./cmd/server

test:
	go test ./...

test-verbose:
	go test -v ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

cover-html: cover
	go tool cover -html=coverage.out -o coverage.html

lint:
	gofmt -l .
	go vet ./...

build:
	go build -o bin/server ./cmd/server

docker-build:
	docker build -t clima-cep:latest .

docker-up:
	docker compose up --build

docker-down:
	docker compose down
