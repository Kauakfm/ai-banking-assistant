.PHONY: build run test clean docker-up docker-down test-bfa test-agent test-all

BFA_DIR := ./bfa-go
AGENT_DIR := ./agent-python


test-bfa:
	@echo "Rodando testes do BFA (Go)..."
	@cd $(BFA_DIR) && go test -v -race ./...

test-agent:
	@echo "Rodando testes do Agent (Python)..."
	@cd $(AGENT_DIR) && python -m pytest tests/ -v --tb=short

test-all: test-bfa test-agent
	@echo "Todos os testes executados."


build-bfa:
	@echo "Compilando BFA localmente..."
	@cd $(BFA_DIR) && go build -o bin/api ./cmd/api


docker-up:
	@echo "Subindo toda a infraestrutura (BFA + Agent + Prometheus + LangFuse)..."
	docker-compose up --build -d

docker-down:
	@echo "Derrubando a infraestrutura..."
	docker-compose down

docker-logs:
	docker-compose logs -f

docker-restart-agent:
	@echo "Recriando agent-python (recarrega .env)..."
	docker-compose up -d --force-recreate agent-python