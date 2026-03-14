.PHONY: build run test clean docker-up docker-down

BFA_DIR := ./bfa-go

test-bfa:
	@echo "Rodando testes do BFA..."
	@cd $(BFA_DIR) && go test -v -race ./...

build-bfa:
	@echo "Compilando BFA localmente..."
	@cd $(BFA_DIR) && go build -o bin/api ./cmd/api

docker-up:
	@echo "Subindo toda a infraestrutura (BFA + Agent + Prometheus)..."
	docker-compose up --build -d

docker-down:
	@echo "Derrubando a infraestrutura..."
	docker-compose down

docker-logs:
	docker-compose logs -f bfa-go