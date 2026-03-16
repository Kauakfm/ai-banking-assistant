"""
Segurança do Agent Python.

Implementa:
- Validação de tamanho de entrada (max 2000 chars)
- Rate limiting por customerId (sliding window in-memory)
- Mascaramento de PII na saída (CPF, email, telefone)
"""

import re
import time
import threading
from collections import defaultdict

MAX_INPUT_LENGTH = 2000
RATE_LIMIT_MAX_REQUESTS = 10
RATE_LIMIT_WINDOW_SECONDS = 60

PII_PATTERNS = [
    (re.compile(r"\b\d{3}\.\d{3}\.\d{3}-\d{2}\b"), "[CPF REDACTED]"),
    (re.compile(r"\b\d{11}\b"), "[CPF REDACTED]"),
    (re.compile(r"\b\d{2}\.\d{3}\.\d{3}/\d{4}-\d{2}\b"), "[CNPJ REDACTED]"),
    (re.compile(r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b"), "[EMAIL REDACTED]"),
    (re.compile(r"\b\d{4}[\s-]\d{4}[\s-]\d{4}[\s-]\d{4}\b"), "[CARD REDACTED]"),
    (re.compile(r"\b\d{16}\b"), "[CARD REDACTED]"),
    (re.compile(r"\+?\d{0,3}\s?\(?\d{2}\)?\s?\d{4,5}[-\s]?\d{4}\b"), "[PHONE REDACTED]"),
]


class InputValidationError(Exception):
    """Erro de validação de entrada."""
    def __init__(self, message: str, status_code: int = 400):
        self.message = message
        self.status_code = status_code
        super().__init__(message)


class RateLimitError(Exception):
    """Erro de rate limiting."""
    def __init__(self, customer_id: str):
        self.message = f"Taxa de requisições excedida para o cliente {customer_id}. Limite: {RATE_LIMIT_MAX_REQUESTS} req/{RATE_LIMIT_WINDOW_SECONDS}s."
        self.status_code = 429
        super().__init__(self.message)


class RateLimiter:
    """
    Rate limiter in-memory com sliding window por customerId.

    Thread-safe para uso em ambiente assíncrono com múltiplas requests.
    """

    def __init__(self, max_requests: int = RATE_LIMIT_MAX_REQUESTS, window_seconds: int = RATE_LIMIT_WINDOW_SECONDS):
        self.max_requests = max_requests
        self.window_seconds = window_seconds
        self._requests: dict[str, list[float]] = defaultdict(list)
        self._lock = threading.Lock()

    def check(self, customer_id: str) -> bool:
        """
        Verifica se o customer_id pode fazer uma nova requisição.
        Retorna True se permitido, levanta RateLimitError se excedido.
        """
        now = time.time()
        cutoff = now - self.window_seconds

        with self._lock:
            self._requests[customer_id] = [
                ts for ts in self._requests[customer_id] if ts > cutoff
            ]

            if len(self._requests[customer_id]) >= self.max_requests:
                raise RateLimitError(customer_id)

            self._requests[customer_id].append(now)
            return True

    def get_remaining(self, customer_id: str) -> int:
        """Retorna quantas requisições restam na janela atual."""
        now = time.time()
        cutoff = now - self.window_seconds

        with self._lock:
            active = [ts for ts in self._requests.get(customer_id, []) if ts > cutoff]
            return max(0, self.max_requests - len(active))


rate_limiter = RateLimiter()


def validate_input_length(text: str) -> None:
    """Valida que o input não excede o tamanho máximo permitido."""
    if len(text) > MAX_INPUT_LENGTH:
        raise InputValidationError(
            f"Input excede o tamanho máximo permitido ({MAX_INPUT_LENGTH} caracteres). "
            f"Tamanho recebido: {len(text)} caracteres."
        )


def mask_pii(text: str) -> str:
    """
    Mascara dados sensíveis (PII) no texto de saída.

    Detecta e substitui: CPF, CNPJ, email, telefone, cartão de crédito.
    """
    masked = text
    for pattern, replacement in PII_PATTERNS:
        masked = pattern.sub(replacement, masked)
    return masked
