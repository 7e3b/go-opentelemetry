package otel

import (
	"context"

	"go.opentelemetry.io/otel/log"
	sdkLog "go.opentelemetry.io/otel/sdk/log"
)

type logger struct {
	provider *sdkLog.LoggerProvider
	logger   log.Logger
	severity log.Severity
}

func (logger *logger) shutdown(ctx context.Context) error {
	err := logger.provider.ForceFlush(ctx)
	if err != nil {
		return err
	}
	err = logger.provider.Shutdown(ctx)
	if err != nil {
		return err
	}
	return nil
}
