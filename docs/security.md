# Segurança e Governança

## Visão Geral das Camadas de Segurança

```mermaid
graph TB
    Input([Input do Usuário]) --> L1

    subgraph Layers["Camadas de Defesa"]
        L1[🛡 Sanitização de Entrada]
        L1 --> L2[🔒 Detecção de Prompt Injection]
        L2 --> L3[🎭 Mascaramento de Dados Sensíveis]
        L3 --> L4[📋 Controle de Contexto]
        L4 --> L5[💰 Rate Limiting / Custo]
    end

    L5 --> Agent([Agente IA])
    Agent --> OutputGuard[🛡 Output Guard]
    OutputGuard --> Response([Resposta Segura])
```

## Sanitização de Entrada

```mermaid
graph LR
    Raw[Input Raw] --> Trim[Trim + Normalize]
    Trim --> Length{Tamanho ≤ max?}
    Length -->|Não| Reject[Rejeitar]
    Length -->|Sim| Encode[Encode caracteres especiais]
    Encode --> Blocklist{Contém padrões<br/>bloqueados?}
    Blocklist -->|Sim| Reject
    Blocklist -->|Não| Clean[Input limpo ✅]
```

| Validação | Ação |
|---|---|
| Tamanho máximo | Rejeita inputs > 2000 chars |
| Caracteres especiais | Encode / strip |
| Padrões maliciosos | Blocklist regex |
| Unicode abuse | Normalização NFKC |

## Proteção contra Prompt Injection

```mermaid
graph TD
    Input([User Input]) --> Classifier{Classifier<br/>Injection Detection}

    Classifier -->|Safe| Process[Processar normalmente]
    Classifier -->|Suspicious| SecondCheck{LLM Guard<br/>Double-check}
    Classifier -->|Malicious| Block[Bloquear + Log]

    SecondCheck -->|Safe| Process
    SecondCheck -->|Confirmed threat| Block

    subgraph Techniques["Técnicas de Proteção"]
        T1[System prompt isolado e imutável]
        T2[Delimitadores claros user/system]
        T3[Input sandboxing — nunca executado como instrução]
        T4[Classifier heurístico + LLM-based]
    end
```

```mermaid
graph LR
    subgraph Prompt["Estrutura do Prompt"]
        SP[System Prompt<br/>— Imutável —<br/>Regras e limites]
        SEP1[--- DELIMITER ---]
        CTX[Context<br/>RAG + Profile + Transactions]
        SEP2[--- DELIMITER ---]
        UP[User Input<br/>— Sandboxed —]
    end

    SP --> SEP1 --> CTX --> SEP2 --> UP
```

## Dados Sensíveis

```mermaid
graph TD
    Data([Dados do Cliente]) --> Detector[PII Detector]

    Detector --> CPF[CPF → ***.***.***-XX]
    Detector --> CNPJ[CNPJ → **.***.***/ ****-XX]
    Detector --> Account[Conta → ****-X]
    Detector --> Email[Email → k***@***.com]
    Detector --> Phone[Telefone → (XX) ****-XXXX]

    CPF --> Masked[Dados Mascarados]
    CNPJ --> Masked
    Account --> Masked
    Email --> Masked
    Phone --> Masked

    Masked --> Agent[Enviado ao Agente / LLM]
```

| Dado | Técnica | Onde aplica |
|---|---|---|
| CPF/CNPJ | Mascaramento parcial | Antes de enviar ao LLM |
| Saldo/Valores | Tokenização | Logs e traces |
| Nome completo | Primeiro nome apenas | Contexto do agente |
| Dados bancários | Nunca enviados ao LLM | Processados apenas no BFA |

## Controle de Vazamento de Contexto

```mermaid
graph TD
    subgraph ContextIsolation["Isolamento de Contexto"]
        S1[Sessão isolada por request]
        S2[Sem memória entre requests]
        S3[Context window limitada]
        S4[Nenhum dado de outro cliente acessível]
    end

    subgraph OutputGuard["Output Guard"]
        O1[Scan de PII na resposta]
        O2[Bloqueia dados que não vieram do input]
        O3[Valida formato da resposta]
    end

    ContextIsolation --> OutputGuard
```

## Versionamento de Prompts

```mermaid
graph LR
    subgraph Registry["Prompt Registry"]
        V1[v1.0.0<br/>System Prompt Base]
        V2[v1.1.0<br/>+ Guardrails]
        V3[v1.2.0<br/>+ Compliance rules]
        V4[v2.0.0<br/>Refatoração]
    end

    V1 -->|minor| V2 -->|minor| V3 -->|major| V4

    subgraph Config["Configuração"]
        ENV[ENV var: PROMPT_VERSION]
        ENV --> Active{Versão ativa}
        Active --> V3
    end
```

| Aspecto | Abordagem |
|---|---|
| Storage | Prompts versionados em arquivo / config |
| Naming | SemVer (major.minor.patch) |
| Rollback | Troca de versão via env/config |
| Audit | Log de qual versão gerou cada resposta |
| A/B test | Duas versões ativas com split de tráfego |

## Limitação de Custo por Usuário

```mermaid
graph TD
    Request([Request]) --> RateLimit{Rate Limit<br/>requests/min}

    RateLimit -->|Excedido| TooMany[429 Too Many Requests]
    RateLimit -->|OK| TokenBudget{Token Budget<br/>diário do usuário}

    TokenBudget -->|Excedido| BudgetError[402 Budget Exceeded]
    TokenBudget -->|OK| Process[Processar request]

    Process --> Track[Registrar tokens usados]
    Track --> Metrics[Métricas de custo]
```

| Controle | Limite | Ação |
|---|---|---|
| Requests/min | 10 por customer | 429 + retry-after |
| Tokens/dia | 50.000 por customer | 402 + reset time |
| Custo/mês | Budget configurável | Alerta + bloqueio |
| Max input | 2.000 chars | 400 Bad Request |

## Observabilidade de Segurança

```mermaid
graph LR
    subgraph Events["Security Events"]
        E1[Injection attempt]
        E2[PII detected in output]
        E3[Rate limit hit]
        E4[Budget exceeded]
        E5[Anomalous pattern]
    end

    Events --> SIEM[SIEM / Log Aggregator]
    SIEM --> Alert[Alertas]
    SIEM --> Dashboard[Security Dashboard]
```
