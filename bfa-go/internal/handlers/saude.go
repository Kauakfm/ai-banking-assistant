package handlers

import "net/http"

type SaudeHandler struct {
	pronto bool
}

func NovoSaudeHandler() *SaudeHandler {
	return &SaudeHandler{pronto: false}
}

func (h *SaudeHandler) MarcarPronto() {
	h.pronto = true
}

func (h *SaudeHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	escreverJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (h *SaudeHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	if !h.pronto {
		escreverJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	escreverJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
