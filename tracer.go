package otel

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
	sdkTrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type tracer struct {
	provider   *sdkTrace.TracerProvider
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

func (tracer *tracer) shutdown(ctx context.Context) error {
	err := tracer.provider.ForceFlush(ctx)
	if err != nil {
		return err
	}
	err = tracer.provider.Shutdown(ctx)
	if err != nil {
		return err
	}
	return nil
}
