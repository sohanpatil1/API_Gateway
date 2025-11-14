package main

/*
OTLP : Open Telemetry Protocol. Format used to send traces/metrics/logs to Collector
Transport protocol + data format
Tracer: Object to create spans
Span: Single timed operation that shows duration, errors, metadata
Jaeger receives the trace from the collector
*/

import (
    "context"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/semconv/v1.21.0"
	"time"
)

func initTracer(service_name string) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	// 1. Exporter sends the span to the Collector at port 4318
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint("localhost:4318"), // Connects to Collector not Jaeger
		otlptracehttp.WithInsecure(), // Use http and not https
	)
	if err != nil {
        return nil, err
    }

	// 2. Resouce: describes your service for viewing Jaeger
	res := resource.NewWithAttributes(
		semconv.SchemaURL,	// To identify which version of OpenTelemetry we are using
		semconv.ServiceNameKey.String(service_name),	// This is a name we set for that service so easy to identify on Jaeger.
	)

	// 3. TracerProvider: controls sampling + batching + exporting
	// Send all, 10% vs slow requests for dashboard.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	    sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// tp is made as a global tracer
	otel.SetTracerProvider(tp)

	return tp, nil
}

func shutdownTracer(tp *sdktrace.TracerProvider, ctx context.Context) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    _ = tp.Shutdown(ctx)
}