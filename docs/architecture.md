# Arquitetura — AI Banking Assistant (Padrão BFA)

## Visão Geral

A arquitetura segue o padrão **BFA (Back-end for Agents)**, onde:
- O **Agente** (Python/LangGraph) é o ponto de entrada e responsável pela **jornada** do cliente
- O **BFA** (Go) é a camada intermediária que encapsula APIs de domínio com cache, resiliência, logging e políticas

> *"A responsabilidade do agente é a jornada — e não o backend."*

```mermaid
graph TB
    Client([Cliente PJ]) -->|HTTP POST /generate| AGENT_SERVICE

    subgraph AGENT_SERVICE["Agente — Python / LangGraph (A Jornada)"]
        direction TB
        InputGuard[Input Guardrail] --> Planner[Planner Node]
        Planner --> Executor[Executor Node]
        Executor --> Tools[Tool Dispatcher]
        Tools --> RAGTool[RAG Tool — Local]
        Tools -->|MCP| BFA_PROFILE[BFA: get_customer_profile]
        Tools -->|MCP| BFA_TXN[BFA: get_customer_transactions]
        Executor --> Formatter[Formatter Node]
        Formatter --> OutputGuard[Output Guardrail]
    end

    subgraph BFA_GO["BFA Go — Back-end for Agents (O Backend)"]
        direction TB
        Router[Router / Middleware Stack]
        Router --> ProfileH[Profile Handler]
        Router --> TxnH[Transaction Handler]

        subgraph BFA_CORE["Responsabilidades BFA"]
            Cache[(Cache TTL)]
            Auth[Políticas / Auth]
            Logging[Logging Centralizado]
            Metrics[Métricas Prometheus]
        end

        ProfileH --- BFA_CORE
        TxnH --- BFA_CORE

        subgraph Resilience["Resiliência"]
            CB[Circuit Breaker]
            RT[Retry + Backoff]
            BH[Bulkhead]
        end

        ProfileH --> ProfileAPI[Profile API de Domínio]
        TxnH --> TransactionsAPI[Transactions API de Domínio]
        ProfileH --- Resilience
        TxnH --- Resilience
    end

    BFA_PROFILE -->|HTTP| Router
    BFA_TXN -->|HTTP| Router

    subgraph KNOWLEDGE["Knowledge Base — Local no Agente"]
        RAGTool --> VectorDB[(ChromaDB)]
        RAGTool --> Embeddings[OpenAI Embeddings]
    end

    subgraph LLM_PROVIDER["LLM Provider"]
        LLM[OpenAI API]
    end

    Planner -->|prompt| LLM
    Executor -->|prompt + tools| LLM
    Formatter -->|prompt| LLM
    InputGuard -->|classificação| LLM
    OutputGuard -->|auditoria| LLM

    subgraph OBSERVABILITY["Observabilidade"]
        Prometheus[Prometheus]
        LangFuse[LangFuse]

        BFA_GO -.->|metrics + traces| Prometheus
        AGENT_SERVICE -.->|traces + tokens| LangFuse
    end
```

## Fluxo Principal — Padrão BFA

```
Cliente → Agente (LLM + Jornada) → BFA (Cache + Resiliência) → APIs de Domínio
```

```mermaid
sequenceDiagram
    participant C as Cliente
    participant Agent as Agente Python
    participant LLM as LLM (OpenAI)
    participant BFA as BFA Go
    participant Cache as BFA Cache
    participant Profile as Profile API
    participant Txn as Transactions API

    C->>Agent: POST /generate {customer_id, query}
    Agent->>Agent: Input Guardrail (sanitize + risk)
    Agent->>LLM: Planner — criar plano
    LLM-->>Agent: Plano de execução
    Agent->>LLM: Executor — executar plano
    LLM-->>Agent: Tool calls necessárias

    par Agente chama BFA para dados de domínio
        Agent->>BFA: GET /v1/customers/{id}/profile
        BFA->>Cache: Check cache
        alt Cache hit
            Cache-->>BFA: Perfil em cache
        else Cache miss
            BFA->>Profile: Buscar perfil (com retry + CB)
            Profile-->>BFA: Dados do perfil
            BFA->>Cache: Armazenar (TTL)
        end
        BFA-->>Agent: Perfil do cliente

        Agent->>BFA: GET /v1/customers/{id}/transactions
        BFA->>Cache: Check cache
        alt Cache hit
            Cache-->>BFA: Transações em cache
        else Cache miss
            BFA->>Txn: Buscar transações (com retry + CB)
            Txn-->>BFA: Lista de transações
            BFA->>Cache: Armazenar (TTL)
        end
        BFA-->>Agent: Transações do cliente
    end

    Agent->>LLM: Formatter — estruturar resposta
    LLM-->>Agent: Resposta formatada (JSON)
    Agent->>Agent: Output Guardrail (auditoria)
    Agent-->>C: 200 OK {response, metadata}
```

## Endpoints de Infraestrutura

```mermaid
graph LR
    subgraph BFA["BFA Go"]
        H["/healthz"] --- liveness[Liveness Check]
        R["/readyz"] --- readiness[Readiness Check]
        M["/metrics"] --- prom[Prometheus Metrics]
    end
```

## Padrões de Resiliência

```mermaid
graph TD
    Request([Request]) --> BH{Bulkhead<br/>Slots disponíveis?}
    BH -->|Não| Reject[429 Too Many Requests]
    BH -->|Sim| CB{Circuit Breaker<br/>Estado?}
    CB -->|Open| Fallback[Fallback / Erro controlado]
    CB -->|Closed / Half-Open| Timeout{Timeout<br/>Context Deadline}
    Timeout -->|Expirado| Cancel[Cancelar + Erro]
    Timeout -->|OK| Call[Chamada externa]
    Call -->|Erro transitório| Retry[Retry c/ Exponential Backoff]
    Retry -->|Max tentativas| Fallback
    Retry -->|Sucesso| Response([Response])
    Call -->|Sucesso| Response
```

## Estratégia de Deploy — AWS (Padrão BFA)

```mermaid
graph TB
    subgraph AWS["AWS Cloud"]
        ALB[Application Load Balancer]

        subgraph ECS["ECS Fargate"]
            AGENT_TASK[Agente Python — Ponto de Entrada]
            BFA_TASK[BFA Go — Backend for Agents]
        end

        subgraph DATA["Data Layer"]
            ElastiCache[(ElastiCache / Redis)]
            S3[(S3 — Knowledge Base)]
            VectorDB[(ChromaDB)]
        end

        subgraph MONITORING["Monitoring"]
            CloudWatch[CloudWatch]
            Prometheus[Prometheus]
            LANGFUSE[LangFuse]
        end

        ALB --> AGENT_TASK
        AGENT_TASK -->|BFA calls| BFA_TASK
        BFA_TASK --> ElastiCache
        AGENT_TASK --> VectorDB
        AGENT_TASK --> S3
        BFA_TASK -.-> Prometheus
        AGENT_TASK -.-> LANGFUSE
        Prometheus -.-> CloudWatch
    end

    subgraph EXTERNAL["External"]
        LLM_API[LLM Provider API]
    end

    AGENT_TASK --> LLM_API
```

## Contratos do BFA (APIs expostas aos Agentes)

| Endpoint | Método | Domínio | Descrição |
|---|---|---|---|
| `/v1/customers/{id}/profile` | GET | Perfil | Dados cadastrais do cliente |
| `/v1/customers/{id}/transactions` | GET | Transações | Histórico financeiro |
| `/healthz` | GET | Infra | Health check |
| `/readyz` | GET | Infra | Readiness check |
| `/metrics` | GET | Infra | Métricas Prometheus |

## Endpoint do Agente (Ponto de entrada do cliente)

| Endpoint | Método | Descrição |
|---|---|---|
| `/generate` | POST | Recebe `{customer_id, query}` e retorna resposta do assistente |

## Comunicação entre Serviços (Padrão BFA)

| De | Para | Protocolo | Motivo |
|---|---|---|---|
| Cliente | Agente Python | HTTP/REST | Ponto de entrada — a jornada |
| Agente Python | BFA Go | HTTP/REST | Obter dados de domínio (perfil, transações) |
| BFA Go | Profile API | HTTP | Dados do cliente (com cache + resiliência) |
| BFA Go | Transactions API | HTTP | Histórico financeiro (com cache + resiliência) |
| Agente Python | LLM Provider | HTTP | Inferência / raciocínio |
| Agente Python | ChromaDB | SDK | Busca semântica na knowledge base |

## Escalabilidade

```mermaid
graph LR
    subgraph AgentScale["Escala do Agente (Ponto de Entrada)"]
        A1[Agent #1]
        A2[Agent #2]
        A3[Agent #N]
    end

    subgraph BFAScale["Escala do BFA (Backend)"]
        BFA1[BFA Go #1]
        BFA2[BFA Go #2]
        BFA3[BFA Go #N]
    end

    LB[Load Balancer] --> A1
    LB --> A2
    LB --> A3

    A1 --> BFA1
    A2 --> BFA2
    A3 --> BFA3
```

- **Agent Python**: Ponto de entrada do cliente — escala horizontalmente via réplicas ECS
- **BFA Go**: Stateless, escala horizontalmente — serve dados de domínio aos agentes
- **Cache**: Redis compartilhado entre instâncias do BFA
- **ChromaDB**: Escala vertical ou managed service
