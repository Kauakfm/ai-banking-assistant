package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/domain"
	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/core/ports"
)

type AssistenteHandler struct {
	servico ports.ServicoAssistente
}

func NovoAssistenteHandler(servico ports.ServicoAssistente) *AssistenteHandler {
	return &AssistenteHandler{servico: servico}
}

func (h *AssistenteHandler) Consultar(w http.ResponseWriter, r *http.Request) {
	clienteID := chi.URLParam(r, "customerId")
	if clienteID == "" {
		escreverErro(w, http.StatusBadRequest, "customerId é obrigatório")
		return
	}

	resposta, err := h.servico.ConsultarAssistente(r.Context(), clienteID)
	if err != nil {
		tratarErroDominio(w, err)
		return
	}

	escreverJSON(w, http.StatusOK, resposta)
}

func tratarErroDominio(w http.ResponseWriter, err error) {
	var errDominio *domain.ErroDominio
	if errors.As(err, &errDominio) {
		switch errDominio.Tipo {
		case domain.ErroNaoEncontrado:
			escreverErro(w, http.StatusNotFound, errDominio.Mensagem)
		case domain.ErroValidacao:
			escreverErro(w, http.StatusBadRequest, errDominio.Mensagem)
		case domain.ErroTimeout:
			escreverErro(w, http.StatusGatewayTimeout, errDominio.Mensagem)
		case domain.ErroServicoIndisponivel:
			escreverErro(w, http.StatusServiceUnavailable, errDominio.Mensagem)
		case domain.ErroCircuitoAberto:
			escreverErro(w, http.StatusServiceUnavailable, errDominio.Mensagem)
		case domain.ErroBulkheadCheio:
			escreverErro(w, http.StatusTooManyRequests, errDominio.Mensagem)
		default:
			escreverErro(w, http.StatusInternalServerError, errDominio.Mensagem)
		}
		return
	}
	escreverErro(w, http.StatusInternalServerError, "erro interno")
}

type respostaErro struct {
	Erro string `json:"erro"`
}

func escreverJSON(w http.ResponseWriter, status int, dados interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(dados)
}

func escreverErro(w http.ResponseWriter, status int, mensagem string) {
	escreverJSON(w, status, respostaErro{Erro: mensagem})
}
