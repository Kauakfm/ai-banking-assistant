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

type ProfileClient struct {
	baseURL string
	http    *http.Client
	cb      *gobreaker.CircuitBreaker
	retry   *resilience.Retrier
	log     *slog.Logger
}

func NewProfileClient(baseURL string, timeout time.Duration, cb *gobreaker.CircuitBreaker, retry *resilience.Retrier, log *slog.Logger) *ProfileClient {
	if log == nil {
		log = slog.Default()
	}
	return &ProfileClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout},
		cb:      cb,
		retry:   retry,
		log:     log,
	}
}

func (c *ProfileClient) GetByID(ctx context.Context, customerID string) (*domain.Profile, error) {
	if c.baseURL == "" {
		c.log.InfoContext(ctx, "API de perfil não configurada, usando mock", "id_cliente", customerID)
		return mockProfile(customerID), nil
	}

	profile, err := c.fetchProfile(ctx, customerID)
	if err != nil {
		c.log.WarnContext(ctx, "API de perfil falhou, usando mock", "id_cliente", customerID, "erro", err)
		return mockProfile(customerID), nil
	}
	return profile, nil
}

func (c *ProfileClient) fetchProfile(ctx context.Context, customerID string) (*domain.Profile, error) {
	url := fmt.Sprintf("%s/customers/%s", c.baseURL, customerID)
	var profile *domain.Profile

	err := c.retry.Do(ctx, "profile-"+customerID, func() error {
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

			if resp.StatusCode == http.StatusNotFound {
				return nil, pkgerr.ErrNotFound
			}
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("API de perfil retornou status %d", resp.StatusCode)
			}

			var p domain.Profile
			if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
				return nil, fmt.Errorf("erro ao decodificar perfil: %w", err)
			}
			return &p, nil
		})
		if cbErr != nil {
			return cbErr
		}
		profile = result.(*domain.Profile)
		return nil
	})
	return profile, err
}

func mockProfile(customerID string) *domain.Profile {
	return &domain.Profile{
		ID:        customerID,
		Name:      "Cliente Simulado",
		Email:     "cliente@bfa.ao",
		Segment:   "premium",
		AccountID: "AO06.0006.0000.0000.0000.0024.7",
		CreatedAt: time.Now().AddDate(-2, 0, 0),
	}
}
