# BFA Go — Backend for Agent

Backend em Go que orquestra chamadas para Profile API, Transactions API e o Serviço de Agente de IA, com padrões de resiliência de nível produção.

## Arquitetura Hexagonal (Ports & Adapters)

```
cmd/
├── server/              → Ponto de entrada do BFA
└── mockserver/          → Mock das APIs externas (Profile, Transactions, Agent)

internal/
├── core/                → Núcleo da aplicação (agnóstico a tecnologia)
│   ├── domain/          → Modelos de domínio e erros
│   ├── ports/           → Interfaces (driver ports + driven ports)
│   └── services/        → Lógica de negócio (implementa driver ports)
├── handlers/            → Driver adapters (HTTP → core)
├── repositories/        → Driven adapters (core → APIs externas)
└── infra/               → Infraestrutura transversal
    ├── config/          → Configuração via variáveis de ambiente
    ├── observabilidade/ → Logger (zap), métricas (Prometheus), tracing (OTel)
    ├── resiliencia/     → Circuit breaker, retry, bulkhead
    └── cache/           → Cache em memória com TTL
```

## Endpoints

| Método | Rota | Descrição |
|--------|------|-----------|
| GET | `/v1/assistant/{customerId}` | Consulta assistente para o cliente |
| GET | `/healthz` | Liveness check |
| GET | `/readyz` | Readiness check |
| GET | `/metrics` | Métricas Prometheus |

## Como Rodar

### Pré-requisitos

- Go 1.22+

### Rodar localmente

```bash
# Terminal 1 — Mock server (simula Profile API, Transactions API e Agent Service)
cd bfa-go
go run ./cmd/mockserver

# Terminal 2 — BFA server
cd bfa-go
go run ./cmd/server
```

### Testar o endpoint

```bash
curl http://localhost:8080/v1/assistant/cliente-001
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/metrics
```

### Rodar testes

```bash
go test ./... -v -cover -race
```

## Variáveis de Ambiente

| Variável | Padrão | Descrição |
|----------|--------|-----------|
| `PORTA` | `8080` | Porta do BFA |
| `PROFILE_API_URL` | `http://localhost:8081` | URL da Profile API |
| `TRANSACTIONS_API_URL` | `http://localhost:8081` | URL da Transactions API |
| `AGENT_SERVICE_URL` | `http://localhost:8081` | URL do Agent Service |
| `CACHE_TTL` | `5m` | TTL do cache |
| `REQUEST_TIMEOUT` | `30s` | Timeout por requisição |
| `MAX_CONCORRENCIA` | `100` | Limite de requisições simultâneas (bulkhead) |
| `RETRY_MAX_TENTATIVAS` | `3` | Máximo de tentativas no retry |
| `CIRCUIT_BREAKER_LIMITE` | `5` | Falhas consecutivas para abrir circuit breaker |
| `LOG_LEVEL` | `info` | Nível de log (debug, info, warn, error) |

## Padrões de Resiliência

- **Circuit Breaker** (sony/gobreaker) — Abre após N falhas consecutivas, evita cascata
- **Retry com Exponential Backoff + Jitter** — Retenta erros transitórios sem thundering herd
- **Bulkhead** (semáforo) — Limita concorrência para proteger recursos downstream
- **Timeout com Context** — Cancelamento propagado via `context.Context`
- **Cache com TTL** — Reduz carga nos serviços externos
- **Graceful Shutdown** — Finaliza conexões ativas antes de desligar

## Decisões Arquiteturais

- **Chi router** — Leve, idiomático, compatível com `net/http`
- **Zap logger** — Logs estruturados em JSON, alta performance
- **Prometheus** — Métricas padronizadas para observabilidade
- **OpenTelemetry** — Tracing distribuído
- **Chamadas concorrentes** — Profile e Transactions são buscados em paralelo via goroutines
- **Fallback no agente** — Se o Agent Service falhar, retorna resposta parcial com aviso

## Trade-offs

- Cache em memória (não compartilhado entre instâncias) → Em produção, usar Redis/ElastiCache
- Mock server local → Em produção, APIs reais via service mesh
- Stdout trace exporter → Em produção, OTLP exporter para Jaeger/Tempo
- Sem autenticação → Em produção, JWT/OAuth2 via API Gateway
