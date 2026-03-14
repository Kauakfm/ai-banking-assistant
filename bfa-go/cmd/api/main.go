package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/kauakfm/ai-banking-assistant/internal/handler"
	"github.com/kauakfm/ai-banking-assistant/internal/service"
	"github.com/kauakfm/ai-banking-assistant/pkg/cache"
	"github.com/kauakfm/ai-banking-assistant/pkg/observability"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func metricsMiddleware(endpoint string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{w, http.StatusOK}
		next(rw, r)

		duration := time.Since(start).Seconds()
		statusStr := strconv.Itoa(rw.statusCode)

		observability.HttpRequestsTotal.WithLabelValues(r.Method, endpoint, statusStr).Inc()
		observability.HttpRequestDuration.WithLabelValues(r.Method, endpoint).Observe(duration)
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	agentURL := os.Getenv("AGENT_URL")
	if agentURL == "" {
		agentURL = "http://localhost:8000"
	}

	localCache := cache.NewLocalCache(5*time.Minute, 10*time.Minute)
	orchestrator := service.NewOrchestrator(agentURL)
	assistantHandler := handler.NewAssistantHandler(orchestrator, localCache)

	mux := http.NewServeMux()

	mux.Handle("GET /metrics", promhttp.Handler())

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("GET /v1/assistant/{customerId}", metricsMiddleware("/v1/assistant", assistantHandler.ServeHTTP))

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("Iniciando BFA em Go", "porta", 8080)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Erro fatal no servidor", "erro", err.Error())
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Recebido sinal de desligamento, encerrando conexões graciosamente...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Falha ao forçar o desligamento do servidor", "erro", err.Error())
	}

	slog.Info("Servidor encerrado com sucesso.")
}
