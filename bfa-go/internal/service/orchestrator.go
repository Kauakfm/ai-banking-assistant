package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kauakfm/ai-banking-assistant/pkg/resilience"

	"github.com/sony/gobreaker"
	"golang.org/x/sync/errgroup"
)

// Bulkhead: Limita a 50 requisições simultâneas processadas pelo orquestrador no total
var bulkheadSemaphore = make(chan struct{}, 50)

type Orchestrator struct {
	agentURL   string
	httpClient *http.Client
	cbAgent    *gobreaker.CircuitBreaker
}

func NewOrchestrator(agentURL string) *Orchestrator {
	return &Orchestrator{
		agentURL: agentURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Timeout base de segurança
		},
		cbAgent: resilience.NewCircuitBreaker("Agent-API"),
	}
}

// Estruturas de dados omitidas para brevidade (Profile, Transaction, etc.)

func (o *Orchestrator) ProcessAssistantQuery(ctx context.Context, customerID string, query string) (map[string]interface{}, error) {
	// 1. Bulkhead limit
	select {
	case bulkheadSemaphore <- struct{}{}: // Adquire o token
		defer func() { <-bulkheadSemaphore }() // Libera o token no final
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout esperando por recursos: %w", ctx.Err())
	}

	// 2. Criação do ErrGroup com Contexto derivado
	// Se uma das APIs falhar, as outras são canceladas automaticamente
	g, gCtx := errgroup.WithContext(ctx)

	var profile map[string]interface{}
	var transactions []map[string]interface{}

	// 3. Concorrência: Buscar Perfil (Goroutine 1)
	g.Go(func() error {
		return resilience.DoWithRetry(gCtx, 3, func() error {
			// Mock da chamada à API de Profile
			return mockFetch(gCtx, "Profile API", &profile)
		})
	})

	// 4. Concorrência: Buscar Transações (Goroutine 2)
	g.Go(func() error {
		return resilience.DoWithRetry(gCtx, 3, func() error {
			// Mock da chamada à API de Transactions
			return mockFetch(gCtx, "Transactions API", &transactions)
		})
	})

	// 5. Aguarda todas as chamadas concorrentes
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("falha ao buscar dados do cliente: %w", err)
	}

	// 6. Monta o payload para o Agente Python
	payload := map[string]interface{}{
		"customer_id": customerID,
		"query":       query,
		"financial_context": map[string]interface{}{
			"profile":      profile,
			"transactions": transactions,
		},
	}

	// 7. Chamada ao Agente de IA passando pelo Circuit Breaker
	agentResponse, err := o.cbAgent.Execute(func() (interface{}, error) {
		return o.callAgent(ctx, payload)
	})

	if err != nil {
		return nil, fmt.Errorf("serviço do agente indisponível: %w", err)
	}

	return agentResponse.(map[string]interface{}), nil
}

// callAgent faz a requisição HTTP real para o serviço Python
func (o *Orchestrator) callAgent(ctx context.Context, payload interface{}) (map[string]interface{}, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.agentURL+"/generate", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent API retornou status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func mockFetch(ctx context.Context, apiName string, result interface{}) error {
	select {
	case <-time.After(150 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
