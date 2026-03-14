package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HttpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bfa_http_requests_total",
		Help: "Total de requisições HTTP recebidas",
	}, []string{"method", "endpoint", "status"})

	HttpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "bfa_http_request_duration_seconds",
		Help:    "Latência das requisições HTTP em segundos",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "endpoint"})
)
