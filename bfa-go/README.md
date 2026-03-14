# BFA-Go — Backend for Frontend (Agente de IA Bancário)

Serviço HTTP em Go que orquestra chamadas para a Profile API, Transactions API e o Agente de IA Python, expondo três endpoints de negócio com resiliência de nível produção: circuit breaker, retry com backoff exponencial, bulkhead, timeout por contexto e cache com TTL.

---

## Estrutura do Projeto

```
bfa-go/
├── cmd/api/main.go                  ponto de entrada, wiring, graceful shutdown
├── config/config.go                 configuração via variáveis de ambiente
├── internal/
│   ├── domain/models.go             modelos de domínio (structs, sem dependências externas)
│   ├── client/
│   │   ├── agent.go                 cliente HTTP para o Agente Python (circuit breaker)
│   │   ├── profile.go               cliente HTTP para a Profile API (circuit breaker + retry + mock)
│   │   └── transaction.go           cliente HTTP para a Transactions API (circuit breaker + retry + mock)
│   └── handler/
│       ├── assistant.go             POST /v1/assistant/{customerId}
│       ├── profile.go               GET  /v1/customers/{customerId}/profile
│       └── transaction.go           GET  /v1/customers/{customerId}/transactions
└── pkg/
    ├── cache/cache.go               wrapper genérico sobre go-cache (Get/Set/TTL)
    ├── errors/errors.go             erros sentinela + helpers WriteJSON/WriteError/HandleError
    ├── logger/logger.go             logger JSON estruturado (log/slog)
    ├── middleware/middleware.go      métricas Prometheus, tracing OTel, logging, recovery
    ├── resilience/resilience.go     circuit breaker, retry, bulkhead
    └── tracing/tracing.go           inicialização do OpenTelemetry TracerProvider
```

---

## Pré-requisitos

- Go 1.25+
- Docker e Docker Compose (opcional)

---

## Como Executar

### Localmente

```bash
cd bfa-go
go run ./cmd/api
```

### Com Docker Compose (raiz do repositório)

```bash
docker compose up --build
```

### Testes

```bash
cd bfa-go
go test ./... -v
```

### Cobertura de testes

```bash
cd bfa-go
go test ./... -cover
```

---

## Endpoints

### POST /v1/assistant

Envia uma consulta ao Agente de IA para o cliente informado. A resposta é armazenada em cache por TTL configurável.

**Requisição**

```
POST /v1/assistant
Content-Type: application/json

{
  "customer_id": "cliente-001",
  "prompt": "qual é o meu saldo atual?"
}
```

**Resposta — 200 OK**

```json
{
  "customer_id": "cliente-001",
  "prompt": "qual é o meu saldo atual?",
  "response": "Seu saldo atual é de AOA 350.000,00...",
  "cached": false,
  "duration_ms": 312,
  "metadata": {}
}
```

Quando a resposta vem do cache, o campo `cached` retorna `true`.

**Respostas de erro**

| Status | Condição |
|--------|----------|
| 400 | `customer_id` ausente ou vazio / `prompt` ausente ou vazio / corpo JSON inválido |
| 429 | Bulkhead cheio (muitas requisições simultâneas) |
| 502 | Agente indisponível ou retornou erro |
| 503 | Circuit breaker aberto |
| 504 | Timeout excedido |
| 500 | Erro interno inesperado |

**Exemplos de erro**

```json
{ "error": "customer_id é obrigatório", "code": 400 }
{ "error": "prompt é obrigatório", "code": 400 }
{ "error": "muitas requisições simultâneas", "code": 429 }
{ "error": "serviço do agente indisponível: agente retornou status 500", "code": 502 }
{ "error": "circuit breaker aberto", "code": 503 }
```

---

### GET /v1/customers/{customerId}/profile

Retorna o perfil do cliente. Quando a `PROFILE_API_URL` não está configurada, retorna dados mock.

**Requisição**

```
GET /v1/customers/cliente-001/profile
```

**Resposta — 200 OK**

```json
{
  "id": "cliente-001",
  "name": "Cliente Simulado",
  "email": "cliente@bfa.ao",
  "segment": "premium",
  "account_id": "AO06.0006.0000.0000.0000.0024.7",
  "created_at": "2024-03-14T10:00:00Z"
}
```

**Respostas de erro**

| Status | Condição |
|--------|----------|
| 400 | `customerId` ausente |
| 404 | Cliente não encontrado |
| 429 | Bulkhead cheio |
| 502 | API de perfil indisponível |
| 503 | Circuit breaker aberto |

---

### GET /v1/customers/{customerId}/transactions

Retorna as transações do cliente. Quando a `TRANSACTIONS_API_URL` não está configurada, retorna dados mock.

**Requisição**

```
GET /v1/customers/cliente-001/transactions
```

**Resposta — 200 OK**

```json
[
  {
    "id": "txn-001",
    "date": "2026-03-13T00:00:00Z",
    "description": "Supermercado Kero",
    "amount": -15000.00,
    "category": "alimentação",
    "type": "débito"
  },
  {
    "id": "txn-002",
    "date": "2026-03-12T00:00:00Z",
    "description": "Salário Mensal",
    "amount": 350000.00,
    "category": "salário",
    "type": "crédito"
  }
]
```

**Respostas de erro**

| Status | Condição |
|--------|----------|
| 400 | `customerId` ausente |
| 429 | Bulkhead cheio |
| 502 | API de transações indisponível |
| 503 | Circuit breaker aberto |

---

### GET /healthz

Health check geral. Indica que o processo está em execução.

**Resposta — 200 OK**

```json
{ "status": "ok" }
```

---

### GET /livez

Liveness probe (padrão Kubernetes). Indica que o processo está vivo.

**Resposta — 200 OK**

```json
{ "status": "alive" }
```

---

### GET /readyz

Readiness probe. Indica que o servidor está pronto para receber tráfego.

**Resposta — 200 OK**

```json
{ "status": "ready" }
```

---

### GET /metrics

Expõe métricas no formato Prometheus (text/plain).

---

## Métricas Prometheus

| Métrica | Tipo | Labels | Descrição |
|---------|------|--------|-----------|
| `bfa_http_requests_total` | Counter | `method`, `path`, `status` | Total de requisições HTTP por método, rota e status |
| `bfa_http_request_duration_seconds` | Histogram | `method`, `path` | Duração das requisições HTTP em segundos |
| `bfa_http_requests_in_flight` | Gauge | - | Requisições HTTP sendo processadas no momento |
| `bfa_cache_hits_total` | Counter | - | Total de acertos no cache do assistente |
| `bfa_cache_misses_total` | Counter | - | Total de falhas no cache do assistente |

Exemplos de consulta PromQL:

```promql
# Taxa de requisições por segundo
rate(bfa_http_requests_total[5m])

# Percentil 99 de latência
histogram_quantile(0.99, rate(bfa_http_request_duration_seconds_bucket[5m]))

# Taxa de acerto do cache
rate(bfa_cache_hits_total[5m]) / (rate(bfa_cache_hits_total[5m]) + rate(bfa_cache_misses_total[5m]))
```

---

## Variáveis de Ambiente

| Variável | Padrão | Descrição |
|----------|--------|-----------|
| `PORT` | `8080` | Porta HTTP do servidor |
| `SERVER_READ_TIMEOUT` | `5s` | Timeout de leitura do servidor |
| `SERVER_WRITE_TIMEOUT` | `30s` | Timeout de escrita do servidor |
| `SERVER_IDLE_TIMEOUT` | `120s` | Timeout de conexão ociosa |
| `AGENT_URL` | `http://localhost:8000` | URL base do Agente Python |
| `AGENT_TIMEOUT` | `25s` | Timeout para chamadas ao agente |
| `PROFILE_API_URL` | `""` (usa mock) | URL base da Profile API |
| `PROFILE_API_TIMEOUT` | `5s` | Timeout para chamadas à Profile API |
| `TRANSACTIONS_API_URL` | `""` (usa mock) | URL base da Transactions API |
| `TRANSACTIONS_API_TIMEOUT` | `5s` | Timeout para chamadas à Transactions API |
| `CACHE_TTL` | `5m` | TTL das entradas no cache |
| `CACHE_CLEANUP_INTERVAL` | `10m` | Intervalo de limpeza do cache expirado |
| `RETRY_MAX_ATTEMPTS` | `3` | Número máximo de tentativas no retry |
| `RETRY_BASE_DELAY` | `200ms` | Delay base para backoff exponencial |
| `RETRY_MAX_DELAY` | `10s` | Delay máximo entre tentativas |
| `CB_FAILURE_THRESHOLD` | `5` | Falhas consecutivas para abrir o circuit breaker |
| `CB_TIMEOUT` | `10s` | Tempo em estado Open antes de tentar Half-Open |
| `BULKHEAD_MAX_CONCURRENT` | `100` | Limite de requisições simultâneas (semáforo) |
| `LOG_LEVEL` | `info` | Nível de log: `debug`, `info`, `warn`, `error` |
| `TRACING_ENABLED` | `true` | Habilitar OpenTelemetry tracing |
| `OTEL_SERVICE_NAME` | `bfa-go` | Nome do serviço reportado ao backend de tracing |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `""` (stdout) | Endpoint OTLP para exportação de traces |

---

## Tecnologias

| Biblioteca | Versão | Finalidade |
|------------|--------|------------|
| `net/http` (stdlib) | Go 1.25 | Servidor HTTP e roteamento com path params nativos |
| `log/slog` (stdlib) | Go 1.21 | Logs estruturados em JSON |
| `github.com/sony/gobreaker` | v1.0.0 | Circuit breaker |
| `github.com/patrickmn/go-cache` | v2.1.0 | Cache em memória com TTL |
| `github.com/prometheus/client_golang` | v1.23.2 | Métricas Prometheus |
| `go.opentelemetry.io/otel` | v1.42.0 | Distributed tracing (OpenTelemetry) |
| `go.opentelemetry.io/otel/sdk` | v1.42.0 | SDK do TracerProvider OTel |
| `go.opentelemetry.io/otel/exporters/stdout/stdouttrace` | v1.42.0 | Exporter de traces para stdout |
| `github.com/joho/godotenv` | v1.5.1 | Carregamento de arquivo `.env` |

---

## Padrões de Resiliência

| Padrão | Implementação | Onde é aplicado |
|--------|---------------|-----------------|
| Circuit Breaker | `sony/gobreaker` | Clientes do agente, profile e transactions |
| Retry + Backoff | Exponencial com jitter (`pkg/resilience`) | Clientes de profile e transactions |
| Bulkhead | Semáforo via channel (`pkg/resilience`) | Middleware global em todas as rotas |
| Timeout / Context | `context.WithTimeout` + `http.Client.Timeout` | Propagado em toda a cadeia de chamadas |
| Cache TTL | `patrickmn/go-cache` | Respostas do endpoint `/v1/assistant` |
| Graceful Shutdown | `signal.Notify` + `http.Server.Shutdown` | Encerramento com dreno de conexões ativas |
| Recovery Middleware | `defer/recover` | Captura panics em qualquer handler, retorna 500 |

---

## Testes

### Testes unitários — Resiliência (`test/resilience/`)

| Teste | Cobertura |
|-------|-----------|
| `TestCircuitBreaker_Created` | Verifica instanciação do circuit breaker |
| `TestRetrier_SuccessFirstAttempt` | Operação bem-sucedida na primeira tentativa |
| `TestRetrier_SuccessAfterRetries` | Sucesso após 2 falhas transitórias |
| `TestRetrier_AllAttemptsFail` | Todas as tentativas falham, retorna último erro |
| `TestRetrier_ContextCancelled` | Context cancelado interrompe o ciclo de retry |
| `TestBulkhead_AcquireRelease` | Acquire/Release, rejeição quando slots esgotados |
| `TestBulkhead_ContextCancelled` | Context cancelado ao tentar adquirir slot cheio |

### Testes de integração — Handlers (`test/handler/`)

| Teste | Cobertura |
|-------|-----------|
| `TestAssistant_Success` | Requisição válida, resposta do agente, sem cache |
| `TestAssistant_CacheHit` | Segunda requisição idêntica retorna `cached: true` |
| `TestAssistant_EmptyPrompt` | Prompt vazio retorna 400 |
| `TestAssistant_InvalidBody` | JSON inválido retorna 400 |
| `TestAssistant_AgentError` | Agente retornando 500 resulta em 502 |
| `TestProfile_MockFallback` | Sem URL configurada, retorna dados mock com 200 |
| `TestProfile_RealAPI_Success` | Servidor mock real responde corretamente |
| `TestTransaction_MockFallback` | Sem URL configurada, retorna 5 transações mock |
| `TestTransaction_RealAPI_Success` | Servidor mock real retorna lista de transações |

```bash
go test ./... -v -count=1
# 16 testes, 0 falhas
```

---

## Checklist de Requisitos

| Requisito | Status | Implementação |
|-----------|--------|---------------|
| Concorrência adequada | OK | Bulkhead global (`pkg/resilience.Bulkhead`) + goroutines do `net/http` |
| Timeout e cancelamento com context | OK | `http.Client.Timeout` + `context` propagado nos clientes |
| Retry com backoff | OK | `pkg/resilience.Retrier` — exponencial com jitter |
| Circuit breaker | OK | `sony/gobreaker` — um CB por cliente downstream |
| Bulkhead (limite de concorrência) | OK | Semáforo global aplicado como middleware antes de qualquer handler |
| `/healthz` | OK | `GET /healthz` retorna `{"status":"ok"}` |
| `/livez` | OK | `GET /livez` retorna `{"status":"alive"}` |
| `/readyz` | OK | `GET /readyz` retorna `{"status":"ready"}` |
| `/metrics` (Prometheus) | OK | `GET /metrics` — 5 métricas customizadas `bfa_*` |
| Logs estruturados | OK | `log/slog` com handler JSON, nível configurável via `LOG_LEVEL` |
| Tracing (OpenTelemetry) | OK | `pkg/tracing` — TracerProvider com exporter stdout/OTLP |
| Cache com TTL | OK | `pkg/cache` wrapping `patrickmn/go-cache`, TTL configurável |
| Tratamento consistente de erros | OK | `pkg/errors` — erros sentinela + `HandleError` mapeando para HTTP status |

---

## Trade-offs e Notas de Produção

- **Cache em memória:** em produção deve ser substituído por Redis para estado compartilhado entre réplicas.
- **Trace exporter stdout:** para produção, configurar `OTEL_EXPORTER_OTLP_ENDPOINT` apontando para um OpenTelemetry Collector (Jaeger, Grafana Tempo, Dynatrace etc.).
- **Mock fallback:** os clientes de profile e transactions retornam dados simulados quando a URL não está configurada, permitindo desenvolvimento e testes sem dependências externas.
- **Sem autenticação:** em produção adicionar JWT/OAuth2 como middleware nas rotas `/v1/`.
