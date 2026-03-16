# Decisões Arquiteturais — AI Banking Assistant

Registro das decisões arquiteturais tomadas no projeto, com contexto, alternativas avaliadas e trade-offs.

---

## ADR-001: Padrão BFA (Back-end for Agents)

**Status:** Adotado  
**Data:** 2025-01

### Contexto

Agentes de IA tendem a se acoplar diretamente às APIs de domínio, duplicando lógica de autenticação, cache, resiliência e tradução de dados em cada agente.

### Decisão

Introduzir uma **camada intermediária (BFA)** entre os agentes e as APIs de domínio, inspirada no padrão BFF (Back-end for Front-end).

### Alternativas Avaliadas

| Alternativa | Por que descartada |
|---|---|
| Agente chama APIs de domínio diretamente | Acoplamento forte, sem cache/resiliência centralizada, difícil de testar |
| API Gateway genérica | Não resolve tradução semântica nem agregação de dados para o agente |
| Microserviço intermediário sem padrão | Sem contrato claro, tende a virar monolito |

### Consequências

- **Positivas:** Desacoplamento semântico, cache e resiliência centralizados, contratos versionados, testabilidade independente
- **Negativas:** Latência extra (1 hop adicional), necessidade de governança de versionamento

### Diagrama

Veja [docs/architecture-local.drawio](architecture-local.drawio)

---

## ADR-002: Supervisor Multi-Agente com LangGraph

**Status:** Adotado  
**Data:** 2025-01

### Contexto

A aplicação precisa decidir dinamicamente quais sub-agentes acionar (perfil, transações, base de conhecimento) e compor respostas a partir dos resultados de múltiplos agentes.

### Decisão

Usar **LangGraph** com padrão Supervisor e 7 nós:

```
input_guardrail → planner → [profile | transaction | knowledge] → formatter → output_guardrail
```

O **planner** é um nó LLM que decide quais sub-agentes acionar com base na query. O roteamento condicional garante que somente os agentes necessários executem.

### Alternativas Avaliadas

| Alternativa | Por que descartada |
|---|---|
| LangChain Agent único com todas as tools | Sem controle fino de roteamento, difícil de testar, sem guardrails por etapa |
| CrewAI | Menos controle sobre o grafo, abstração mais alta que dificulta debug |
| Autogen | Focado em chat multi-turn, não em workflows estruturados |
| LangGraph ReAct puro | Sem separação clara de responsabilidades entre sub-agentes |

### Consequências

- **Positivas:** Controle fino do fluxo, guardrails em entrada e saída, sub-agentes isolados e testáveis, routing condicional
- **Negativas:** Complexidade adicional na construção do grafo, curva de aprendizado do LangGraph

---

## ADR-003: Separação Backend (Go) / AI (Python)

**Status:** Adotado  
**Data:** 2025-01

### Contexto

A aplicação tem dois domínios técnicos distintos: backend com resiliência/cache/métricas e orquestração de agentes com LLMs.

### Decisão

- **BFA em Go:** Performance, tipagem forte, excelente para HTTP servers com resiliência (circuit breaker, retry, bulkhead)
- **Agente em Python:** Ecossistema LangChain/LangGraph, libraries de ML/AI maduras, integração nativa com LLMs

### Alternativas Avaliadas

| Alternativa | Por que descartada |
|---|---|
| Tudo em Python | Performance inferior para o BFA, sem go-cache/gobreaker equivalentes nativos |
| Tudo em Go | Ecossistema LangChain inexistente em Go, muito boilerplate para LLM orchestration |
| Java/Spring | Overhead de JVM para o BFA, ecossistema LLM menos maduro que Python |

### Consequências

- **Positivas:** Cada linguagem na sua força, deploy independente, times especializados
- **Negativas:** Dois Dockerfiles, dois conjuntos de dependências, comunicação via rede

---

## ADR-004: MCP como Protocolo de Tool Calling

**Status:** Adotado  
**Data:** 2025-01

### Contexto

Sub-agentes precisam chamar o BFA para obter dados de domínio. A alternativa seria function calling direto com httpx/requests.

### Decisão

Usar **MCP (Model Context Protocol)** via `langchain-mcp-adapters` com transporte **stdio**. O MCP Server (`bfa_server.py`) expõe tools que encapsulam chamadas REST ao BFA Go.

### Alternativas Avaliadas

| Alternativa | Por que descartada |
|---|---|
| Function calling direto (httpx) | Acopla o agente ao formato REST do BFA, sem padronização |
| gRPC | Overhead de protobuf, complexidade desnecessária para 2 endpoints |
| REST direto dentro dos tools | O agente ficaria ciente dos detalhes de URL/headers do BFA |

### Consequências

- **Positivas:** Interface padronizada, o agente só conhece o contrato MCP, desacoplamento do transporte
- **Negativas:** MCP ainda é protocolo emergente, stdio requer spawn de processo

---

## ADR-005: RAG com ChromaDB Local

**Status:** Adotado (desenvolvimento)  
**Data:** 2025-01

### Contexto

O agente de conhecimento precisa consultar uma base de políticas e FAQs do banco.

### Decisão

Usar **ChromaDB** local (in-memory/persistido) com `langchain-community` para indexação e busca por similaridade.

### Alternativas Avaliadas (para produção)

| Alternativa | Quando usar |
|---|---|
| OpenSearch Serverless | Produção — vector search gerenciado, escalável |
| Gemini Long Context | Produção — documentos grandes sem chunking |
| Pinecone | Quando precisa de managed vector DB fora da AWS |

### Consequências

- **Positivas:** Zero infra adicional em dev, setup instantâneo, funciona offline
- **Negativas:** Não escala para produção, sem persistence distribuída

---

## ADR-006: Guardrails em Duas Camadas

**Status:** Adotado  
**Data:** 2025-01

### Contexto

Assistentes bancários lidam com dados sensíveis e precisam de proteção contra prompt injection e vazamento de informações.

### Decisão

Implementar **dois guardrails**:

1. **Input Guardrail:** Sanitização via regex (15 padrões) + classificação de risco via LLM
2. **Output Guardrail:** Verificação da resposta final via LLM (aprovação/reprovação)

Ambos são nós do grafo LangGraph, executados antes e depois do pipeline principal.

### Consequências

- **Positivas:** Proteção em dupla camada, cada guardrail é testável independentemente, fallback seguro
- **Negativas:** 2 chamadas LLM extras por requisição (custo), latência adicional

---

## ADR-007: LangFuse para Observabilidade de LLM

**Status:** Adotado  
**Data:** 2025-01

### Contexto

Precisamos monitorar traces, tokens, custos e latência de cada chamada LLM.

### Decisão

Usar **LangFuse** (self-hosted via Docker) com `CallbackHandler` integrado ao LangChain. Um shim de compatibilidade (`langchain_compat.py`) resolve incompatibilidades entre LangFuse SDK v2 e LangChain v1.x.

### Alternativas Avaliadas

| Alternativa | Por que descartada |
|---|---|
| LangSmith | Proprietário, requer conta LangChain, sem self-hosting |
| OpenTelemetry puro | Não tem suporte nativo para traces de LLM (tokens, custos) |
| Weights & Biases | Mais focado em ML training que em LLM observability |

### Consequências

- **Positivas:** Self-hosted, UI rica, traces detalhados por nó do grafo, custos por modelo
- **Negativas:** Requer Postgres para LangFuse, shim de compatibilidade necessário

---

## ADR-008: Observabilidade do BFA com Prometheus + slog

**Status:** Adotado  
**Data:** 2025-01

### Contexto

O BFA Go precisa expor métricas de performance e health checks.

### Decisão

- **Prometheus** para métricas (`/metrics`)
- **slog** para logging estruturado (JSON)
- **OpenTelemetry** preparado para tracing distribuído
- Health checks: `/healthz`, `/livez`, `/readyz`

### Consequências

- **Positivas:** Stack padrão, integrável com Grafana/AlertManager, zero vendor lock-in
- **Negativas:** Prometheus requer scraping, não push-based

---

## Diferenças Local vs. Produção

| Aspecto | Local | Produção |
|---|---|---|
| **LLM** | OpenAI GPT-4o-mini | Amazon Bedrock (Claude) |
| **RAG** | ChromaDB local | OpenSearch Serverless + Gemini |
| **Auth** | Sem autenticação | Cognito/Keycloak + M2M (mTLS/JWT) |
| **BFA Runtime** | Docker container | Lambda / EKS |
| **Agent Runtime** | Docker container | Bedrock AgentCore |
| **Métricas** | Prometheus local | CloudWatch + X-Ray + Grafana |
| **LLM Observability** | LangFuse self-hosted | LangFuse Cloud |
| **API Gateway** | Direto ao FastAPI | Amazon API Gateway + WAF |
| **Memory** | In-process | AgentCore Memory |

Veja diagramas em:
- [docs/architecture-local.drawio](architecture-local.drawio)
- [docs/architecture-production.drawio](architecture-production.drawio)
