package repositories

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/domain"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/ports"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/infra/resiliencia"
)

type agenteHTTP struct {
	baseURL     string
	client      *http.Client
	resiliencia *resiliencia.Wrapper
}

func NovoRepositorioAgente(baseURL string, client *http.Client, res *resiliencia.Wrapper) ports.RepositorioAgente {
	return &agenteHTTP{
		baseURL:     baseURL,
		client:      client,
		resiliencia: res,
	}
}

type requisicaoAgente struct {
	Perfil     *domain.Perfil     `json:"perfil"`
	Transacoes []domain.Transacao `json:"transacoes"`
}

func (r *agenteHTTP) ConsultarAgente(ctx context.Context, perfil *domain.Perfil, transacoes []domain.Transacao) (*domain.RespostaAgente, error) {
	var resposta domain.RespostaAgente

	err := r.resiliencia.Executar(ctx, "consultar_agente", func(ctx context.Context) error {
		body, err := json.Marshal(requisicaoAgente{
			Perfil:     perfil,
			Transacoes: transacoes,
		})
		if err != nil {
			return err
		}

		url := fmt.Sprintf("%s/api/v1/agent/invoke", r.baseURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := r.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("agent service retornou status %d", resp.StatusCode)
		}

		return json.NewDecoder(resp.Body).Decode(&resposta)
	})

	if err != nil {
		return nil, err
	}

	return &resposta, nil
}
