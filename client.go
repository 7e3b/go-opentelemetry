package otx

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
)

// Client provides application-level access to OpenTelemetry tracing,
// structured logging, metrics, and trace-context propagation.
//
// A Client should normally be created once during application startup and
// reused throughout the lifetime of the application.
type Client interface {
	// Start starts a new span using the supplied context.
	//
	// Span configuration can be supplied through SpanConfig. Attributes
	// configured on the span are also attached to log records emitted
	// through the returned Span.
	//
	// When the span name is not explicitly specified, the client derives
	// the name from the calling operation.
	//
	// The returned Span should normally be ended using defer:
	//
	//	span := client.Start(ctx)
	//	defer span.End()
	// At most one SpanConfig should be supplied. If multiple configurations
	// are supplied, only the first configuration is used.
	Start(context.Context, ...SpanConfig) Span

	// Shutdown flushes pending telemetry and shuts down all enabled
	// OpenTelemetry providers.
	//
	// Shutdown should be called during graceful application shutdown.
	Shutdown(context.Context) error

	// Propagator returns the OpenTelemetry text-map propagator used by
	// the client.
	//
	// Use the propagator to inject trace context into outbound requests
	// and extract trace context from inbound requests when integrating
	// with transports that are not automatically instrumented.
	//
	// Propagator returns nil when tracing is disabled.
	Propagator() propagation.TextMapPropagator

	// WithNewTimeout creates a new context with the specified timeout while
	// preserving the OpenTelemetry trace context from the supplied context.
	//
	// This is useful when a new timeout or cancellation boundary is required
	// without losing the current distributed trace context.
	//
	// The returned CancelFunc must be called when the context is no longer
	// needed.
	WithNewTimeout(context.Context, time.Duration) (context.Context, context.CancelFunc)

	// WithoutTimeout creates a new context without inheriting the deadline
	// or cancellation of the supplied context while preserving its
	// OpenTelemetry trace context.
	//
	// This can be used when work must continue independently of the lifetime
	// of the original request context while remaining part of the same trace.
	WithoutTimeout(context.Context) context.Context

	// TraceProvider returns the underlying OpenTelemetry SDK tracer provider.
	//
	// Use this only when direct access to the OpenTelemetry SDK is required,
	// such as integrating with a library that accepts a *trace.TracerProvider.
	//
	// TraceProvider returns nil when tracing is disabled.
	TraceProvider() *trace.TracerProvider
}

var mu = &sync.RWMutex{}

var global Client

// Start starts a new span using the globally configured OpenTelemetry client.
//
// Config.Connect must be called successfully before using Start.
//
// The returned Span should normally be ended using defer:
//
//	span := otx.Start(ctx)
//	defer span.End()
//
// Span configuration can be supplied through SpanConfig.
// At most one SpanConfig should be supplied. If multiple configurations
// are supplied, only the first configuration is used.
func Start(ctx context.Context, config ...SpanConfig) Span {
	mu.RLock()
	client := global
	mu.RUnlock()
	return client.Start(ctx, config...)
}

// Shutdown flushes pending telemetry and shuts down all enabled
// OpenTelemetry providers on the globally configured client.
//
// Config.Connect must be called successfully before using Shutdown.
//
// Shutdown should be called during graceful application shutdown.
func Shutdown(ctx context.Context) error {
	mu.RLock()
	client := global
	mu.RUnlock()
	return client.Shutdown(ctx)
}

// WithNewTimeout creates a new context with the specified timeout while
// preserving the OpenTelemetry trace context from the supplied context.
//
// Config.Connect must be called successfully before using
// WithNewTimeout.
//
// The returned context does not inherit the deadline or cancellation of
// the supplied context, but retains its OpenTelemetry trace context.
//
// The returned CancelFunc must be called when the context is no longer
// needed.
func WithNewTimeout(ctx context.Context, expiry time.Duration) (context.Context, context.CancelFunc) {
	mu.RLock()
	client := global
	mu.RUnlock()
	return client.WithNewTimeout(ctx, expiry)
}

// WithoutTimeout creates a new context without inheriting the deadline or
// cancellation of the supplied context while preserving its OpenTelemetry
// trace context.
//
// Config.Connect must be called successfully before using WithoutTimeout.
//
// This is useful when work must continue independently of the lifetime of
// the original request context while remaining part of the same trace.
func WithoutTimeout(ctx context.Context) context.Context {
	mu.RLock()
	client := global
	mu.RUnlock()
	return client.WithoutTimeout(ctx)
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

func (client *client) Start(ctx context.Context, config ...SpanConfig) Span {
	span := &span{ctx: ctx, client: client}
	if client.tracer == nil && client.logger == nil {
		return span
	}
	span.start(ctx, config...)
	return span
}

func (client *client) TraceProvider() *trace.TracerProvider {
	tracer := client.tracer
	if client.tracer == nil {
		return nil
	}
	return tracer.provider
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
