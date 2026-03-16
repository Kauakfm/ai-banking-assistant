# Arquitetura de Produção — AI Banking Assistant (Hipotética)

> Este documento descreve a **visão de produção** do AI Banking Assistant usando serviços AWS. A arquitetura local (Docker Compose) serve como prova de conceito; este documento projeta como seria o deployment em um ambiente bancário real.

---

## Visão Geral

```
Clientes (Mobile / Web / Partners)
         │
    ┌────▼──────┐
    │  Cognito / │
    │  Keycloak  │  ← Autenticação
    └────┬──────┘
         │ JWT
    ┌────▼──────────────────┐
    │  Amazon API Gateway   │  ← Rate Limiting, WAF, Throttling
    └────┬──────────────────┘
         │
    ┌────▼──────────────────────────────────────────┐
    │  Amazon Bedrock AgentCore Runtime              │
    │  ┌─────────────────────────────────────────┐  │
    │  │ Supervisor (LangGraph)                  │  │
    │  │  ├── Guardrails (Input + Output)        │  │
    │  │  ├── Planner                            │  │
    │  │  ├── Profile Agent ──┐                  │  │
    │  │  ├── Transaction Agent ──MCP─→ BFA      │  │
    │  │  ├── Knowledge Agent ──RAG─→ OpenSearch │  │
    │  │  ├── Formatter                          │  │
    │  │  └── Memory (AgentCore)                 │  │
    │  └─────────────────────────────────────────┘  │
    └──────────────┬──────────────┬─────────────────┘
                   │              │
          ┌────────▼───┐   ┌─────▼─────────┐
          │ Bedrock    │   │ Gemini         │
          │ Claude     │   │ Long Context   │
          │ (LLM)      │   │ (RAG)          │
          └────────────┘   └───────────────┘
                   │
    ┌──────────────▼──────────────────┐
    │  BFA (Lambda / EKS)             │
    │  ├── M2M Auth (mTLS / JWT)      │
    │  ├── Cache (ElastiCache/Redis)  │
    │  ├── Circuit Breaker            │
    │  ├── Retry / Bulkhead           │
    │  └── API Gateway (Internal)     │
    └──────────────┬──────────────────┘
                   │
    ┌──────────────▼──────────────────┐
    │  Core Banking APIs (Domínio)    │
    │  ├── Profile Service            │
    │  ├── Transaction Service        │
    │  └── Credit Service             │
    └─────────────────────────────────┘
```

---

## Componentes AWS

### 1. Amazon Bedrock AgentCore Runtime

O **Bedrock AgentCore** hospeda e executa os agentes em produção. Substitui o Docker container do `agent-python`.

**Responsabilidades:**
- Runtime gerenciado para agentes LangGraph
- Gerenciamento de memória (session memory)
- Hospedagem de MCP tools
- Auto-scaling baseado em demanda
- Integração nativa com modelos Bedrock

**Mapeamento Local → Produção:**

| Local | Produção |
|---|---|
| `agent-python` (Docker) | Bedrock AgentCore Runtime |
| In-process memory | AgentCore Memory |
| `langchain-mcp-adapters` (stdio) | AgentCore MCP Tools |

### 2. Amazon Bedrock (LLMs)

Substitui as chamadas diretas à OpenAI API.

**Modelos recomendados:**
- **Claude 3.5 Sonnet:** Planner, Formatter (raciocínio complexo)
- **Claude 3.5 Haiku:** Guardrails, sub-agentes (baixa latência, menor custo)

**Vantagens:**
- Dados não saem da conta AWS
- SLA e compliance bancário
- Sem dependência de API terceira

### 3. Gemini (Long Context RAG)

Para documentos extensos (políticas bancárias, contratos, regulamentações), o **Gemini** com long context (1M+ tokens) permite processar documentos inteiros sem necessidade de chunking.

**Uso:** Knowledge Agent para documentos > 100 páginas que não se beneficiam de chunking/embedding.

### 4. OpenSearch Serverless (Vector DB)

Substitui o ChromaDB local para busca vetorial em escala.

**Uso:** Knowledge Agent para FAQs, políticas padrão — chunking + embedding + similarity search.

**Configuração:**
- Collection type: Vector Search
- Embedding: `amazon.titan-embed-text-v2`
- Dimensões: 1024

### 5. BFA — Lambda ou EKS

O BFA Go pode rodar em **Lambda** (para workloads com picos) ou **EKS** (para latência consistente).

| Opção | Quando usar |
|---|---|
| **Lambda** | Poucas requisições, custo por invocação, cold start aceitável |
| **EKS** | Alta demanda, latência < 100ms necessária, muitas conexões de banco |

**Cross-cutting em produção:**
- **Cache:** ElastiCache (Redis) no lugar de go-cache
- **Circuit Breaker:** Mesmo gobreaker, configurado para múltiplas instâncias
- **Métricas:** CloudWatch Metrics via embedded metric format
- **Tracing:** X-Ray (via OpenTelemetry Collector)

### 6. Autenticação

#### Cliente → API Gateway
- **Amazon Cognito** ou **Keycloak** para autenticação de usuários finais
- JWT token validado no API Gateway
- Authorizer Lambda para políticas customizadas

#### Agente → BFA (M2M)
- **mTLS** (mutual TLS) para comunicação machine-to-machine
- Ou **JWT M2M** com client credentials grant
- Certificados gerenciados via AWS Certificate Manager

```
┌─────────┐                    ┌─────────┐
│ Agente  │──── mTLS/JWT ────→│  BFA    │
│ (Agent  │    M2M Auth       │  (Go)   │
│  Core)  │                    │         │
└─────────┘                    └─────────┘
```

### 7. API Gateway

**Externo (cliente → agente):**
- Amazon API Gateway (REST ou HTTP API)
- WAF para proteção contra ataques
- Rate limiting por cliente
- Throttling por endpoint

**Interno (BFA → Core Banking):**
- API Gateway interno ou VPC Link
- Service mesh (App Mesh) opcional
- mTLS entre serviços

### 8. Observabilidade

| Componente | Ferramenta | Função |
|---|---|---|
| **Logs** | CloudWatch Logs | Logs estruturados (JSON) |
| **Métricas** | CloudWatch Metrics | Latência, erros, throughput |
| **Tracing** | X-Ray / Jaeger | Traces distribuídos end-to-end |
| **LLM Observability** | LangFuse Cloud | Traces de LLM, custos, tokens |
| **Dashboards** | Grafana (Amazon Managed) | Visualização unificada |
| **Alertas** | CloudWatch Alarms + SNS | Alertas por threshold |

---

## Diagrama Draw.io

A arquitetura completa está disponível em:
- **[docs/architecture-production.drawio](architecture-production.drawio)** — Abrir no [draw.io](https://app.diagrams.net/)

---

## Fluxo de Requisição em Produção

```
1. Cliente autentica via Cognito → recebe JWT
2. Cliente envia POST /generate com JWT no header Authorization
3. API Gateway valida JWT, aplica rate limiting
4. Requisição chega ao Bedrock AgentCore (Supervisor)
5. Input Guardrail (Haiku) → sanitiza + classifica risco
6. Planner (Sonnet) → decide quais agentes acionar
7. Sub-agentes executam em paralelo:
   a. Profile Agent → MCP → BFA (mTLS) → Core Banking Profile API
   b. Transaction Agent → MCP → BFA (mTLS) → Core Banking Transaction API
   c. Knowledge Agent → RAG → OpenSearch Serverless / Gemini
8. Formatter (Sonnet) → consolida respostas
9. Output Guardrail (Haiku) → verifica segurança da resposta
10. Resposta retornada ao cliente
11. Traces enviados para LangFuse Cloud + X-Ray
```

---

## Estimativa de Latência

| Etapa | Latência Estimada |
|---|---|
| API Gateway + Auth | ~20ms |
| Input Guardrail (Haiku) | ~300ms |
| Planner (Sonnet) | ~500ms |
| Sub-agentes (paralelo) | ~800ms |
| BFA → Core Banking | ~100ms |
| Formatter (Sonnet) | ~500ms |
| Output Guardrail (Haiku) | ~300ms |
| **Total (end-to-end)** | **~2.5s** |

> Em produção, com cache no BFA e paralelização real dos sub-agentes, o tempo pode cair para ~1.5s.

---

## Custos Estimados (Referência)

| Serviço | Custo Mensal (estimativa) |
|---|---|
| Bedrock AgentCore | Varia por invocação |
| Bedrock Claude (Sonnet) | ~$3/M input tokens, ~$15/M output tokens |
| Bedrock Claude (Haiku) | ~$0.25/M input, ~$1.25/M output |
| Lambda (BFA) | ~$0.20/M invocações |
| OpenSearch Serverless | ~$700/mês (2 OCU mín.) |
| ElastiCache Redis | ~$50/mês (t3.micro) |
| API Gateway | ~$3.50/M requests |
| LangFuse Cloud | $59/mês (Pro) |

> Valores de referência. Consultar AWS Pricing Calculator para estimativa precisa.

---

## Segurança em Produção

| Controle | Implementação |
|---|---|
| **Encryption at rest** | KMS para todos os dados |
| **Encryption in transit** | TLS 1.3 em todas as comunicações |
| **Data masking** | BFA mascara dados sensíveis antes de retornar ao agente |
| **Audit trail** | Todos os acessos logados no CloudTrail |
| **Network isolation** | VPC privada, subnets privadas para BFA e Core Banking |
| **Secrets management** | AWS Secrets Manager para chaves e tokens |
| **Guardrails** | Input + Output em cada requisição |
| **Tool whitelist** | Apenas tools autorizadas podem ser invocadas |
