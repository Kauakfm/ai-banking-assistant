package errors

import (
	"encoding/json"
	"errors"
	"net/http"
)

var (
	ErrNotFound     = errors.New("recurso não encontrado")
	ErrUnavailable  = errors.New("serviço temporariamente indisponível")
	ErrTimeout      = errors.New("tempo limite da requisição excedido")
	ErrCircuitOpen  = errors.New("circuit breaker aberto")
	ErrBulkheadFull = errors.New("muitas requisições simultâneas")
	ErrBadRequest   = errors.New("requisição inválida")
)

type APIError struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, APIError{Error: msg, Code: status})
}

func HandleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrBadRequest):
		WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrBulkheadFull):
		WriteError(w, http.StatusTooManyRequests, err.Error())
	case errors.Is(err, ErrCircuitOpen):
		WriteError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, ErrTimeout):
		WriteError(w, http.StatusGatewayTimeout, err.Error())
	case errors.Is(err, ErrUnavailable):
		WriteError(w, http.StatusBadGateway, err.Error())
	default:
		WriteError(w, http.StatusInternalServerError, "erro interno do servidor")
	}
}
