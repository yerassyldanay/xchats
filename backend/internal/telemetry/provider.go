// Package telemetry wires OpenTelemetry tracing to Langfuse over OTLP/HTTP, so
// every LLM call surfaces as a Langfuse "generation" observation. The actual
// span emission lives in internal/brain/llm; this package only builds and
// installs the global TracerProvider.
package telemetry

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/yerassyldanay/xchats/backend/internal/config"
)

// defaultHost is used when LANGFUSE_HOST is empty.
const defaultHost = "https://cloud.langfuse.com"

// NewLangfuseProvider builds an OTel TracerProvider that exports spans to
// Langfuse via OTLP/HTTP. It uses a BatchSpanProcessor so export is fully
// asynchronous and never blocks the calling goroutine.
//
// The caller must call Shutdown on the returned provider at exit so buffered
// spans are flushed.
func NewLangfuseProvider(ctx context.Context, cfg *config.Config, serviceName string) (*sdktrace.TracerProvider, error) {
	host := cfg.LangfuseHost
	if host == "" {
		host = defaultHost
	}
	parsedURL, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("invalid langfuse host %q: %w", host, err)
	}

	authToken := base64.StdEncoding.EncodeToString(
		[]byte(cfg.LangfusePublicKey + ":" + cfg.LangfuseSecretKey),
	)

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(parsedURL.Host),
		otlptracehttp.WithURLPath("/api/public/otel/v1/traces"),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization":                "Basic " + authToken,
			"x-langfuse-ingestion-version": "4",
		}),
	}
	if parsedURL.Scheme == "http" {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create langfuse otlp exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	batchOpts := []sdktrace.BatchSpanProcessorOption{}
	if cfg.LangfuseFlushIntervalMS > 0 {
		batchOpts = append(batchOpts, sdktrace.WithBatchTimeout(
			time.Duration(cfg.LangfuseFlushIntervalMS)*time.Millisecond))
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, batchOpts...), // async — background export
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}
