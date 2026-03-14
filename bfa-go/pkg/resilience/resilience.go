package resilience

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"github.com/sony/gobreaker"
)

type CBConfig struct {
	Name             string
	MaxRequests      uint32
	Interval         time.Duration
	Timeout          time.Duration
	FailureThreshold uint32
}

func NewCircuitBreaker(cfg CBConfig) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cfg.FailureThreshold
		},
	})
}

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

type Retrier struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
}

func NewRetrier(cfg RetryConfig) *Retrier {
	return &Retrier{
		maxAttempts: cfg.MaxAttempts,
		baseDelay:   cfg.BaseDelay,
		maxDelay:    cfg.MaxDelay,
	}
}

func (r *Retrier) Do(ctx context.Context, operation string, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < r.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := fn(); err != nil {
			lastErr = err
			if attempt < r.maxAttempts-1 {
				delay := r.backoff(attempt)
				slog.WarnContext(ctx, "nova tentativa agendada",
					"operacao", operation,
					"tentativa", attempt+1,
					"max_tentativas", r.maxAttempts,
					"atraso", delay,
					"erro", err,
				)
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			continue
		}
		return nil
	}
	return lastErr
}

func (r *Retrier) backoff(attempt int) time.Duration {
	delay := float64(r.baseDelay) * math.Pow(2, float64(attempt))
	jitter := rand.Float64() * float64(r.baseDelay)
	delay += jitter
	if delay > float64(r.maxDelay) {
		delay = float64(r.maxDelay)
	}
	return time.Duration(delay)
}

type Bulkhead struct {
	sem chan struct{}
}

func NewBulkhead(maxConcurrency int) *Bulkhead {
	return &Bulkhead{sem: make(chan struct{}, maxConcurrency)}
}

func (b *Bulkhead) Acquire(ctx context.Context) error {
	select {
	case b.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrBulkheadFull
	}
}

func (b *Bulkhead) Release() {
	<-b.sem
}

var ErrBulkheadFull = errors.New("muitas requisições simultâneas")
