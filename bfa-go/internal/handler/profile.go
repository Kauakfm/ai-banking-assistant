package handler

import (
	"log/slog"
	"net/http"

	"github.com/kauakfm/ai-banking-assistant/internal/client"
	pkgerr "github.com/kauakfm/ai-banking-assistant/pkg/errors"
)

type ProfileHandler struct {
	client *client.ProfileClient
	log    *slog.Logger
}

func NewProfileHandler(c *client.ProfileClient, log *slog.Logger) *ProfileHandler {
	if log == nil {
		log = slog.Default()
	}
	return &ProfileHandler{client: c, log: log}
}

func (h *ProfileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")
	if customerID == "" {
		pkgerr.WriteError(w, http.StatusBadRequest, "ID do cliente é obrigatório")
		return
	}

	profile, err := h.client.GetByID(r.Context(), customerID)
	if err != nil {
		pkgerr.HandleError(w, err)
		return
	}

	pkgerr.WriteJSON(w, http.StatusOK, profile)
}
