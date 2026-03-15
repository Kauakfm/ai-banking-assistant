package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kauakfm/ai-banking-assistant/config"
	"github.com/kauakfm/ai-banking-assistant/internal/client"
	"github.com/kauakfm/ai-banking-assistant/internal/handler"
	"github.com/kauakfm/ai-banking-assistant/pkg/cache"
	"github.com/kauakfm/ai-banking-assistant/pkg/logger"
	"github.com/kauakfm/ai-banking-assistant/pkg/middleware"
	"github.com/kauakfm/ai-banking-assistant/pkg/resilience"
	"github.com/kauakfm/ai-banking-assistant/pkg/tracing"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()

	log := logger.New(cfg.Log.Level)
	slog.SetDefault(log)

	slog.Info("BFA inicializado — Back-end for Agents",
		"porta", cfg.Server.Port,
		"rastreamento", cfg.Tracing.Enabled,
	)

	// --- Observabilidade centralizada (responsabilidade do BFA) ---
	if cfg.Tracing.Enabled {
		shutdown, err := tracing.Init(cfg.Tracing.ServiceName)
		if err != nil {
			slog.Error("falha ao inicializar rastreamento", "erro", err)
		} else {
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := shutdown(ctx); err != nil {
					slog.Error("erro ao encerrar rastreamento", "erro", err)
				}
			}()
		}
	}

	// --- Resiliência (responsabilidade do BFA) ---
	cbCfg := resilience.CBConfig{
		MaxRequests:      5,
		Interval:         10 * time.Second,
		Timeout:          cfg.Resilience.CBTimeout,
		FailureThreshold: cfg.Resilience.CBFailureThreshold,
	}

	retrier := resilience.NewRetrier(resilience.RetryConfig{
		MaxAttempts: cfg.Resilience.MaxRetries,
		BaseDelay:   cfg.Resilience.RetryBaseDelay,
		MaxDelay:    cfg.Resilience.RetryMaxDelay,
	})

	bulkhead := resilience.NewBulkhead(cfg.Resilience.BulkheadMaxConc)

	// --- Clients de domínio (APIs que o BFA encapsula) ---
	profileClient := client.NewProfileClient(
		cfg.Profile.URL, cfg.Profile.Timeout,
		resilience.NewCircuitBreaker(withName(cbCfg, "profile-api")),
		retrier, log,
	)

	transactionClient := client.NewTransactionClient(
		cfg.Transaction.URL, cfg.Transaction.Timeout,
		resilience.NewCircuitBreaker(withName(cbCfg, "transactions-api")),
		retrier, log,
	)

	// --- Cache e métricas (responsabilidade do BFA) ---
	appCache := cache.New(cfg.Cache.TTL, cfg.Cache.CleanupInterval)
	metrics := middleware.NewMetrics()

	// --- Handlers BFA por domínio ---
	profileH := handler.NewProfileHandler(profileClient, appCache, metrics, log)
	transactionH := handler.NewTransactionHandler(transactionClient, appCache, metrics, log)

	// --- Rotas expostas pelo BFA ---
	mux := http.NewServeMux()

	// Health checks
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})
	mux.Handle("GET /metrics", promhttp.Handler())

	// Contratos estáveis do BFA — operações de domínio expostas aos agentes
	mux.Handle("GET /v1/customers/{customerId}/profile", profileH)
	mux.Handle("GET /v1/customers/{customerId}/transactions", transactionH)

	// --- Middleware stack (responsabilidade do BFA) ---
	var h http.Handler = mux
	h = middleware.InstrumentHTTP(metrics)(h)
	h = middleware.Tracing(h)
	h = middleware.Logging(h)
	h = middleware.Recovery(h)
	h = bulkheadMiddleware(bulkhead)(h)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      h,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		slog.Info("BFA escutando — expondo contratos de domínio para agentes", "porta", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("falha no servidor BFA", "erro", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("sinal de encerramento recebido, drenando conexões...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("encerramento forçado", "erro", err)
	}

	slog.Info("BFA encerrado com sucesso")
}

func withName(cfg resilience.CBConfig, name string) resilience.CBConfig {
	cfg.Name = name
	return cfg
}

func bulkheadMiddleware(b *resilience.Bulkhead) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := b.Acquire(r.Context()); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"muitas requisições simultâneas","code":429}`))
				return
			}
			defer b.Release()
			next.ServeHTTP(w, r)
		})
	}
}
