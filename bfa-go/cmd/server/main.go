package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/services"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/handlers"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/infra/cache"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/infra/config"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/infra/observabilidade"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/infra/resiliencia"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/repositories"
)

func main() {
	cfg := config.Carregar()
	logger := observabilidade.NovoLogger(cfg.LogLevel)
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tp, err := observabilidade.InicializarTracing(ctx)
	if err != nil {
		logger.Fatal("falha ao inicializar tracing", zap.Error(err))
	}
	defer tp.Shutdown(ctx)

	metricas := observabilidade.NovasMetricas()

	httpClient := &http.Client{Timeout: cfg.RequestTimeout}
	res := resiliencia.NovoWrapper(cfg.RetryMaxTentativas, cfg.CBLimiteFalhas, cfg.MaxConcorrencia)

	repoPerfil := repositories.NovoRepositorioPerfil(cfg.ProfileAPIURL, httpClient, res)
	repoTransacoes := repositories.NovoRepositorioTransacoes(cfg.TransactionsAPIURL, httpClient, res)
	repoAgente := repositories.NovoRepositorioAgente(cfg.AgentServiceURL, httpClient, res)
	cacheImpl := cache.NovoCacheMemoria(cfg.CacheTTL)

	servico := services.NovoServicoAssistente(repoPerfil, repoTransacoes, repoAgente, cacheImpl)

	assistenteHandler := handlers.NovoAssistenteHandler(servico)
	saudeHandler := handlers.NovoSaudeHandler()

	router := handlers.NovoRouter(assistenteHandler, saudeHandler, logger, metricas)

	servidor := &http.Server{
		Addr:         ":" + cfg.Porta,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: cfg.RequestTimeout + 5*time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		saudeHandler.MarcarPronto()
		logger.Info("servidor iniciado", zap.String("porta", cfg.Porta))
		if err := servidor.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("falha ao iniciar servidor", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("desligando servidor...")

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()

	if err := servidor.Shutdown(ctxShutdown); err != nil {
		logger.Fatal("erro no shutdown do servidor", zap.Error(err))
	}

	fmt.Println("servidor encerrado")
}
