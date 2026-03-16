"""
Testes unitários do middleware de segurança.

Cobre: validação de input, rate limiting (sliding window), mascaramento de PII.
"""

import time
import pytest
from app.security.middleware import (
    validate_input_length,
    mask_pii,
    RateLimiter,
    InputValidationError,
    RateLimitError,
    MAX_INPUT_LENGTH,
)


class TestInputValidation:
    """Testes da validação de tamanho de entrada."""

    def test_input_valido_nao_levanta_erro(self):
        validate_input_length("Qual é o meu saldo?")

    def test_input_vazio_nao_levanta_erro(self):
        validate_input_length("")

    def test_input_no_limite_nao_levanta_erro(self):
        validate_input_length("A" * MAX_INPUT_LENGTH)

    def test_input_excede_limite_levanta_erro(self):
        with pytest.raises(InputValidationError) as exc_info:
            validate_input_length("A" * (MAX_INPUT_LENGTH + 1))
        assert exc_info.value.status_code == 400
        assert "2000" in exc_info.value.message

    def test_input_muito_grande(self):
        with pytest.raises(InputValidationError):
            validate_input_length("x" * 10000)


class TestRateLimiter:
    """Testes do rate limiter com sliding window."""

    def test_permite_requests_dentro_do_limite(self):
        limiter = RateLimiter(max_requests=5, window_seconds=60)
        for _ in range(5):
            assert limiter.check("cust-001") is True

    def test_bloqueia_apos_exceder_limite(self):
        limiter = RateLimiter(max_requests=3, window_seconds=60)
        for _ in range(3):
            limiter.check("cust-001")

        with pytest.raises(RateLimitError) as exc_info:
            limiter.check("cust-001")
        assert exc_info.value.status_code == 429
        assert "cust-001" in exc_info.value.message

    def test_clientes_diferentes_independentes(self):
        limiter = RateLimiter(max_requests=2, window_seconds=60)
        limiter.check("cust-001")
        limiter.check("cust-001")

        assert limiter.check("cust-002") is True

    def test_get_remaining(self):
        limiter = RateLimiter(max_requests=5, window_seconds=60)
        assert limiter.get_remaining("cust-001") == 5

        limiter.check("cust-001")
        assert limiter.get_remaining("cust-001") == 4

        limiter.check("cust-001")
        assert limiter.get_remaining("cust-001") == 3

    def test_sliding_window_expira_requests_antigos(self):
        limiter = RateLimiter(max_requests=2, window_seconds=1)
        limiter.check("cust-001")
        limiter.check("cust-001")

        time.sleep(1.1)

        assert limiter.check("cust-001") is True

    def test_rate_limit_error_message(self):
        error = RateLimitError("cust-123")
        assert "cust-123" in str(error)
        assert error.status_code == 429


class TestPIIMasking:
    """Testes do mascaramento de dados sensíveis."""

    def test_mascara_cpf_formatado(self):
        result = mask_pii("O CPF é 123.456.789-00.")
        assert "123.456.789-00" not in result
        assert "[CPF REDACTED]" in result

    def test_mascara_cpf_numerico(self):
        result = mask_pii("CPF: 12345678900")
        assert "12345678900" not in result
        assert "[CPF REDACTED]" in result

    def test_mascara_cnpj(self):
        result = mask_pii("CNPJ: 12.345.678/0001-00")
        assert "12.345.678/0001-00" not in result
        assert "[CNPJ REDACTED]" in result

    def test_mascara_email(self):
        result = mask_pii("Email: joao.silva@banco.com.br")
        assert "joao.silva@banco.com.br" not in result
        assert "[EMAIL REDACTED]" in result

    def test_mascara_telefone(self):
        result = mask_pii("Telefone: (11) 91234-5678")
        assert "91234-5678" not in result
        assert "[PHONE REDACTED]" in result

    def test_mascara_cartao_credito(self):
        result = mask_pii("Cartão: 1234 5678 9012 3456")
        assert "1234 5678 9012 3456" not in result
        assert "[CARD REDACTED]" in result

    def test_texto_sem_pii_nao_altera(self):
        text = "Olá, qual é o meu saldo bancário?"
        assert mask_pii(text) == text

    def test_mascara_multiplos_pii(self):
        text = "CPF: 123.456.789-00, email: user@test.com"
        result = mask_pii(text)
        assert "123.456.789-00" not in result
        assert "user@test.com" not in result

    def test_texto_vazio(self):
        assert mask_pii("") == ""
