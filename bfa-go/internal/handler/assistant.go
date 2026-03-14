package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/kauakfm/ai-banking-assistant/internal/service"
	"github.com/kauakfm/ai-banking-assistant/pkg/cache"

	"go.opentelemetry.io/otel"
)

type AssistantHandler struct {
	orchestrator *service.Orchestrator
	cache        *cache.LocalCache
}

func NewAssistantHandler(o *service.Orchestrator, c *cache.LocalCache) *AssistantHandler {
	return &AssistantHandler{
		orchestrator: o,
		cache:        c,
	}
}

func respondWithError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func (h *AssistantHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("bfa-go").Start(r.Context(), "AssistantHandler.ServeHTTP")
	defer span.End()

	customerID := r.PathValue("customerId")
	if customerID == "" {
		respondWithError(w, http.StatusBadRequest, "customerId é obrigatório")
		return
	}

	query := r.URL.Query().Get("query")
	if query == "" {
		respondWithError(w, http.StatusBadRequest, "O parâmetro 'query' é obrigatório")
		return
	}

	slog.InfoContext(ctx, "Processando requisição", "customerId", customerID, "query", query)

	cacheKey := "assistant:" + customerID + ":" + query
	if cachedData, found := h.cache.Get(cacheKey); found {
		slog.InfoContext(ctx, "Retornando dados do cache", "cacheKey", cacheKey)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		json.NewEncoder(w).Encode(cachedData)
		return
	}

	result, err := h.orchestrator.ProcessAssistantQuery(ctx, customerID, query)
	if err != nil {
		slog.ErrorContext(ctx, "Falha ao processar requisição do agente", 
			"error", err.Error(),
			"customerId", customerID,
			"query", query)
		respondWithError(w, http.StatusServiceUnavailable, "Serviço temporariamente indisponível. Tente novamente mais tarde.")
		return
	}

	h.cache.Set(cacheKey, result, 2*time.Minute)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	json.NewEncoder(w).Encode(result)
}
