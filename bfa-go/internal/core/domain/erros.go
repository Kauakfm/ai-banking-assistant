package domain

import "fmt"

type TipoErro int

const (
	ErroNaoEncontrado TipoErro = iota
	ErroValidacao
	ErroTimeout
	ErroServicoIndisponivel
	ErroInterno
	ErroCircuitoAberto
	ErroBulkheadCheio
)

type ErroDominio struct {
	Tipo     TipoErro
	Mensagem string
	Causa    error
}

func (e *ErroDominio) Error() string {
	if e.Causa != nil {
		return fmt.Sprintf("%s: %v", e.Mensagem, e.Causa)
	}
	return e.Mensagem
}

func (e *ErroDominio) Unwrap() error {
	return e.Causa
}

func NovoErroNaoEncontrado(msg string) *ErroDominio {
	return &ErroDominio{Tipo: ErroNaoEncontrado, Mensagem: msg}
}

func NovoErroValidacao(msg string) *ErroDominio {
	return &ErroDominio{Tipo: ErroValidacao, Mensagem: msg}
}

func NovoErroTimeout(msg string, causa error) *ErroDominio {
	return &ErroDominio{Tipo: ErroTimeout, Mensagem: msg, Causa: causa}
}

func NovoErroServicoIndisponivel(msg string, causa error) *ErroDominio {
	return &ErroDominio{Tipo: ErroServicoIndisponivel, Mensagem: msg, Causa: causa}
}

func NovoErroInterno(msg string, causa error) *ErroDominio {
	return &ErroDominio{Tipo: ErroInterno, Mensagem: msg, Causa: causa}
}
