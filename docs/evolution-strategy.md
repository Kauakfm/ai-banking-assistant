# Estratégia de Evolução Futura — AI Banking Assistant

> Roadmap técnico de evolução do projeto, com foco em controle de acesso por ferramenta, governança e escalabilidade do padrão BFA.

---

## 1. Controle de Acesso por customerId (Tool-Level Authorization)

### Problema Atual

Hoje, qualquer requisição com um `customer_id` válido pode acessar qualquer ferramenta (get_customer_profile, get_customer_transactions, rag_search). Não existe validação de que o solicitante tem permissão para ver os dados daquele `customer_id` específico.

### Solução Proposta

Implementar **controle de acesso por ferramenta** no BFA, validando a relação entre o token JWT do solicitante e o `customer_id` da requisição.

```
Agente → MCP Tool → BFA → [Validação de Acesso] → API de Domínio
                           ↑
                    O BFA verifica:
                    1. JWT do solicitante é válido?
                    2. O solicitante tem permissão para acessar este customer_id?
                    3. A ferramenta solicitada é permitida para este perfil?
```

### Implementação no BFA Go

```go
// middleware/authorization.go

type ToolPolicy struct {
    AllowedTools   []string  // Ferramentas permitidas para este perfil
    AllowedScopes  []string  // Escopos do token (read:profile, read:transactions)
    CustomerAccess string    // "self" | "team" | "all"
}

func AuthorizeToolCall(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. Extrair customer_id da URL
        customerID := chi.URLParam(r, "customerId")
        
        // 2. Extrair subject do JWT
        claims := r.Context().Value("claims").(JWTClaims)
        
        // 3. Verificar se subject pode acessar este customer_id
        if claims.CustomerAccess == "self" && claims.Subject != customerID {
            http.Error(w, "Acesso negado: você só pode acessar seus próprios dados", 403)
            return
        }
        
        // 4. Verificar se a ferramenta solicitada é permitida
        tool := extractToolFromPath(r.URL.Path)
        if !contains(claims.AllowedTools, tool) {
            http.Error(w, "Ferramenta não autorizada para este perfil", 403)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}
```

### Matriz de Acesso

| Perfil | get_customer_profile | get_customer_transactions | rag_search | Acesso |
|---|---|---|---|---|
| **Cliente** | ✅ (self) | ✅ (self) | ✅ | Apenas dados próprios |
| **Atendente** | ✅ (team) | ✅ (team) | ✅ | Dados dos clientes da carteira |
| **Gerente** | ✅ (all) | ✅ (all) | ✅ | Todos os clientes da agência |
| **Auditor** | ✅ (all) | ✅ (all) | ❌ | Somente dados, sem KB |

---

## 2. Versionamento de Contratos MCP

### Problema

Quando o BFA evolui (novos campos, campos removidos), os agentes podem quebrar se não houver versionamento.

### Solução

Versionar os tools MCP e manter compatibilidade retroativa:

```python
# bfa_server.py — versão atual

@mcp.tool()
async def get_customer_profile_v2(customer_id: str, include_credit_score: bool = False) -> str:
    """
    Versão 2: inclui opção de credit score.
    Retrocompatível com v1 (include_credit_score default = False).
    """
    url = f"{BFA_GO_URL}/v2/customers/{customer_id}/profile"
    params = {"include_credit_score": include_credit_score}
    ...
```

**Política de deprecação:**
1. Nova versão do tool é adicionada (v2)
2. Versão anterior (v1) recebe log de deprecation warning
3. Após 2 sprints, v1 é removida

---

## 3. Novos Sub-Agentes

### Credit Agent

```python
# subagents/credit_agent.py

def create_credit_agent(mcp_tools):
    credit_tools = [t for t in mcp_tools if t.name == "get_credit_analysis"]
    
    async def credit_agent_node(state, config=None):
        # Analisa score de crédito, limites, simulações
        ...
    
    return credit_agent_node
```

**Tools MCP necessárias:**
- `get_credit_analysis` — Score, limite disponível, parcelas
- `simulate_loan` — Simulação de empréstimo

### Investment Agent

**Tools MCP necessárias:**
- `get_investment_portfolio` — Carteira atual
- `get_investment_recommendations` — Sugestões baseadas no perfil

### Fraud Detection Agent

**Tools MCP necessárias:**
- `check_transaction_risk` — Score de risco por transação
- `get_fraud_alerts` — Alertas ativos

---

## 4. Orquestração Avançada

### Planner com Instruções Mais Ricas

Evolução do planner para incluir:
- **Priorização** de agentes (qual resolver primeiro)
- **Paralelização** real (LangGraph `parallel` branches)
- **Budget de tokens** por execução

```python
# Exemplo de plano estruturado
{
    "agents": [
        {"name": "profile", "priority": 1, "budget_tokens": 500},
        {"name": "transactions", "priority": 1, "budget_tokens": 1000},
        {"name": "knowledge", "priority": 2, "budget_tokens": 300},
    ],
    "parallel_groups": [[0, 1], [2]],  # groups 0,1 em paralelo, depois 2
    "plan": "Buscar perfil e transações em paralelo, depois consultar KB."
}
```

### Human-in-the-Loop

Para operações sensíveis (transferências, cancelamentos), adicionar nó de aprovação humana:

```
planner → [precisa aprovação?] → human_approval_node → agent
                                        ↑
                              Envia notificação para
                              o gerente aprovar
```

---

## 5. Governança e Compliance

### Audit Log por Tool Call

Todo tool call registrado com:
- Timestamp
- customer_id
- tool_name
- tool_args (sanitizados)
- Resultado (success/failure)
- Solicitante (JWT subject)
- Trace ID

```go
// logger/audit.go

type AuditEntry struct {
    Timestamp   time.Time `json:"timestamp"`
    TraceID     string    `json:"trace_id"`
    CustomerID  string    `json:"customer_id"`
    Requester   string    `json:"requester"`
    Tool        string    `json:"tool"`
    Args        string    `json:"args"`
    Result      string    `json:"result"`
    Duration    int64     `json:"duration_ms"`
}
```

### Data Retention

- Traces de LLM: 90 dias
- Audit logs: 5 anos (compliance bancário)
- Métricas: 15 meses (CloudWatch default)

### LGPD / Privacy

- Dados pessoais mascarados nos logs
- Opt-out de traces para clientes que solicitem
- Right to erasure: script para purgar dados de um customer_id

---

## 6. Performance

### Caching Inteligente

Evolução do cache no BFA:
- **L1:** In-memory (go-cache) — TTL 5min
- **L2:** Redis (ElastiCache) — TTL 30min
- **Invalidação:** Event-driven via SQS/SNS quando dados do core mudam

### Connection Pooling

- Pool de conexões HTTP para APIs de domínio
- Pre-warm de conexões no startup
- Keep-alive com health checks

### Batch Processing

Para operações que envolvem múltiplos clientes (relatórios gerenciais):
- Endpoint batch no BFA
- Rate limiting por tier
- Background processing com SQS

---

## 7. Roadmap

| Fase | Entregáveis | Timeline |
|---|---|---|
| **v1.0 (Atual)** | Multi-agent + BFA + RAG + Guardrails + LangFuse | ✅ |
| **v1.1** | Controle de acesso por customerId | Próximo sprint |
| **v1.2** | Versionamento de contratos MCP | +2 sprints |
| **v2.0** | Credit Agent + Investment Agent | +1 quarter |
| **v2.1** | Human-in-the-loop + Planner avançado | +1 quarter |
| **v3.0** | Produção AWS (AgentCore + Bedrock) | +2 quarters |
| **v3.1** | Fraud Detection Agent + Audit | +1 quarter |

---

## Referências

- [BFA Pattern](bfa-pattern.md)
- [Decisões Arquiteturais](architectural-decisions.md)
- [Arquitetura de Produção](production-architecture.md)
