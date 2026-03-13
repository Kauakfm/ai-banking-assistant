package ports

import (
	"context"

	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/domain"
)

type ServicoAssistente interface {
	ConsultarAssistente(ctx context.Context, clienteID string) (*domain.RespostaAssistente, error)
}
