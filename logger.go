package otx

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/log"
	sdkLog "go.opentelemetry.io/otel/sdk/log"
)

type logger struct {
	provider *sdkLog.LoggerProvider
	logger   log.Logger
	severity log.Severity
	console  *slog.Logger
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
