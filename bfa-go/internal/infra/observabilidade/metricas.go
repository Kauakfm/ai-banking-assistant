package observabilidade

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metricas struct {
	requisicoes    *prometheus.CounterVec
	latencia       *prometheus.HistogramVec
	errosExternos  *prometheus.CounterVec
	cacheHits      prometheus.Counter
	cacheMisses    prometheus.Counter
	circuitoAberto *prometheus.CounterVec
	tokensUsados   *prometheus.CounterVec
	custoEstimado  prometheus.Counter
}

func NovasMetricas() *Metricas {
	return &Metricas{
		requisicoes: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "bfa_requisicoes_total",
			Help: "Total de requisições HTTP recebidas",
		}, []string{"rota", "metodo", "status"}),

		latencia: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "bfa_latencia_segundos",
			Help:    "Latência das requisições HTTP em segundos",
			Buckets: prometheus.DefBuckets,
		}, []string{"rota", "metodo"}),

		errosExternos: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "bfa_erros_externos_total",
			Help: "Total de erros em chamadas a serviços externos",
		}, []string{"servico", "tipo"}),

		cacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "bfa_cache_hits_total",
			Help: "Total de cache hits",
		}),

		cacheMisses: promauto.NewCounter(prometheus.CounterOpts{
			Name: "bfa_cache_misses_total",
			Help: "Total de cache misses",
		}),

		circuitoAberto: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "bfa_circuito_aberto_total",
			Help: "Total de vezes que o circuit breaker abriu",
		}, []string{"servico"}),

		tokensUsados: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "bfa_tokens_usados_total",
			Help: "Total de tokens consumidos pelo LLM",
		}, []string{"direcao"}),

		custoEstimado: promauto.NewCounter(prometheus.CounterOpts{
			Name: "bfa_custo_estimado_total",
			Help: "Custo estimado total em USD",
		}),
	}
}

func (m *Metricas) RegistrarRequisicao(rota, metodo string, status int, duracao time.Duration) {
	m.requisicoes.WithLabelValues(rota, metodo, fmt.Sprintf("%d", status)).Inc()
	m.latencia.WithLabelValues(rota, metodo).Observe(duracao.Seconds())
}

func (m *Metricas) RegistrarErroExterno(servico, tipo string) {
	m.errosExternos.WithLabelValues(servico, tipo).Inc()
}

func (m *Metricas) RegistrarCacheHit() {
	m.cacheHits.Inc()
}

func (m *Metricas) RegistrarCacheMiss() {
	m.cacheMisses.Inc()
}

func (m *Metricas) RegistrarCircuitoAberto(servico string) {
	m.circuitoAberto.WithLabelValues(servico).Inc()
}

func (m *Metricas) RegistrarTokens(entrada, saida int) {
	m.tokensUsados.WithLabelValues("entrada").Add(float64(entrada))
	m.tokensUsados.WithLabelValues("saida").Add(float64(saida))
}

func (m *Metricas) RegistrarCusto(custo float64) {
	m.custoEstimado.Add(custo)
}
