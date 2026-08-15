package otel

import (
	"context"

	"go.opentelemetry.io/otel/metric"
	sdkMetric "go.opentelemetry.io/otel/sdk/metric"
)

type meter struct {
	provider *sdkMetric.MeterProvider
	meter    metric.Meter
}

func (meter *meter) shutdown(ctx context.Context) error {
	err := meter.provider.ForceFlush(ctx)
	if err != nil {
		return err
	}
	err = meter.provider.Shutdown(ctx)
	if err != nil {
		return err
	}
	return nil
}
