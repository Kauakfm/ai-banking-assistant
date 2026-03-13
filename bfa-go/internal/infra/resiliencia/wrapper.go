package resiliencia

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/sony/gobreaker/v2"
)

type Wrapper struct {
	cb            *gobreaker.CircuitBreaker[any]
	maxTentativas int
	bulkhead      chan struct{}
}

func NovoWrapper(maxTentativas int, cbLimiteFalhas int, maxConcorrencia int) *Wrapper {
	cb := gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name:        "bfa-cb",
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return int(counts.ConsecutiveFailures) >= cbLimiteFalhas
		},
	})

	return &Wrapper{
		cb:            cb,
		maxTentativas: maxTentativas,
		bulkhead:      make(chan struct{}, maxConcorrencia),
	}
}

func (w *Wrapper) Executar(ctx context.Context, operacao string, fn func(ctx context.Context) error) error {
	select {
	case w.bulkhead <- struct{}{}:
		defer func() { <-w.bulkhead }()
	case <-ctx.Done():
		return ctx.Err()
	}

	_, err := w.cb.Execute(func() (any, error) {
		var ultimoErro error
		for tentativa := 0; tentativa < w.maxTentativas; tentativa++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			if err := fn(ctx); err != nil {
				ultimoErro = err
				if tentativa < w.maxTentativas-1 {
					espera := calcularBackoff(tentativa)
					select {
					case <-time.After(espera):
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				}
				continue
			}
			return nil, nil
		}
		return nil, ultimoErro
	})

	return err
}

func calcularBackoff(tentativa int) time.Duration {
	base := math.Pow(2, float64(tentativa)) * 100
	jitter := rand.Float64() * base * 0.5
	return time.Duration(base+jitter) * time.Millisecond
}
