# Arquitetura — AI Banking Assistant

## Visão Geral

```mermaid
graph TB
    Client([Cliente PJ]) -->|HTTP| APIGW[API Gateway]

    subgraph BFA_GO["BFA Go — Orquestrador"]
        APIGW --> Router[Router / Middleware]
        Router --> Handler[Assistant Handler]
        Handler --> Orchestrator[Orchestrator]

        Orchestrator -->|goroutine| ProfileAPI[Profile API]
        Orchestrator -->|goroutine| TransactionsAPI[Transactions API]
        Orchestrator -->|goroutine| AgentCall[Agent Service Call]

        subgraph Resilience["Resiliência"]
            CB[Circuit Breaker]
            RT[Retry + Backoff]
            BH[Bulkhead]
            TO[Timeout / Context]
        end

        Orchestrator --- Resilience
        Handler --> Cache[(Cache TTL)]
    end

    subgraph AGENT_SERVICE["Agent Service — Python / LangGraph"]
        AgentCall -->|gRPC / HTTP| Planner[Planner Node]
        Planner --> Executor[Executor Node]
        Executor --> Tools[Tool Dispatcher]
        Tools --> RAGTool[RAG Tool]
        Tools --> FinancialTool[Financial Analysis Tool]
        Tools --> RecommendationTool[Recommendation Tool]
        Executor --> Consolidator[Consolidator Node]
    end

    subgraph KNOWLEDGE["Knowledge Base"]
        RAGTool --> VectorDB[(Vector Store)]
        RAGTool --> Embeddings[Embedding Model]
        VectorDB --- Chunks[Document Chunks]
    end

    subgraph OBSERVABILITY["Observabilidade"]
        OTel[OpenTelemetry Collector]
        Prometheus[Prometheus]
        LangFuse[LangFuse]
        Grafana[Grafana]

        BFA_GO -.->|traces| OTel
        BFA_GO -.->|metrics| Prometheus
        AGENT_SERVICE -.->|traces + tokens| LangFuse
        OTel -.-> Grafana
        Prometheus -.-> Grafana
    end

    subgraph LLM_PROVIDER["LLM Provider"]
        LLM[LLM API]
    end

    Planner -->|prompt| LLM
    Consolidator -->|prompt| LLM
```

## Endpoint Principal

```
GET /v1/assistant/{customerId}
```

```mermaid
sequenceDiagram
    participant C as Cliente
    participant BFA as BFA Go
    participant Cache as Cache
    participant Profile as Profile API
    participant Txn as Transactions API
    participant Agent as Agent Service
    participant LLM as LLM

    C->>BFA: GET /v1/assistant/{customerId}
    BFA->>Cache: Check cache
    alt Cache hit
        Cache-->>BFA: Cached response
        BFA-->>C: 200 OK (cached)
    else Cache miss
        par Chamadas concorrentes
            BFA->>Profile: Get profile
            BFA->>Txn: Get transactions
        end
        Profile-->>BFA: Customer profile
        Txn-->>BFA: Transaction history
        BFA->>Agent: Send context (profile + transactions)
        Agent->>LLM: Plan + Execute + Consolidate
        LLM-->>Agent: Structured response
        Agent-->>BFA: Recommendation + justification
        BFA->>Cache: Store (TTL)
        BFA-->>C: 200 OK
    end
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

## Estratégia de Deploy — AWS

```mermaid
graph TB
    subgraph AWS["AWS Cloud"]
        ALB[Application Load Balancer]

        subgraph ECS["ECS Fargate"]
            BFA_TASK[BFA Go Task]
            AGENT_TASK[Agent Service Task]
        end

        subgraph DATA["Data Layer"]
            ElastiCache[(ElastiCache / Redis)]
            S3[(S3 — Knowledge Base)]
            VectorDB[(Vector Store)]
        end

        subgraph MONITORING["Monitoring"]
            CloudWatch[CloudWatch]
            OTEL_COL[OTel Collector]
            LANGFUSE[LangFuse]
        end

        ALB --> BFA_TASK
        BFA_TASK --> AGENT_TASK
        BFA_TASK --> ElastiCache
        AGENT_TASK --> VectorDB
        AGENT_TASK --> S3
        BFA_TASK -.-> OTEL_COL
        AGENT_TASK -.-> LANGFUSE
        OTEL_COL -.-> CloudWatch
    end

    subgraph EXTERNAL["External"]
        LLM_API[LLM Provider API]
    end

    AGENT_TASK --> LLM_API
```

## Comunicação entre Serviços

| De | Para | Protocolo | Motivo |
|---|---|---|---|
| Client | BFA Go | HTTP/REST | Ponto de entrada |
| BFA Go | Profile API | HTTP | Dados do cliente |
| BFA Go | Transactions API | HTTP | Histórico financeiro |
| BFA Go | Agent Service | HTTP/gRPC | Invocação do agente |
| Agent Service | LLM Provider | HTTP | Inferência |
| Agent Service | Vector Store | SDK | Busca semântica |

## Escalabilidade

```mermaid
graph LR
    subgraph Horizontal["Escala Horizontal"]
        BFA1[BFA Go #1]
        BFA2[BFA Go #2]
        BFA3[BFA Go #N]
    end

    subgraph AgentScale["Agent Scale"]
        A1[Agent #1]
        A2[Agent #2]
        A3[Agent #N]
    end

    LB[Load Balancer] --> BFA1
    LB --> BFA2
    LB --> BFA3

    BFA1 --> A1
    BFA2 --> A2
    BFA3 --> A3
```

- **BFA Go**: Stateless, escala horizontalmente via réplicas ECS
- **Agent Service**: Escala independente, separado do BFA
- **Cache**: Redis compartilhado entre instâncias do BFA
- **Vector Store**: Escala vertical ou managed service
