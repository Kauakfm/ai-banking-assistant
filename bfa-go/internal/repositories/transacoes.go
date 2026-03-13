package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/domain"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/ports"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/infra/resiliencia"
)

type transacoesHTTP struct {
	baseURL     string
	client      *http.Client
	resiliencia *resiliencia.Wrapper
}

func NovoRepositorioTransacoes(baseURL string, client *http.Client, res *resiliencia.Wrapper) ports.RepositorioTransacoes {
	return &transacoesHTTP{
		baseURL:     baseURL,
		client:      client,
		resiliencia: res,
	}
}

func (r *transacoesHTTP) BuscarTransacoes(ctx context.Context, clienteID string) ([]domain.Transacao, error) {
	var transacoes []domain.Transacao

	err := r.resiliencia.Executar(ctx, "buscar_transacoes", func(ctx context.Context) error {
		url := fmt.Sprintf("%s/api/v1/transactions/%s", r.baseURL, clienteID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		resp, err := r.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return domain.NovoErroNaoEncontrado("transações não encontradas")
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("transactions api retornou status %d", resp.StatusCode)
		}

		return json.NewDecoder(resp.Body).Decode(&transacoes)
	})

	if err != nil {
		return nil, err
	}

	return transacoes, nil
}
