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

type ProfileHandler struct {
	client  *client.ProfileClient
	cache   *cache.Cache
	metrics *middleware.Metrics
	log     *slog.Logger
}

func NewProfileHandler(c *client.ProfileClient, cache *cache.Cache, m *middleware.Metrics, log *slog.Logger) *ProfileHandler {
	if log == nil {
		log = slog.Default()
	}
	return &ProfileHandler{client: c, cache: cache, metrics: m, log: log}
}

func (h *ProfileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	customerID := r.PathValue("customerId")
	if customerID == "" {
		pkgerr.WriteError(w, http.StatusBadRequest, "ID do cliente é obrigatório")
		return
	}

	cacheKey := fmt.Sprintf("profile:%s", customerID)
	if cached, ok := h.cache.Get(cacheKey); ok {
		h.metrics.CacheHits.Inc()
		h.log.InfoContext(r.Context(), "perfil servido do cache BFA",
			"id_cliente", customerID,
			"duracao_ms", time.Since(start).Milliseconds(),
		)
		pkgerr.WriteJSON(w, http.StatusOK, cached)
		return
	}
	h.metrics.CacheMisses.Inc()

	profile, err := h.client.GetByID(r.Context(), customerID)
	if err != nil {
		h.log.ErrorContext(r.Context(), "falha ao consultar perfil via BFA",
			"id_cliente", customerID,
			"erro", err,
		)
		pkgerr.HandleError(w, err)
		return
	}

	h.cache.Set(cacheKey, profile)

	h.log.InfoContext(r.Context(), "perfil consultado via BFA",
		"id_cliente", customerID,
		"duracao_ms", time.Since(start).Milliseconds(),
	)

	pkgerr.WriteJSON(w, http.StatusOK, profile)
}
