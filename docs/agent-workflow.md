# Agent Workflow — LangGraph

## Grafo de Execução do Agente

```mermaid
graph TD
    START([Input: Customer Context]) --> Planner

    subgraph LangGraph["LangGraph — State Machine"]
        Planner[🧠 Planner Node]
        Planner -->|plan| Router{Router Node}

        Router -->|needs_profile| ProfileTool[📋 Profile Tool]
        Router -->|needs_transactions| TransactionTool[💳 Transaction Tool]
        Router -->|needs_knowledge| RAGTool[📚 RAG Tool]
        Router -->|needs_analysis| AnalysisTool[📊 Financial Analysis Tool]
        Router -->|ready| Consolidator

        ProfileTool --> Evaluator
        TransactionTool --> Evaluator
        RAGTool --> Evaluator
        AnalysisTool --> Evaluator

        Evaluator{🔍 Evaluator Node}
        Evaluator -->|more_info_needed| Router
        Evaluator -->|sufficient| Consolidator

        Consolidator[✅ Consolidator Node]
    end

    Consolidator --> END([Output: Structured Response])
```

## Estado do Agente (State Schema)

```mermaid
classDiagram
    class AgentState {
        +str customer_id
        +dict profile
        +list transactions
        +list rag_results
        +list plan_steps
        +int current_step
        +list tool_results
        +list messages
        +str final_response
        +dict metadata
    }
```

## Fluxo Detalhado por Nó

```mermaid
sequenceDiagram
    participant BFA as BFA Go
    participant P as Planner
    participant R as Router
    participant T as Tools
    participant E as Evaluator
    participant C as Consolidator
    participant LLM as LLM

    BFA->>P: {customer_id, profile, transactions}
    P->>LLM: "Quais informações são necessárias?"
    LLM-->>P: Plan [step1, step2, ...]

    loop Para cada step do plano
        P->>R: next_step
        R->>T: invoke tool
        T-->>E: tool_result
        E->>LLM: "Temos informação suficiente?"
        LLM-->>E: sufficient / need_more
        alt need_more
            E->>R: request additional tool
        end
    end

    E->>C: all results collected
    C->>LLM: "Consolide a resposta final"
    LLM-->>C: structured_response + justification
    C-->>BFA: final response
```

## Tools (Ferramentas)

```mermaid
graph LR
    subgraph Tools["Tool Registry"]
        T1[📋 get_customer_profile<br/>Dados cadastrais e segmento]
        T2[💳 get_transactions<br/>Histórico de transações]
        T3[📚 search_knowledge_base<br/>Busca semântica RAG]
        T4[📊 analyze_financials<br/>Análise de padrões]
        T5[💡 generate_recommendation<br/>Recomendação personalizada]
    end
```

| Tool | Input | Output | Fallback |
|---|---|---|---|
| `get_customer_profile` | customer_id | Profile dict | Cached profile / erro parcial |
| `get_transactions` | customer_id, period | Transaction list | Últimas N cached |
| `search_knowledge_base` | query, top_k | Relevant chunks | Resposta sem contexto RAG |
| `analyze_financials` | transactions, profile | Analysis dict | Análise simplificada |
| `generate_recommendation` | full_context | Recommendation | Resposta genérica |

## Execução Condicional

```mermaid
graph TD
    Input([Context]) --> CheckProfile{Tem profile<br/>no input?}

    CheckProfile -->|Sim| CheckTxn{Tem transações<br/>no input?}
    CheckProfile -->|Não| FetchProfile[Fetch Profile Tool]
    FetchProfile --> CheckTxn

    CheckTxn -->|Sim| NeedRAG{Pergunta requer<br/>conhecimento base?}
    CheckTxn -->|Não| FetchTxn[Fetch Transactions Tool]
    FetchTxn --> NeedRAG

    NeedRAG -->|Sim| RAG[RAG Search Tool]
    NeedRAG -->|Não| Analyze

    RAG --> Analyze[Financial Analysis]
    Analyze --> Consolidate[Consolidar Resposta]
```

## Estrutura da Resposta

```mermaid
classDiagram
    class AgentResponse {
        +str recommendation
        +str justification
        +list sources
        +dict metadata
        +float confidence_score
    }

    class ResponseMetadata {
        +int total_tokens
        +float latency_ms
        +int steps_executed
        +list tools_used
        +float estimated_cost
    }

    AgentResponse --> ResponseMetadata
```

## Tratamento de Erros

```mermaid
graph TD
    ToolCall([Tool Call]) --> Result{Resultado}

    Result -->|Sucesso| Continue[Continuar fluxo]
    Result -->|Timeout| RetryOnce{Retry 1x}
    Result -->|Erro LLM| FallbackLLM[Fallback response]
    Result -->|Erro Tool| SkipTool[Marcar tool como indisponível]

    RetryOnce -->|Sucesso| Continue
    RetryOnce -->|Falha| SkipTool

    SkipTool --> CanContinue{Pode continuar<br/>sem esta tool?}
    CanContinue -->|Sim| Continue
    CanContinue -->|Não| PartialResponse[Resposta parcial + aviso]
```
