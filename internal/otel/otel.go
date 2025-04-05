package otel

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	// "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"

	// "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
)

// Config holds configuration for OpenTelemetry
type Config struct {
	// OTLPEndpoint is the endpoint for OpenTelemetry collector (e.g., "localhost:4318")
	OTLPEndpoint string
	// ServiceName is the name of this service
	ServiceName string
	// ServiceVersion is the version of this service
	ServiceVersion string
	// Environment is the deployment environment (e.g., "production", "development")
	Environment string
}

// InitTracer initializes OpenTelemetry with the given configuration
func InitTracer(cfg Config) (shutdown func(context.Context) error, err error) {
	// Create resource for this service
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create trace exporter using HTTP protocol
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		otlptracehttp.WithURLPath("/v1/traces"),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create trace provider
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)

	// Set propagators
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	/* Metrics code commented out to avoid 404 errors
	// Setup metrics exporter
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(cfg.OTLPEndpoint),
		otlpmetrichttp.WithURLPath("/v1/metrics"),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// Create metric provider
	metricProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter, metric.WithInterval(30*time.Second))),
	)
	otel.SetMeterProvider(metricProvider)
	*/

	// Return a shutdown function that will flush and shutdown the providers
	return func(ctx context.Context) error {
		// Shutdown trace provider
		if err := tracerProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}

		/* Metrics shutdown commented out
		// Shutdown metric provider
		if err := metricProvider.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down meter provider: %v", err)
		}
		*/

		return nil
	}, nil
}
