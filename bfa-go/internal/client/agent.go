package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/kauakfm/ai-banking-assistant/internal/domain"
	pkgerr "github.com/kauakfm/ai-banking-assistant/pkg/errors"
	"github.com/sony/gobreaker"
)

type AgentClient struct {
	baseURL string
	http    *http.Client
	cb      *gobreaker.CircuitBreaker
	log     *slog.Logger
}

func NewAgentClient(baseURL string, timeout time.Duration, cb *gobreaker.CircuitBreaker, log *slog.Logger) *AgentClient {
	if log == nil {
		log = slog.Default()
	}
	return &AgentClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout},
		cb:      cb,
		log:     log,
	}
}

func (c *AgentClient) Generate(ctx context.Context, customerID, prompt string) (*domain.AgentResponse, error) {
	body, err := json.Marshal(domain.AgentRequest{
		CustomerID: customerID,
		Query:      prompt,
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar requisição do agente: %w", err)
	}

	result, err := c.cb.Execute(func() (any, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/generate", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", pkgerr.ErrAgentUnavailable, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%w: agente retornou status %d", pkgerr.ErrAgentUnavailable, resp.StatusCode)
		}

		var agentResp domain.AgentResponse
		if err := json.NewDecoder(resp.Body).Decode(&agentResp); err != nil {
			return nil, fmt.Errorf("erro ao decodificar resposta do agente: %w", err)
		}
		return &agentResp, nil
	})

	if err != nil {
		c.log.WarnContext(ctx, "chamada ao agente falhou", "erro", err)
		return nil, err
	}
	return result.(*domain.AgentResponse), nil
}
