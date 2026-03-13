package domain

import "time"

type Perfil struct {
	ClienteID string `json:"cliente_id"`
	Nome      string `json:"nome"`
	Segmento  string `json:"segmento"`
	CNPJ      string `json:"cnpj"`
	Email     string `json:"email"`
	Telefone  string `json:"telefone"`
}

type Transacao struct {
	ID        string    `json:"id"`
	ClienteID string    `json:"cliente_id"`
	Tipo      string    `json:"tipo"`
	Valor     float64   `json:"valor"`
	Descricao string    `json:"descricao"`
	Data      time.Time `json:"data"`
	Categoria string    `json:"categoria"`
}

type RespostaAgente struct {
	Recomendacao  string           `json:"recomendacao"`
	Justificativa string           `json:"justificativa"`
	Fontes        []string         `json:"fontes,omitempty"`
	Confianca     float64          `json:"confianca"`
	Metadata      MetadataResposta `json:"metadata"`
}

type MetadataResposta struct {
	TotalTokens       int      `json:"total_tokens"`
	LatenciaMs        float64  `json:"latencia_ms"`
	PassosExecutados  int      `json:"passos_executados"`
	FerramentasUsadas []string `json:"ferramentas_usadas"`
	CustoEstimado     float64  `json:"custo_estimado"`
}

type RespostaAssistente struct {
	ClienteID  string         `json:"cliente_id"`
	Perfil     Perfil         `json:"perfil"`
	Transacoes []Transacao    `json:"transacoes"`
	Agente     RespostaAgente `json:"agente"`
	CacheHit   bool           `json:"cache_hit"`
}
