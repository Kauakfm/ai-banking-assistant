package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
)

type Metrics struct {
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RequestsInFlight prometheus.Gauge
	CacheHits        prometheus.Counter
	CacheMisses      prometheus.Counter
}

func NewMetrics() *Metrics {
	return &Metrics{
		RequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "bfa_http_requests_total",
			Help: "Total de requisições HTTP por método, caminho e status.",
		}, []string{"method", "path", "status"}),

		RequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "bfa_http_request_duration_seconds",
			Help:    "Duração das requisições HTTP em segundos.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),

		RequestsInFlight: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bfa_http_requests_in_flight",
			Help: "Número de requisições HTTP sendo processadas.",
		}),

		CacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "bfa_cache_hits_total",
			Help: "Total de acertos no cache.",
		}),

		CacheMisses: promauto.NewCounter(prometheus.CounterOpts{
			Name: "bfa_cache_misses_total",
			Help: "Total de falhas no cache.",
		}),
	}
}

func InstrumentHTTP(m *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

			m.RequestsInFlight.Inc()
			defer m.RequestsInFlight.Dec()

			next.ServeHTTP(rw, r)

			m.RequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(time.Since(start).Seconds())
			m.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, http.StatusText(rw.status)).Inc()
		})
	}
}

func isOperational(path string) bool {
	return path == "/healthz" || path == "/livez" || path == "/readyz" || path == "/metrics"
}

func Tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isOperational(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		ctx, span := otel.Tracer("bfa-go").Start(r.Context(), r.Method+" "+r.URL.Path)
		defer span.End()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isOperational(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("requisição http",
			"metodo", r.Method,
			"caminho", r.URL.Path,
			"status", rw.status,
			"duracao_ms", time.Since(start).Milliseconds(),
			"endereco_remoto", r.RemoteAddr,
		)
	})
}

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recuperado",
					"panic", rec,
					"stack", string(debug.Stack()),
					"caminho", r.URL.Path,
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"erro interno do servidor","code":500}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}
