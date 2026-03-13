package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()

	r.Get("/api/v1/profile/{customerId}", handlePerfil)
	r.Get("/api/v1/transactions/{customerId}", handleTransacoes)
	r.Post("/api/v1/agent/invoke", handleAgente)

	fmt.Println("mock server rodando na porta 8081")
	http.ListenAndServe(":8081", r)
}

func handlePerfil(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "customerId")
	perfil := map[string]interface{}{
		"cliente_id": id,
		"nome":       "Empresa " + id,
		"segmento":   "middle_market",
		"cnpj":       "12.345.678/0001-99",
		"email":      "contato@empresa.com.br",
		"telefone":   "(11) 99999-0000",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(perfil)
}

func handleTransacoes(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "customerId")
	categorias := []string{"folha_pagamento", "fornecedores", "impostos", "receita", "investimento"}
	tipos := []string{"debito", "credito"}

	transacoes := make([]map[string]interface{}, 10)
	for i := 0; i < 10; i++ {
		transacoes[i] = map[string]interface{}{
			"id":         fmt.Sprintf("txn-%s-%d", id, i+1),
			"cliente_id": id,
			"tipo":       tipos[rand.Intn(len(tipos))],
			"valor":      float64(rand.Intn(50000)) + 100.50,
			"descricao":  fmt.Sprintf("Transação %d", i+1),
			"data":       time.Now().AddDate(0, 0, -rand.Intn(30)).Format(time.RFC3339),
			"categoria":  categorias[rand.Intn(len(categorias))],
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transacoes)
}

func handleAgente(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body)
	defer r.Body.Close()

	resposta := map[string]interface{}{
		"recomendacao":  "Com base na análise do fluxo de caixa e histórico de transações, recomendamos a contratação de uma linha de crédito para capital de giro com taxa preferencial para o segmento middle market. O padrão de receitas indica sazonalidade que pode ser suavizada com antecipação de recebíveis.",
		"justificativa": "A análise identificou: (1) Concentração de despesas nos dias 5-10 do mês (folha + impostos), (2) Receitas distribuídas ao longo do mês, (3) Gap de fluxo de caixa de aproximadamente R$ 45.000 nos primeiros 10 dias. A linha de crédito rotativo com carência é a opção mais adequada para o perfil.",
		"fontes":        []string{"politica_credito_pj_v3", "tabela_taxas_middle_market_2025", "faq_antecipacao_recebiveis"},
		"confianca":     0.87,
		"metadata": map[string]interface{}{
			"total_tokens":       1250,
			"latencia_ms":        340.5,
			"passos_executados":  4,
			"ferramentas_usadas": []string{"buscar_perfil", "buscar_transacoes", "consultar_base_conhecimento", "analisar_fluxo_caixa"},
			"custo_estimado":     0.0038,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resposta)
}
