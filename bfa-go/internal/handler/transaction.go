package handler

import (
	"log/slog"
	"net/http"

	"github.com/kauakfm/ai-banking-assistant/internal/client"
	pkgerr "github.com/kauakfm/ai-banking-assistant/pkg/errors"
)

type TransactionHandler struct {
	client *client.TransactionClient
	log    *slog.Logger
}

func NewTransactionHandler(c *client.TransactionClient, log *slog.Logger) *TransactionHandler {
	if log == nil {
		log = slog.Default()
	}
	return &TransactionHandler{client: c, log: log}
}

func (h *TransactionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("customerId")
	if customerID == "" {
		pkgerr.WriteError(w, http.StatusBadRequest, "ID do cliente é obrigatório")
		return
	}

	transactions, err := h.client.GetByCustomerID(r.Context(), customerID)
	if err != nil {
		pkgerr.HandleError(w, err)
		return
	}

	pkgerr.WriteJSON(w, http.StatusOK, transactions)
}
