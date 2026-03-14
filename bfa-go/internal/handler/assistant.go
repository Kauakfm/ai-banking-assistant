package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/kauakfm/ai-banking-assistant/internal/client"
	"github.com/kauakfm/ai-banking-assistant/internal/domain"
	"github.com/kauakfm/ai-banking-assistant/pkg/cache"
	pkgerr "github.com/kauakfm/ai-banking-assistant/pkg/errors"
	"github.com/kauakfm/ai-banking-assistant/pkg/middleware"
)

type AssistantHandler struct {
	agent   *client.AgentClient
	cache   *cache.Cache
	metrics *middleware.Metrics
	log     *slog.Logger
}

func NewAssistantHandler(agent *client.AgentClient, c *cache.Cache, m *middleware.Metrics, log *slog.Logger) *AssistantHandler {
	if log == nil {
		log = slog.Default()
	}
	return &AssistantHandler{agent: agent, cache: c, metrics: m, log: log}
}

func (h *AssistantHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	customerID := r.PathValue("customerId")
	if customerID == "" {
		pkgerr.WriteError(w, http.StatusBadRequest, "ID do cliente é obrigatório")
		return
	}

	var req domain.AssistantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		pkgerr.WriteError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	defer r.Body.Close()

	if strings.TrimSpace(req.Prompt) == "" {
		pkgerr.WriteError(w, http.StatusBadRequest, "prompt é obrigatório")
		return
	}

	cacheKey := fmt.Sprintf("assistant:%s:%s", customerID, req.Prompt)
	if cached, ok := h.cache.Get(cacheKey); ok {
		h.metrics.CacheHits.Inc()
		resp := cached.(*domain.AssistantResponse)
		resp.Cached = true
		resp.DurationMs = time.Since(start).Milliseconds()
		pkgerr.WriteJSON(w, http.StatusOK, resp)
		return
	}
	h.metrics.CacheMisses.Inc()

	agentResp, err := h.agent.Generate(r.Context(), customerID, req.Prompt)
	if err != nil {
		pkgerr.HandleError(w, err)
		return
	}

	resp := &domain.AssistantResponse{
		CustomerID: customerID,
		Prompt:     req.Prompt,
		Response:   agentResp.Response,
		Cached:     false,
		DurationMs: time.Since(start).Milliseconds(),
		Metadata:   agentResp.Metadata,
	}

	h.cache.Set(cacheKey, resp)

	h.log.InfoContext(r.Context(), "requisição do assistente processada",
		"id_cliente", customerID,
		"duracao_ms", resp.DurationMs,
	)

	pkgerr.WriteJSON(w, http.StatusOK, resp)
}
