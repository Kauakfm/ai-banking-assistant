package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/kauakfm/ai-banking-assistant/internal/client"
	"github.com/kauakfm/ai-banking-assistant/pkg/cache"
	pkgerr "github.com/kauakfm/ai-banking-assistant/pkg/errors"
	"github.com/kauakfm/ai-banking-assistant/pkg/middleware"
)

// TransactionHandler é o handler BFA para o domínio de transações do cliente.
// Encapsula cache, métricas, logging e resiliência.
type TransactionHandler struct {
	client  *client.TransactionClient
	cache   *cache.Cache
	metrics *middleware.Metrics
	log     *slog.Logger
}

func NewTransactionHandler(c *client.TransactionClient, cache *cache.Cache, m *middleware.Metrics, log *slog.Logger) *TransactionHandler {
	if log == nil {
		log = slog.Default()
	}
	return &TransactionHandler{client: c, cache: cache, metrics: m, log: log}
}

func (h *TransactionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	customerID := r.PathValue("customerId")
	if customerID == "" {
		pkgerr.WriteError(w, http.StatusBadRequest, "ID do cliente é obrigatório")
		return
	}

	// Cache: verificar se já temos as transações em cache
	cacheKey := fmt.Sprintf("transactions:%s", customerID)
	if cached, ok := h.cache.Get(cacheKey); ok {
		h.metrics.CacheHits.Inc()
		h.log.InfoContext(r.Context(), "transações servidas do cache BFA",
			"id_cliente", customerID,
			"duracao_ms", time.Since(start).Milliseconds(),
		)
		pkgerr.WriteJSON(w, http.StatusOK, cached)
		return
	}
	h.metrics.CacheMisses.Inc()

	// Chamada à API de domínio via client com resiliência
	transactions, err := h.client.GetByCustomerID(r.Context(), customerID)
	if err != nil {
		h.log.ErrorContext(r.Context(), "falha ao consultar transações via BFA",
			"id_cliente", customerID,
			"erro", err,
		)
		pkgerr.HandleError(w, err)
		return
	}

	// Armazenar no cache do BFA
	h.cache.Set(cacheKey, transactions)

	h.log.InfoContext(r.Context(), "transações consultadas via BFA",
		"id_cliente", customerID,
		"total_transacoes", len(transactions),
		"duracao_ms", time.Since(start).Milliseconds(),
	)

	pkgerr.WriteJSON(w, http.StatusOK, transactions)
}
