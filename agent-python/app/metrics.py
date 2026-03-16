"""
Métricas Prometheus do Agent Python.

Expõe métricas reais de latência, tokens e custo estimado
por request, coletáveis via /metrics.
"""

from prometheus_client import Counter, Histogram, generate_latest, CONTENT_TYPE_LATEST

MODEL_PRICING = {
    "gpt-4o-mini": {"input": 0.150 / 1_000_000, "output": 0.600 / 1_000_000},
    "gpt-4o": {"input": 2.50 / 1_000_000, "output": 10.00 / 1_000_000},
    "gpt-4": {"input": 30.00 / 1_000_000, "output": 60.00 / 1_000_000},
    "gpt-3.5-turbo": {"input": 0.50 / 1_000_000, "output": 1.50 / 1_000_000},
}

REQUEST_LATENCY = Histogram(
    "agent_request_latency_seconds",
    "Latência de cada request ao agente (segundos).",
    buckets=[0.5, 1.0, 2.0, 3.0, 5.0, 10.0, 20.0, 30.0, 60.0],
)

TOKENS_INPUT = Counter(
    "agent_tokens_input_total",
    "Total de tokens de entrada consumidos pelo agente.",
)

TOKENS_OUTPUT = Counter(
    "agent_tokens_output_total",
    "Total de tokens de saída gerados pelo agente.",
)

ESTIMATED_COST = Counter(
    "agent_estimated_cost_usd_total",
    "Custo estimado acumulado em USD.",
)

REQUESTS_TOTAL = Counter(
    "agent_requests_total",
    "Total de requests processados pelo agente.",
    ["status"],
)

GUARDRAIL_BLOCKS = Counter(
    "agent_guardrail_blocks_total",
    "Total de requests bloqueados por guardrails.",
    ["guardrail_type"],
)

TOOL_ERRORS = Counter(
    "agent_tool_errors_total",
    "Total de erros por ferramenta.",
    ["tool_name"],
)


def estimate_cost(model: str, input_tokens: int, output_tokens: int) -> float:
    """Calcula custo estimado com base no modelo e tokens."""
    pricing = MODEL_PRICING.get(model, MODEL_PRICING["gpt-4o-mini"])
    return (input_tokens * pricing["input"]) + (output_tokens * pricing["output"])


def record_token_usage(model: str, input_tokens: int, output_tokens: int):
    """Registra tokens consumidos e custo estimado."""
    TOKENS_INPUT.inc(input_tokens)
    TOKENS_OUTPUT.inc(output_tokens)
    cost = estimate_cost(model, input_tokens, output_tokens)
    ESTIMATED_COST.inc(cost)


def metrics_response():
    """Gera resposta HTTP com métricas Prometheus."""
    return generate_latest(), CONTENT_TYPE_LATEST
