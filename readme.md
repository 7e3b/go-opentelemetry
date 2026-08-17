# go-otx

A lightweight Go wrapper around [OpenTelemetry](https://opentelemetry.io/) for **tracing, structured logging, metrics, and trace-context propagation**.

It provides a simple application-level API while allowing access to the underlying OpenTelemetry SDK when needed.

## Features

* Distributed tracing
* Configurable trace sampling
* Structured logging through OpenTelemetry Logs
* JSON console logging with `log/slog`
* Trace-correlated logs
* Trace, Debug, Info, Warn, Error, and Fatal log levels
* OpenTelemetry metrics
* OTLP/HTTP export
* Gzip compression
* Configurable TLS/insecure connections
* W3C Trace Context and Baggage propagation
* Manual context injection and extraction
* Span links for asynchronous work
* Context timeout/cancellation helpers that preserve trace context
* Optional source-code metadata
* Access to the underlying `TracerProvider`
* Global or explicitly managed clients
* Graceful shutdown with telemetry flushing

## Installation

```bash
go get github.com/7e3b/go-otx
```

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/7e3b/go-otx"
)

func main() {
	ctx := context.Background()

	err := (otx.Config{
		Name:        "my-service",
		Namespace:   "my-application",
		Environment: "production",
		Version:     "1.0.0",
		InstanceID:  "instance-1",

		Endpoint: "localhost:4318",
		Insecure: true,

		Tracer: otx.TracerConfig{
			SamplingRatio: 1.0,
		},

		Logger: otx.LoggerConfig{
			Severity: "info",
			Console:  true,
		},

		Meter: otx.MeterConfig{
			Enabled: true,
		},
	}).Connect(ctx)

	if err != nil {
		log.Fatal(err)
	}

	defer otx.Shutdown(ctx)

	span := otx.Start(ctx)
	defer span.End()

	span.Info("application started")
}
```

## Configuration

`Config` controls service identity, OTLP export, tracing, logging, metrics, and source-code metadata.

```go
type Config struct {
	Name            string
	Namespace       string
	Environment     string
	Version         string
	InstanceID      string
	Endpoint        string
	Insecure        bool
	WithoutMetadata bool

	Tracer TracerConfig
	Meter  MeterConfig
	Logger LoggerConfig
}
```

### Service Identity

```go
otx.Config{
	Name:        "user-service",
	Namespace:   "my-application",
	Environment: "production",
	Version:     "2.1.0",
	InstanceID:  "user-service-01",
}
```

These values become OpenTelemetry resource attributes:

| Config        | Attribute                     |
| ------------- | ----------------------------- |
| `Name`        | `service.name`                |
| `Namespace`   | `service.namespace`           |
| `Environment` | `deployment.environment.name` |
| `Version`     | `service.version`             |
| `InstanceID`  | `service.instance.id`         |
| Hostname      | `host.name`                   |

The hostname is obtained automatically from the running system.

### OTLP Endpoint

The configured endpoint is used by the enabled tracing, logging, and metrics exporters.

```go
Endpoint: "localhost:4318",
Insecure: true,
```

For TLS:

```go
Endpoint: "otel-collector.example.com:4318",
Insecure: false,
```

`Insecure: true` disables TLS and is useful for local or private environments where TLS is not required.

## Tracing

Tracing is enabled when `SamplingRatio` is greater than zero.

```go
Tracer: otx.TracerConfig{
	SamplingRatio: 1.0,
},
```

Examples:

```text
1.0  -> approximately 100%
0.5  -> approximately 50%
0.1  -> approximately 10%
0.01 -> approximately 1%
```

The sampler is parent-based:

```text
ParentBased(
    TraceIDRatioBased(SamplingRatio)
)
```

When tracing is disabled, `Start` still returns a valid `Span`, but no OpenTelemetry trace is created.

## Logging

Logging is enabled when `Logger.Severity` is not empty.

```go
Logger: otx.LoggerConfig{
	Severity: "info",
},
```

Supported levels:

```go
otx.SeverityTrace
otx.SeverityDebug
otx.SeverityInfo
otx.SeverityWarn
otx.SeverityError
otx.SeverityFatal
```

The configured severity is the minimum exported severity.

For example:

```go
Logger: otx.LoggerConfig{
	Severity: "warn",
},
```

exports:

```text
warn
error
fatal
```

### Fatal

`Fatal` records a critical error but **does not terminate the application**.

```go
span.Fatal(err)
```

It does not call `os.Exit`, `panic`, or otherwise terminate the process.

## Console Logging

Console logging is enabled independently:

```go
Logger: otx.LoggerConfig{
	Severity: "info",
	Console:  true,
},
```

Logs are written as structured JSON to `stdout` using Go's `log/slog` JSON handler.

Example:

```json
{"time":"2026-08-17T10:00:00Z","level":"INFO","msg":"user created","user_id":"123"}
```

This is suitable for Docker, Kubernetes, and other environments where application `stdout` is collected by infrastructure.

The console logger uses the same minimum severity configured for OpenTelemetry logging.

## Spans

A span represents a unit of work within a trace.

```go
span := otx.Start(ctx)
defer span.End()
```

A span also provides structured logging methods, allowing logs to be correlated with the span.

```go
span.Info("processing request")
span.Debug("loading user")
span.Warn("cache miss")
span.Error(err)
```

### Span Configuration

```go
span := otx.Start(ctx, otx.SpanConfig{
	Name: "create-user",
	Kind: otx.KindServer,
	Attributes: map[string]any{
		"user_id": "123",
		"source":  "api",
		"attempt": 1,
	},
})

defer span.End()
```

Only the first `SpanConfig` supplied to `Start` or `Link` is used.

### Span Name

If `Name` is omitted, the name is derived from the calling function when metadata is enabled.

Explicit names are recommended for important operations:

```go
span := otx.Start(ctx, otx.SpanConfig{
	Name: "create-user",
})
```

### Span Kind

Supported kinds:

```go
otx.KindInternal
otx.KindServer
otx.KindClient
otx.KindProducer
otx.KindConsumer
```

Example:

```go
span := otx.Start(ctx, otx.SpanConfig{
	Name: "publish-order",
	Kind: otx.KindProducer,
})
defer span.End()
```

## Span Attributes

Attributes are provided as `map[string]any`:

```go
span := otx.Start(ctx, otx.SpanConfig{
	Attributes: map[string]any{
		"user_id":  123,
		"username": "gokul",
		"premium":  true,
	},
})
```

Attribute values are converted to strings before being attached to telemetry.

The span attributes are also attached to log records emitted through that span.

## Structured Logging

All logging methods accept optional attributes:

```go
span.Info("user created", map[string]any{
	"user_id":  userID,
	"username": username,
})
```

Available methods:

```go
span.Trace(...)
span.Debug(...)
span.Info(...)
span.Warn(...)
span.Error(...)
span.Fatal(...)
```

### Error

```go
span.Error(err, map[string]any{
	"operation": "create-user",
})
```

An error:

* Records the error on the OpenTelemetry span
* Sets the span status to `Error`
* Emits the record at error severity

`Error(nil)` is ignored.

### Fatal

```go
span.Fatal(err, map[string]any{
	"operation": "database-migration",
})
```

`Fatal` records fatal telemetry without terminating the application.

## Trace and Log Correlation

Logs emitted through a span use the span's context:

```go
span := otx.Start(ctx)
defer span.End()

span.Info("processing request")
```

This allows an OpenTelemetry backend to associate the log with the trace and span that produced it.

Conceptually:

```text
Trace
 |
 +-- Span
      |
      +-- Log: processing request
      +-- Log: database query
      +-- Log: request completed
```

## Span Context

Use `Span.Ctx()` when continuing work within the current span:

```go
span := otx.Start(ctx)
defer span.End()

ctx := span.Ctx()

doSomething(ctx)
```

`Span.Ctx()` contains the span's OpenTelemetry trace context.

## Asynchronous Work and Span Links

Use `Link` when work crosses an asynchronous boundary such as:

* Message queues
* NATS
* Kafka
* RabbitMQ
* Background jobs
* Event-driven processing

A producer can inject its context into a message:

```go
span := otx.Start(ctx, otx.SpanConfig{
	Name: "publish-order",
	Kind: otx.KindProducer,
})
defer span.End()

headers := map[string]string{}

otx.Inject(span.Ctx(), headers)

publish(headers)
```

The consumer extracts the context and creates a linked span:

```go
ctx := otx.Extract(context.Background(), headers)

span := otx.Link(ctx, otx.SpanConfig{
	Name: "process-order",
	Kind: otx.KindConsumer,
})
defer span.End()
```

`Link` creates a new root span and attaches the extracted span context as a span link rather than making the consumer span a direct child.

```text
Trace
 |
 +-- Producer Span
 |      |
 |      +-- Message
 |            |
 |            +-------------------+
 |                                |
 +-- Consumer Span <---------------+
        |
        +-- Span Link -> Producer Span
```

## Trace Context Propagation

The package uses:

* W3C Trace Context
* W3C Baggage

### Inject

```go
headers := map[string]string{}

otx.Inject(ctx, headers)
```

The resulting map can be transported through:

* HTTP headers
* Queue messages
* NATS headers
* Kafka headers
* Custom RPC metadata

### Extract

```go
ctx := otx.Extract(context.Background(), headers)
```

The returned context can then be used with `Start` or `Link`.

## Context Management

### WithNewTimeout

`WithNewTimeout` creates a new timeout/cancellation boundary while preserving the OpenTelemetry trace context.

```go
ctx, cancel := otx.WithNewTimeout(ctx, 30*time.Second)
defer cancel()
```

The returned context does not inherit the original deadline or cancellation.

### WithoutTimeout

`WithoutTimeout` removes the deadline and cancellation from the supplied context while preserving its OpenTelemetry trace context.

```go
ctx := otx.WithoutTimeout(ctx)
```

This is useful for background work that should continue after the original request completes.

```text
HTTP Request Context
 |
 +-- Request Span
 |
 +-- Background Work
       |
       +-- Same trace context
       +-- Independent cancellation
```

## Metrics

Enable metrics with:

```go
Meter: otx.MeterConfig{
	Enabled: true,
},
```

The package creates an OpenTelemetry `MeterProvider` and exports metrics through OTLP/HTTP.

The high-level API currently focuses on configuring and managing the provider. Use the underlying OpenTelemetry SDK when direct metric instrumentation is required.

## Underlying OpenTelemetry APIs

An explicit client can expose the underlying tracer provider:

```go
client, err := config.Client(ctx)
if err != nil {
	return err
}

provider := client.TraceProvider()
```

`TraceProvider()` returns `nil` when tracing is disabled.

The configured propagator is also available:

```go
propagator := client.Propagator()
```

`Propagator()` returns `nil` when tracing is disabled.

## Global Client

For application-wide telemetry, call `Connect` once during startup:

```go
err := (otx.Config{
	Name: "my-service",

	Tracer: otx.TracerConfig{
		SamplingRatio: 1.0,
	},
}).Connect(ctx)

if err != nil {
	return err
}
```

Then use the package-level API:

```go
span := otx.Start(ctx)
defer span.End()

otx.Inject(ctx, headers)

ctx = otx.Extract(ctx, headers)

ctx, cancel := otx.WithNewTimeout(ctx, time.Second)
defer cancel()

ctx = otx.WithoutTimeout(ctx)

otx.Shutdown(ctx)
```

`Connect` must succeed before using the global API.

## Explicit Client

Alternatively, create and manage a client explicitly:

```go
client, err := config.Client(ctx)
if err != nil {
	return err
}

defer client.Shutdown(ctx)

span := client.Start(ctx)
defer span.End()
```

Explicit clients are useful when dependency injection or multiple independently managed telemetry clients are required.

## Application Lifecycle

A typical application lifecycle is:

```text
Application startup
       |
       v
  Config.Connect()
       |
       v
 Application runs
       |
       +--> Start spans
       +--> Emit logs
       +--> Record metrics
       +--> Inject/extract context
       |
       v
 Graceful shutdown
       |
       v
    Shutdown()
       |
       v
 Flush telemetry
       |
       v
      Exit
```

Example:

```go
func main() {
	ctx := context.Background()

	config := otx.Config{
		Name:        "user-service",
		Namespace:   "my-application",
		Environment: "production",
		Version:     "1.0.0",
		Endpoint:    "otel-collector:4318",

		Tracer: otx.TracerConfig{
			SamplingRatio: 1.0,
		},

		Logger: otx.LoggerConfig{
			Severity: "info",
			Console:  true,
		},

		Meter: otx.MeterConfig{
			Enabled: true,
		},
	}

	if err := config.Connect(ctx); err != nil {
		panic(err)
	}

	defer otx.Shutdown(ctx)

	runApplication(ctx)
}
```

## Source-Code Metadata

Source metadata is enabled by default.

The package can attach:

* Source file
* Function
* Line number

to spans and log records.

Disable it with:

```go
WithoutMetadata: true,
```

Disabling metadata can reduce runtime overhead and telemetry volume.

## Error and Span Status

Calling:

```go
span.Error(err)
```

records the error and marks the span as failed.

`Fatal` also marks the span as failed.

Normal `Trace`, `Debug`, `Info`, and `Warn` events do not automatically mark a span as failed.

## Shutdown

Providers use batching, so applications should call `Shutdown` during graceful termination:

```go
defer otx.Shutdown(ctx)
```

Shutdown flushes pending telemetry and shuts down all enabled providers.

The enabled providers are flushed and shut down before `Shutdown` returns.

If a provider fails to flush or shut down, the error is returned.

## Environment-Based Configuration

`Config` works well with environment variables:

```go
config := otx.Config{
	Name:        os.Getenv("OTEL_SERVICE_NAME"),
	Namespace:   os.Getenv("OTEL_SERVICE_NAMESPACE"),
	Environment: os.Getenv("APP_ENVIRONMENT"),
	Version:     os.Getenv("APP_VERSION"),
	InstanceID:  os.Getenv("APP_INSTANCE_ID"),

	Endpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	Insecure: os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "true",

	Tracer: otx.TracerConfig{
		SamplingRatio: 1.0,
	},

	Logger: otx.LoggerConfig{
		Severity: os.Getenv("LOG_LEVEL"),
		Console:  os.Getenv("LOG_CONSOLE") == "true",
	},

	Meter: otx.MeterConfig{
		Enabled: os.Getenv("OTEL_METRICS_ENABLED") == "true",
	},
}
```

This allows the same application binary to use different configurations across environments.

Example:

```text
Development
    LOG_LEVEL=debug
    LOG_CONSOLE=true
    OTEL_EXPORTER_OTLP_INSECURE=true

Staging
    LOG_LEVEL=info
    LOG_CONSOLE=true

Production
    LOG_LEVEL=warn
    LOG_CONSOLE=false
```

## OTLP Architecture

The package exports telemetry using OTLP over HTTP.

```text
+----------------+
|   Go Service   |
|                |
|    go-otx      |
+-------+--------+
        |
        | OTLP / HTTP
        v
+----------------+
| OTel Collector |
+-------+--------+
        |
   +----+----+----+
   |         |    |
   v         v    v
Traces     Logs  Metrics
```

The OTLP endpoint should normally point to an OpenTelemetry Collector or another OTLP-compatible backend.

## Examples

### HTTP Request

```go
func (h *Handler) CreateUser(ctx context.Context) error {
	span := otx.Start(ctx, otx.SpanConfig{
		Name: "create-user",
		Kind: otx.KindServer,
	})
	defer span.End()

	span.Info("creating user")

	user, err := h.service.CreateUser(span.Ctx())
	if err != nil {
		span.Error(err)
		return err
	}

	span.Info("user created", map[string]any{
		"user_id": user.ID,
	})

	return nil
}
```

### Outbound Request

```go
func (c *Client) GetUser(ctx context.Context, id string) error {
	span := otx.Start(ctx, otx.SpanConfig{
		Name: "get-user",
		Kind: otx.KindClient,
	})
	defer span.End()

	span.Info("sending user request")

	// Perform outbound request using span.Ctx().

	return nil
}
```

### Queue Producer

```go
func publishOrder(ctx context.Context, orderID string) error {
	span := otx.Start(ctx, otx.SpanConfig{
		Name: "publish-order",
		Kind: otx.KindProducer,
	})
	defer span.End()

	headers := map[string]string{}

	otx.Inject(span.Ctx(), headers)

	// Publish orderID and headers to the queue.

	span.Info("order published", map[string]any{
		"order_id": orderID,
	})

	return nil
}
```

### Queue Consumer

```go
func consumeOrder(headers map[string]string, orderID string) error {
	ctx := otx.Extract(context.Background(), headers)

	span := otx.Link(ctx, otx.SpanConfig{
		Name: "consume-order",
		Kind: otx.KindConsumer,
	})
	defer span.End()

	span.Info("processing order", map[string]any{
		"order_id": orderID,
	})

	return nil
}
```

## API Reference

### Global Functions

```go
Start(context.Context, ...SpanConfig) Span

Link(context.Context, ...SpanConfig) Span

Shutdown(context.Context) error

Inject(context.Context, map[string]string)

Extract(context.Context, map[string]string) context.Context

WithNewTimeout(context.Context, time.Duration) (context.Context, context.CancelFunc)

WithoutTimeout(context.Context) context.Context
```

All global functions require `Config.Connect()` to have completed successfully.

### Client

```go
type Client interface {
	Start(context.Context, ...SpanConfig) Span
	Link(context.Context, ...SpanConfig) Span

	Shutdown(context.Context) error

	Propagator() propagation.TextMapPropagator
	TraceProvider() *sdkTrace.TracerProvider

	Inject(context.Context, map[string]string)
	Extract(context.Context, map[string]string) context.Context

	WithNewTimeout(context.Context, time.Duration) (context.Context, context.CancelFunc)
	WithoutTimeout(context.Context) context.Context
}
```

### Span

```go
type Span interface {
	End()

	Trace(string, ...map[string]any)
	Debug(string, ...map[string]any)
	Info(string, ...map[string]any)
	Warn(string, ...map[string]any)
	Error(error, ...map[string]any)
	Fatal(error, ...map[string]any)

	Ctx() context.Context
}
```

### Configuration

```go
type Config struct {
	Name            string
	Namespace       string
	Environment     string
	Version         string
	InstanceID      string
	Endpoint        string
	Insecure        bool
	WithoutMetadata bool

	Tracer TracerConfig
	Meter  MeterConfig
	Logger LoggerConfig
}
```

```go
type TracerConfig struct {
	SamplingRatio float64
}
```

```go
type MeterConfig struct {
	Enabled bool
}
```

```go
type LoggerConfig struct {
	Severity string
	Console  bool
}
```

```go
type SpanConfig struct {
	Name       string
	Kind       string
	Attributes map[string]any
}
```

## Design Principles

### One Client per Application

Initialize telemetry once during application startup and reuse it:

```text
Application
    |
    +-- One otx Client
          |
          +-- Request 1
          +-- Request 2
          +-- Request 3
          +-- ...
```

### Context Carries Trace State

Trace state is carried through `context.Context`.

Use:

```go
span.Ctx()
```

when continuing work inside the current span.

### Explicit Propagation for Custom Transports

For transports without automatic OpenTelemetry instrumentation:

```go
otx.Inject(...)
otx.Extract(...)
```

Use these functions to explicitly propagate trace context.

### Asynchronous Work Uses Links

For asynchronous boundaries such as queues, use `Link` when the consumer should be related to the producer without being represented as its direct child.

## Dependencies

The package uses:

* OpenTelemetry API
* OpenTelemetry SDK
* OTLP HTTP trace exporter
* OTLP HTTP log exporter
* OTLP HTTP metric exporter
* Go `log/slog`

## License

See [LICENSE](LICENSE).
