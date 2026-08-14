package opentelemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/propagation"
)

type Client interface {
	Shutdown(context.Context) error
}

type client struct {
	metadata bool
	tracer   *tracer
	logger   *logger
	meter    *meter
}

func (client *client) Shutdown(ctx context.Context) error {
	if client.tracer != nil {
		err := client.tracer.shutdown(ctx)
		if err != nil {
			return err
		}
	}
	if client.logger != nil {
		err := client.logger.shutdown(ctx)
		if err != nil {
			return err
		}
	}
	if client.meter != nil {
		err := client.meter.shutdown(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (client *client) Start(ctx context.Context, extras ...map[string]any) Span {
	span := &span{ctx: ctx, client: client}
	if client.tracer == nil && client.logger == nil {
		return span
	}
	span.start(ctx, extras...)
	return span
}

func (client *client) StartWithName(ctx context.Context, name string, extras ...map[string]any) Span {
	span := &span{ctx: ctx, client: client}
	if client.tracer == nil && client.logger == nil {
		return span
	}
	span.startWithName(ctx, name, extras...)
	return span
}

func (client *client) Propagator() propagation.TextMapPropagator {
	tracer := client.tracer
	if client.tracer == nil {
		return nil
	}
	return tracer.propagator
}

func (client *client) inject(ctx context.Context, obj map[string]string) {
	tracer := client.tracer
	if tracer == nil {
		return
	}
	propagator := tracer.propagator
	propagator.Inject(ctx, propagation.MapCarrier(obj))
}

func (client *client) extract(ctx context.Context, obj map[string]string) context.Context {
	tracer := client.tracer
	if tracer == nil {
		return context.Background()
	}
	propagator := tracer.propagator
	return propagator.Extract(ctx, propagation.MapCarrier(obj))
}

func (client *client) WithNewTimeout(ctx context.Context, expiry time.Duration) (context.Context, context.CancelFunc) {
	carrier := map[string]string{}
	client.inject(ctx, carrier)
	ctx = client.extract(context.Background(), carrier)
	return context.WithTimeout(ctx, expiry)
}

func (client *client) WithoutTimeout(ctx context.Context) context.Context {
	carrier := map[string]string{}
	client.inject(ctx, carrier)
	return client.extract(context.Background(), carrier)
}
