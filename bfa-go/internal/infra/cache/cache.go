package cache

import (
	"context"
	"sync"
	"time"

	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/domain"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/ports"
)

type entrada struct {
	valor    *domain.RespostaAssistente
	expiraEm time.Time
}

type cacheMemoria struct {
	mu    sync.RWMutex
	dados map[string]entrada
	ttl   time.Duration
}

func NovoCacheMemoria(ttl time.Duration) ports.Cache {
	c := &cacheMemoria{
		dados: make(map[string]entrada),
		ttl:   ttl,
	}
	go c.limparExpirados()
	return c
}

func (c *cacheMemoria) Obter(_ context.Context, chave string) (*domain.RespostaAssistente, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.dados[chave]
	if !ok || time.Now().After(e.expiraEm) {
		return nil, nil
	}

	return e.valor, nil
}

func (c *cacheMemoria) Salvar(_ context.Context, chave string, valor *domain.RespostaAssistente) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.dados[chave] = entrada{
		valor:    valor,
		expiraEm: time.Now().Add(c.ttl),
	}
	return nil
}

func (c *cacheMemoria) limparExpirados() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		agora := time.Now()
		for chave, e := range c.dados {
			if agora.After(e.expiraEm) {
				delete(c.dados, chave)
			}
		}
		c.mu.Unlock()
	}
}
