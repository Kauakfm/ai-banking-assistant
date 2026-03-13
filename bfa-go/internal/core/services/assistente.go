package services

import (
	"context"
	"sync"

	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/domain"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/ports"
)

type servicoAssistente struct {
	repoPerfil     ports.RepositorioPerfil
	repoTransacoes ports.RepositorioTransacoes
	repoAgente     ports.RepositorioAgente
	cache          ports.Cache
}

func NovoServicoAssistente(
	repoPerfil ports.RepositorioPerfil,
	repoTransacoes ports.RepositorioTransacoes,
	repoAgente ports.RepositorioAgente,
	cache ports.Cache,
) ports.ServicoAssistente {
	return &servicoAssistente{
		repoPerfil:     repoPerfil,
		repoTransacoes: repoTransacoes,
		repoAgente:     repoAgente,
		cache:          cache,
	}
}

func (s *servicoAssistente) ConsultarAssistente(ctx context.Context, clienteID string) (*domain.RespostaAssistente, error) {
	if clienteID == "" {
		return nil, domain.NovoErroValidacao("cliente_id é obrigatório")
	}

	if cached, err := s.cache.Obter(ctx, clienteID); err == nil && cached != nil {
		cached.CacheHit = true
		return cached, nil
	}

	var (
		perfil     *domain.Perfil
		transacoes []domain.Transacao
		errPerfil  error
		errTxn     error
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		perfil, errPerfil = s.repoPerfil.BuscarPerfil(ctx, clienteID)
	}()

	go func() {
		defer wg.Done()
		transacoes, errTxn = s.repoTransacoes.BuscarTransacoes(ctx, clienteID)
	}()

	wg.Wait()

	if errPerfil != nil {
		return nil, domain.NovoErroServicoIndisponivel("falha ao buscar perfil", errPerfil)
	}

	if errTxn != nil {
		return nil, domain.NovoErroServicoIndisponivel("falha ao buscar transações", errTxn)
	}

	respostaAgente, err := s.repoAgente.ConsultarAgente(ctx, perfil, transacoes)
	if err != nil {
		respostaAgente = &domain.RespostaAgente{
			Recomendacao:  "Serviço de IA temporariamente indisponível. Tente novamente em instantes.",
			Justificativa: "Fallback automático - agente não respondeu",
			Confianca:     0,
			Metadata: domain.MetadataResposta{
				FerramentasUsadas: []string{"fallback"},
			},
		}
	}

	resposta := &domain.RespostaAssistente{
		ClienteID:  clienteID,
		Perfil:     *perfil,
		Transacoes: transacoes,
		Agente:     *respostaAgente,
		CacheHit:   false,
	}

	_ = s.cache.Salvar(ctx, clienteID, resposta)

	return resposta, nil
}
