package resilience

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/sony/gobreaker"
)

// Configura e retorna um novo Circuit Breaker
func NewCircuitBreaker(name string) *gobreaker.CircuitBreaker {
	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: 5,                // Máximo de requisições permitidas no estado Half-Open
		Interval:    10 * time.Second, // Tempo de limpeza do contador de falhas
		Timeout:     5 * time.Second,  // Tempo que o CB fica Open antes de tentar Half-Open
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Abre o circuito se houver mais de 3 falhas consecutivas
			return counts.ConsecutiveFailures > 3
		},
	}
	return gobreaker.NewCircuitBreaker(settings)
}

// DoWithRetry executa uma função com Exponential Backoff
func DoWithRetry(ctx context.Context, maxRetries int, operation func() error) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = operation()
		if err == nil {
			return nil
		}

		// Se o contexto foi cancelado (ex: timeout), abortamos o retry
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ctx.Err()
		}

		// Exponential backoff: 200ms, 400ms, 800ms...
		backoffDuration := time.Duration(math.Pow(2, float64(i))) * 200 * time.Millisecond

		select {
		case <-time.After(backoffDuration):
			// Esperou o tempo, tenta de novo
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}
