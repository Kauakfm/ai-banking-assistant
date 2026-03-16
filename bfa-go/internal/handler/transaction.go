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

	transactions, err := h.client.GetByCustomerID(r.Context(), customerID)
	if err != nil {
		h.log.ErrorContext(r.Context(), "falha ao consultar transações via BFA",
			"id_cliente", customerID,
			"erro", err,
		)
		pkgerr.HandleError(w, err)
		return
	}

	h.cache.Set(cacheKey, transactions)

	h.log.InfoContext(r.Context(), "transações consultadas via BFA",
		"id_cliente", customerID,
		"total_transacoes", len(transactions),
		"duracao_ms", time.Since(start).Milliseconds(),
	)

	pkgerr.WriteJSON(w, http.StatusOK, transactions)
}
