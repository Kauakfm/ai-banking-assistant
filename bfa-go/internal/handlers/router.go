package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/Kauakfm/ai-banking-assistant/bfa-go/internal/infra/observabilidade"
)

func NovoRouter(
	assistenteHandler *AssistenteHandler,
	saudeHandler *SaudeHandler,
	logger *zap.Logger,
	metricas *observabilidade.Metricas,
) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middlewareRecovery(logger))
	r.Use(middlewareLog(logger))
	r.Use(middlewareMetricas(metricas))
	r.Use(middlewareTracing())

	r.Get("/healthz", saudeHandler.Healthz)
	r.Get("/readyz", saudeHandler.Readyz)
	r.Handle("/metrics", promhttp.Handler())

	r.Route("/v1", func(r chi.Router) {
		r.Get("/assistant/{customerId}", assistenteHandler.Consultar)
	})

	return r
}

func middlewareRecovery(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recuperado", zap.Any("panic", rec), zap.String("path", r.URL.Path))
					escreverErro(w, http.StatusInternalServerError, "erro interno do servidor")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func middlewareLog(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inicio := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			logger.Info("requisicao",
				zap.String("metodo", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Duration("duracao", time.Since(inicio)),
				zap.String("request_id", middleware.GetReqID(r.Context())),
			)
		})
	}
}

func middlewareMetricas(metricas *observabilidade.Metricas) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inicio := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			rctx := chi.RouteContext(r.Context())
			padrao := rctx.RoutePattern()
			if padrao == "" {
				padrao = r.URL.Path
			}

			metricas.RegistrarRequisicao(padrao, r.Method, ww.Status(), time.Since(inicio))
		})
	}
}

func middlewareTracing() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tracer := otel.Tracer("bfa-go")
			ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path)
			defer span.End()

			span.SetAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
