package tracing

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func InitTracer(serviceName, collector string) (*tracesdk.TracerProvider, error) {
	endpoint := normalizeJaegerCollector(collector)

	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(
		jaeger.WithEndpoint(endpoint),
	))
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return tp, nil
}

func normalizeJaegerCollector(value string) string {
	const defaultEndpoint = "http://localhost:14268/api/traces"
	if strings.TrimSpace(value) == "" {
		return defaultEndpoint
	}

	endpoint := strings.TrimSpace(value)
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	if strings.HasSuffix(endpoint, "/api/traces") {
		return endpoint
	}

	return fmt.Sprintf("%s/api/traces", strings.TrimSuffix(endpoint, "/"))
}
