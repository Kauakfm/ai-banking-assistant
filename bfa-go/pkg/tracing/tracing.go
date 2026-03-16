package tracing

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

func Init(serviceName string, exporterURL string) (func(context.Context) error, error) {
	var exp sdktrace.SpanExporter
	var err error

	if exporterURL != "" {
		exp, err = otlptracegrpc.New(
			context.Background(),
			otlptracegrpc.WithEndpoint(exporterURL),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}
		slog.Info("tracing: exporter OTLP gRPC configurado", "endpoint", exporterURL)
	} else {
		exp, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
		slog.Info("tracing: usando exporter stdout (OTEL_EXPORTER_OTLP_ENDPOINT não definido)")
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)

	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

func Tracer() trace.Tracer {
	return otel.Tracer("bfa-go")
}
