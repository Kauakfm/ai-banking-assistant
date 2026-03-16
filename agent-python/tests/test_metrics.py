"""
Testes unitários do módulo de métricas Prometheus.

Cobre: estimativa de custo, registro de tokens, resposta de métricas.
"""

import pytest
from app.metrics import (
    estimate_cost,
    record_token_usage,
    metrics_response,
    MODEL_PRICING,
    TOKENS_INPUT,
    TOKENS_OUTPUT,
    ESTIMATED_COST,
    REQUESTS_TOTAL,
    GUARDRAIL_BLOCKS,
    TOOL_ERRORS,
)


class TestEstimateCost:
    """Testes da estimativa de custo por modelo."""

    def test_custo_gpt4o_mini(self):
        cost = estimate_cost("gpt-4o-mini", 1000, 500)
        expected = (1000 * MODEL_PRICING["gpt-4o-mini"]["input"]) + \
                   (500 * MODEL_PRICING["gpt-4o-mini"]["output"])
        assert cost == pytest.approx(expected)

    def test_custo_gpt4o(self):
        cost = estimate_cost("gpt-4o", 1000, 500)
        expected = (1000 * MODEL_PRICING["gpt-4o"]["input"]) + \
                   (500 * MODEL_PRICING["gpt-4o"]["output"])
        assert cost == pytest.approx(expected)

    def test_custo_gpt4(self):
        cost = estimate_cost("gpt-4", 100, 50)
        expected = (100 * MODEL_PRICING["gpt-4"]["input"]) + \
                   (50 * MODEL_PRICING["gpt-4"]["output"])
        assert cost == pytest.approx(expected)

    def test_custo_modelo_desconhecido_usa_fallback(self):
        """Modelo desconhecido usa pricing do gpt-4o-mini como fallback."""
        cost_unknown = estimate_cost("unknown-model", 1000, 500)
        cost_mini = estimate_cost("gpt-4o-mini", 1000, 500)
        assert cost_unknown == pytest.approx(cost_mini)

    def test_custo_zero_tokens(self):
        cost = estimate_cost("gpt-4o-mini", 0, 0)
        assert cost == 0.0

    def test_custo_proporcional(self):
        """O custo deve ser proporcional ao número de tokens."""
        cost_1x = estimate_cost("gpt-4o-mini", 100, 50)
        cost_2x = estimate_cost("gpt-4o-mini", 200, 100)
        assert cost_2x == pytest.approx(cost_1x * 2)


class TestRecordTokenUsage:
    """Testes do registro de tokens."""

    def test_record_token_usage_incrementa_contadores(self):
        """Verifica que os contadores são incrementados."""
        input_before = TOKENS_INPUT._value.get()
        output_before = TOKENS_OUTPUT._value.get()
        cost_before = ESTIMATED_COST._value.get()

        record_token_usage("gpt-4o-mini", 100, 50)

        assert TOKENS_INPUT._value.get() == input_before + 100
        assert TOKENS_OUTPUT._value.get() == output_before + 50
        assert ESTIMATED_COST._value.get() > cost_before


class TestMetricsResponse:
    """Testes da resposta HTTP de métricas."""

    def test_metrics_response_retorna_bytes_e_content_type(self):
        body, content_type = metrics_response()
        assert isinstance(body, bytes)
        assert "text/plain" in content_type or "openmetrics" in content_type

    def test_metrics_response_contem_metricas(self):
        body, _ = metrics_response()
        text = body.decode("utf-8")
        assert "agent_request_latency_seconds" in text
        assert "agent_tokens_input_total" in text
        assert "agent_tokens_output_total" in text
        assert "agent_estimated_cost_usd_total" in text


class TestPrometheuCounters:
    """Testes dos contadores Prometheus."""

    def test_requests_total_labels(self):
        """Verifica que o counter aceita labels válidos."""
        before = REQUESTS_TOTAL.labels(status="test")._value.get()
        REQUESTS_TOTAL.labels(status="test").inc()
        assert REQUESTS_TOTAL.labels(status="test")._value.get() == before + 1

    def test_guardrail_blocks_labels(self):
        before = GUARDRAIL_BLOCKS.labels(guardrail_type="test")._value.get()
        GUARDRAIL_BLOCKS.labels(guardrail_type="test").inc()
        assert GUARDRAIL_BLOCKS.labels(guardrail_type="test")._value.get() == before + 1

    def test_tool_errors_labels(self):
        before = TOOL_ERRORS.labels(tool_name="test_tool")._value.get()
        TOOL_ERRORS.labels(tool_name="test_tool").inc()
        assert TOOL_ERRORS.labels(tool_name="test_tool")._value.get() == before + 1
