package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/kauakfm/ai-banking-assistant/internal/domain"
	pkgerr "github.com/kauakfm/ai-banking-assistant/pkg/errors"
	"github.com/kauakfm/ai-banking-assistant/pkg/resilience"
	"github.com/sony/gobreaker"
)

type TransactionClient struct {
	baseURL string
	http    *http.Client
	cb      *gobreaker.CircuitBreaker
	retry   *resilience.Retrier
	log     *slog.Logger
}

func NewTransactionClient(baseURL string, timeout time.Duration, cb *gobreaker.CircuitBreaker, retry *resilience.Retrier, log *slog.Logger) *TransactionClient {
	if log == nil {
		log = slog.Default()
	}
	return &TransactionClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout},
		cb:      cb,
		retry:   retry,
		log:     log,
	}
}

func (c *TransactionClient) GetByCustomerID(ctx context.Context, customerID string) ([]domain.Transaction, error) {
	if c.baseURL == "" {
		c.log.InfoContext(ctx, "API de transações não configurada, usando mock", "id_cliente", customerID)
		return mockTransactions(), nil
	}

	txns, err := c.fetchTransactions(ctx, customerID)
	if err != nil {
		c.log.WarnContext(ctx, "API de transações falhou, usando mock", "id_cliente", customerID, "erro", err)
		return mockTransactions(), nil
	}
	return txns, nil
}

func (c *TransactionClient) fetchTransactions(ctx context.Context, customerID string) ([]domain.Transaction, error) {
	url := fmt.Sprintf("%s/customers/%s/transactions", c.baseURL, customerID)
	var transactions []domain.Transaction

	err := c.retry.Do(ctx, "transactions-"+customerID, func() error {
		result, cbErr := c.cb.Execute(func() (any, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, err
			}

			resp, err := c.http.Do(req)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", pkgerr.ErrUnavailable, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("API de transações retornou status %d", resp.StatusCode)
			}

			var t []domain.Transaction
			if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
				return nil, fmt.Errorf("erro ao decodificar transações: %w", err)
			}
			return t, nil
		})
		if cbErr != nil {
			return cbErr
		}
		transactions = result.([]domain.Transaction)
		return nil
	})
	return transactions, err
}

func mockTransactions() []domain.Transaction {
	now := time.Now()
	return []domain.Transaction{
		{ID: "txn-001", Date: now.AddDate(0, 0, -1), Description: "Supermercado Kero", Amount: -15000.00, Category: "alimentação", Type: "débito"},
		{ID: "txn-002", Date: now.AddDate(0, 0, -2), Description: "Salário Mensal", Amount: 350000.00, Category: "salário", Type: "crédito"},
		{ID: "txn-003", Date: now.AddDate(0, 0, -3), Description: "ENDE - Eletricidade", Amount: -8500.00, Category: "utilidades", Type: "débito"},
		{ID: "txn-004", Date: now.AddDate(0, 0, -5), Description: "Transferência PIX", Amount: -25000.00, Category: "transferência", Type: "débito"},
		{ID: "txn-005", Date: now.AddDate(0, 0, -7), Description: "Restaurante Caçador", Amount: -12000.00, Category: "alimentação", Type: "débito"},
	}
}
