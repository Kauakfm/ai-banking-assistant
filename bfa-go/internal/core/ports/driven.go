package ports

import (
	"context"

	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/domain"
)

type RepositorioPerfil interface {
	BuscarPerfil(ctx context.Context, clienteID string) (*domain.Perfil, error)
}

type RepositorioTransacoes interface {
	BuscarTransacoes(ctx context.Context, clienteID string) ([]domain.Transacao, error)
}

type RepositorioAgente interface {
	ConsultarAgente(ctx context.Context, perfil *domain.Perfil, transacoes []domain.Transacao) (*domain.RespostaAgente, error)
}

type Cache interface {
	Obter(ctx context.Context, chave string) (*domain.RespostaAssistente, error)
	Salvar(ctx context.Context, chave string, valor *domain.RespostaAssistente) error
}
