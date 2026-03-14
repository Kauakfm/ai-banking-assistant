package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kauakfm/ai-banking-assistant/pkg/resilience"
)

func TestCircuitBreaker_Created(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CBConfig{
		Name:             "test",
		MaxRequests:      5,
		FailureThreshold: 3,
	})
	if cb == nil {
		t.Fatal("circuit breaker não deveria ser nil")
	}
}

func TestRetrier_SuccessFirstAttempt(t *testing.T) {
	r := resilience.NewRetrier(resilience.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
	})

	calls := 0
	err := r.Do(context.Background(), "test", func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("esperado sucesso, obteve: %v", err)
	}
	if calls != 1 {
		t.Errorf("esperado 1 chamada, obteve %d", calls)
	}
}

func TestRetrier_SuccessAfterRetries(t *testing.T) {
	r := resilience.NewRetrier(resilience.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
	})

	calls := 0
	err := r.Do(context.Background(), "test", func() error {
		calls++
		if calls < 3 {
			return errors.New("falha temporária")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("esperado sucesso na terceira tentativa, obteve: %v", err)
	}
	if calls != 3 {
		t.Errorf("esperado 3 chamadas, obteve %d", calls)
	}
}

func TestRetrier_AllAttemptsFail(t *testing.T) {
	r := resilience.NewRetrier(resilience.RetryConfig{
		MaxAttempts: 2,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
	})

	err := r.Do(context.Background(), "test", func() error {
		return errors.New("falha permanente")
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRetrier_ContextCancelled(t *testing.T) {
	r := resilience.NewRetrier(resilience.RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := r.Do(ctx, "test", func() error {
		return errors.New("continua falhando")
	})
	if err == nil {
		t.Fatal("esperado erro")
	}
}

func TestBulkhead_AcquireRelease(t *testing.T) {
	b := resilience.NewBulkhead(2)

	if err := b.Acquire(context.Background()); err != nil {
		t.Fatalf("primeiro acquire falhou: %v", err)
	}
	if err := b.Acquire(context.Background()); err != nil {
		t.Fatalf("segundo acquire falhou: %v", err)
	}

	err := b.Acquire(context.Background())
	if !errors.Is(err, resilience.ErrBulkheadFull) {
		t.Errorf("esperado ErrBulkheadFull, obteve: %v", err)
	}

	b.Release()
	if err := b.Acquire(context.Background()); err != nil {
		t.Fatalf("acquire após release falhou: %v", err)
	}
}

func TestBulkhead_ContextCancelled(t *testing.T) {
	b := resilience.NewBulkhead(1)
	b.Acquire(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := b.Acquire(ctx)
	if err == nil {
		t.Fatal("esperado erro com contexto cancelado")
	}
	b.Release()
}
