# go-otx

A lightweight Go wrapper around [OpenTelemetry](https://opentelemetry.io/) providing a unified API for:

* Distributed tracing
* Structured logging
* Metrics
* Trace-context propagation
* Context timeout/cancellation management

The package provides a simple application-level telemetry client while retaining access to the underlying OpenTelemetry SDK when required.

## Features

* Distributed tracing through OpenTelemetry
* Configurable trace sampling
* Structured logging through OpenTelemetry Logs
* Console JSON logging through Go's `log/slog`
* Trace-correlated logs
* Log severity levels:

  * Trace
  * Debug
  * Info
  * Warn
  * Error
  * Fatal
* OpenTelemetry metrics
* OTLP/HTTP exporters
* Gzip compression for OTLP exports
* Configurable TLS/insecure OTLP connections
* W3C Trace Context propagation
* W3C Baggage propagation
* Manual context injection and extraction
* Span links for asynchronous operations such as queues
* Context creation with a new timeout while preserving trace context
* Context creation without inheriting the parent deadline or cancellation
* Optional source-code metadata
* Access to the underlying OpenTelemetry `TracerProvider`
* Package-level global client or explicitly managed client
* Graceful shutdown with telemetry flushing

## Installation

```bash
go get github.com/7e3b/go-otx
```

## Quick Start

The package can be configured once during application startup and then used throughout the application.

```go
package main

import (
	"context"
	"log"

	"github.com/7e3b/go-otx"
)

func main() {
	ctx := context.Background()

	err := otx.Config{
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
	}.Connect(ctx)

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

The main configuration type is `otx.Config`.

```go
type Config struct {
	Name             string
	Namespace        string
	Environment      string
	Version          string
	InstanceID       string
	Endpoint         string
	Insecure         bool
	WithoutMetadata  bool
	Tracer           TracerConfig
	Meter            MeterConfig
	Logger           LoggerConfig
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

These values are used to populate OpenTelemetry resource attributes:

| Config        | OpenTelemetry attribute       |
| ------------- | ----------------------------- |
| `Name`        | `service.name`                |
| `Namespace`   | `service.namespace`           |
| `Environment` | `deployment.environment.name` |
| `Version`     | `service.version`             |
| `InstanceID`  | `service.instance.id`         |
| Hostname      | `host.name`                   |

The host name is obtained automatically from the running system.

## OTLP Endpoint

The same OTLP/HTTP endpoint configuration is used by the enabled tracing, logging, and metrics exporters.

```go
Endpoint: "localhost:4318",
Insecure: true,
```

For a TLS-enabled collector:

```go
Endpoint: "otel-collector.example.com:4318",
Insecure: false,
```

`Insecure: true` disables TLS and is generally useful for local development or private environments where TLS is not required.

## Tracing

Tracing is enabled when `SamplingRatio` is greater than zero.

```go
Tracer: otx.TracerConfig{
	SamplingRatio: 1.0,
},
```

Examples:

```text
1.0  -> sample approximately 100% of traces
0.5  -> sample approximately 50% of traces
0.1  -> sample approximately 10% of traces
0.01 -> sample approximately 1% of traces
```

The package uses a parent-based sampler with a trace-ID ratio sampler:

```text
ParentBased(
    TraceIDRatioBased(SamplingRatio)
)
```

If tracing is disabled, `Start` still returns a valid `Span` object, but no OpenTelemetry trace is created.

## Logging

Logging is enabled when `Logger.Severity` is not empty.

```go
Logger: otx.LoggerConfig{
	Severity: "info",
},
```

Supported severities are:

```go
otx.SeverityTrace
otx.SeverityDebug
otx.SeverityInfo
otx.SeverityWarn
otx.SeverityError
otx.SeverityFatal
```

Or their string values:

```text
trace
debug
info
warn
error
fatal
```

The configured severity acts as the minimum exported severity.

For example:

```go
Logger: otx.LoggerConfig{
	Severity: "warn",
},
```

will export:

```text
warn
error
fatal
```

while `trace`, `debug`, and `info` records are filtered out.

### Fatal

`Fatal` represents a critical error, but it **does not terminate the application**.

```go
span.Fatal(err)
```

This is intentional. The package records the event as fatal telemetry without calling `os.Exit`, `panic`, or otherwise terminating the process.

## Console Logging

Console logging can be enabled independently using `LoggerConfig.Console`.

```go
Logger: otx.LoggerConfig{
	Severity: "info",
	Console:  true,
},
```

When enabled, log records are also written as structured JSON to the process's standard output using Go's `log/slog` JSON handler.

For example:

```json
{"time":"2026-08-17T10:00:00Z","level":"INFO","msg":"user created","user_id":"123"}
```

This makes the output suitable for containerized applications where the container runtime, Docker, Kubernetes, or a log collector reads the application's `stdout`.

The console logger uses the same configured minimum severity.

### Why JSON Console Logging?

Console logging is intended primarily for environments where application output is collected by the runtime or infrastructure.

For example:

```text
Go application
      |
      v
  slog JSON
      |
    stdout
      |
      v
Docker / Kubernetes
      |
      v
Log collector
```

The OpenTelemetry logger continues to export structured log records through OTLP independently.

## Spans

A span represents a unit of work within a distributed trace.

```go
span := otx.Start(ctx)
defer span.End()
```

The span also provides structured logging methods, allowing logs and traces to be correlated.

```go
span.Info("processing request")
span.Debug("loading user")
span.Warn("cache miss")
span.Error(err)
```

## Span Configuration

A span can be configured using `SpanConfig`.

```go
span := otx.Start(ctx, otx.SpanConfig{
	Name: "create-user",
	Kind: otx.KindServer,
	Attributes: map[string]any{
		"user_id":  "123",
		"source":   "api",
		"attempt":  1,
	},
})

defer span.End()
```

Only the first `SpanConfig` supplied to `Start` or `Link` is used.

### Span Name

If `Name` is omitted, the package derives the name from the calling function when source-code metadata is enabled.

```go
span := otx.Start(ctx)
```

Explicit names are recommended for important application operations:

```go
span := otx.Start(ctx, otx.SpanConfig{
	Name: "create-user",
})
```

### Span Kind

Supported span kinds are:

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
	Name: "publish-event",
	Kind: otx.KindProducer,
})

defer span.End()
```

## Span Attributes

Attributes can be provided as a `map[string]any`.

```go
span := otx.Start(ctx, otx.SpanConfig{
	Attributes: map[string]any{
		"user_id":  123,
		"username": "gokul",
		"premium":  true,
	},
})
```

Values are converted to strings before being attached to telemetry.

The same span attributes are also available on log records emitted through that span.

## Structured Logging

Logging methods accept an optional attributes map.

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

### Trace

```go
span.Trace("entering repository")
```

### Debug

```go
span.Debug("cache lookup completed", map[string]any{
	"key": "user:123",
})
```

### Info

```go
span.Info("user created", map[string]any{
	"user_id": 123,
})
```

### Warn

```go
span.Warn("cache miss", map[string]any{
	"key": "user:123",
})
```

### Error

```go
span.Error(err, map[string]any{
	"operation": "create-user",
})
```

An error record:

* Records the error on the OpenTelemetry span
* Marks the span status as `Error`
* Emits the error at error severity

A nil error is ignored.

### Fatal

```go
span.Fatal(err, map[string]any{
	"operation": "database-migration",
})
```

`Fatal` behaves like a fatal telemetry event but does not terminate the application.

## Trace and Log Correlation

Logs emitted through a `Span` use the span's context.

```go
span := otx.Start(ctx)
defer span.End()

span.Info("processing request")
```

This allows an OpenTelemetry backend such as SigNoz to correlate the log record with the trace and span that produced it.

The resulting relationship is conceptually:

```text
Trace
 |
 +-- Span
      |
      +-- Log: processing request
      |
      +-- Log: database query
      |
      +-- Log: request completed
```

## Span Context

A span exposes its context through `Ctx()`.

```go
span := otx.Start(ctx)
defer span.End()

ctx := span.Ctx()

doSomething(ctx)
```

Use `Span.Ctx()` when an operation needs to continue the trace from the current span.

## Linking Asynchronous Work

`Link` is intended for asynchronous operations where the new span should reference another span without becoming its direct child.

This is particularly useful with:

* Message queues
* NATS
* Kafka
* RabbitMQ
* Background jobs
* Event-driven processing

For example, a producer can inject its trace context into a message:

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

The consumer can then extract the context:

```go
ctx := otx.Extract(context.Background(), headers)

span := otx.Link(ctx, otx.SpanConfig{
	Name: "process-order",
	Kind: otx.KindConsumer,
})
defer span.End()
```

`Link` creates a new root span and attaches the extracted span context as a span link.

Conceptually:

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

This avoids representing asynchronous work as a direct parent-child relationship while still preserving the relationship between the operations.

## Trace Context Propagation

The package uses:

```text
W3C Trace Context
W3C Baggage
```

through:

```go
propagation.NewCompositeTextMapPropagator(
	propagation.TraceContext{},
	propagation.Baggage{},
)
```

### Inject

Inject the current trace context into a string map:

```go
headers := map[string]string{}

otx.Inject(ctx, headers)
```

The resulting map can be transported with:

* HTTP headers
* Queue messages
* NATS message headers
* Kafka headers
* Custom RPC metadata
* Other message transports

### Extract

Extract the context on the receiving side:

```go
ctx := otx.Extract(context.Background(), headers)
```

The resulting context can then be used to create a span:

```go
span := otx.Link(ctx)
defer span.End()
```

## Propagator

The configured propagator can also be accessed directly from a client.

```go
client, err := config.Client(ctx)
if err != nil {
	return err
}

propagator := client.Propagator()
```

`Propagator()` returns `nil` when tracing is disabled.

## Context Management

The package provides helpers for creating contexts while preserving OpenTelemetry trace context.

### WithNewTimeout

`WithNewTimeout` creates a new timeout/cancellation boundary while preserving the current OpenTelemetry trace context.

```go
ctx, cancel := otx.WithNewTimeout(ctx, 30*time.Second)
defer cancel()
```

The new context does not inherit the previous context's deadline or cancellation, but retains its OpenTelemetry trace context.

This is useful when an operation needs its own independent timeout.

### WithoutTimeout

`WithoutTimeout` removes the deadline and cancellation inherited from the supplied context while preserving its OpenTelemetry trace context.

```go
ctx := otx.WithoutTimeout(ctx)
```

This is useful for background work that should continue after the original request has completed.

For example:

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

Metrics are enabled using:

```go
Meter: otx.MeterConfig{
	Enabled: true,
},
```

The package initializes an OpenTelemetry SDK `MeterProvider` and exports metrics through OTLP/HTTP.

The current high-level `Client` API focuses on configuring and managing the metrics provider. The underlying provider can be accessed through the OpenTelemetry SDK when direct metric instrumentation is required.

## Underlying Trace Provider

The underlying OpenTelemetry SDK `TracerProvider` can be accessed through:

```go
provider := client.TraceProvider()
```

This is useful when integrating with libraries that expect:

```go
*trace.TracerProvider
```

For example:

```go
provider := client.TraceProvider()

if provider != nil {
	// Integrate provider with a library that accepts
	// *sdkTrace.TracerProvider.
}
```

`TraceProvider()` returns `nil` when tracing is disabled.

## Client vs Package-Level API

There are two ways to use the package.

### Package-Level API

Call `Config.Connect()` once:

```go
err := otx.Config{
	Name: "my-service",

	Tracer: otx.TracerConfig{
		SamplingRatio: 1.0,
	},
}.Connect(ctx)

if err != nil {
	return err
}
```

Then use:

```go
span := otx.Start(ctx)
defer span.End()
```

and:

```go
otx.Inject(ctx, headers)
otx.Extract(ctx, headers)
otx.WithNewTimeout(ctx, time.Second)
otx.WithoutTimeout(ctx)
otx.Shutdown(ctx)
```

This style is convenient for application-wide telemetry.

### Explicit Client

Alternatively, create an explicit client:

```go
client, err := config.Client(ctx)
if err != nil {
	return err
}

defer client.Shutdown(ctx)

span := client.Start(ctx)
defer span.End()
```

This approach is useful when dependency injection is preferred or when an application needs to manage telemetry clients explicitly.

## Recommended Application Lifecycle

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
        |
        +--> Emit logs
        |
        +--> Record metrics
        |
        +--> Inject/extract trace context
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

		Endpoint: "otel-collector:4318",

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

Source-code metadata is enabled by default.

The package can attach:

* Source file
* Function
* Line number

to spans and log records.

For example:

```text
file=/app/user/service.go
function=github.com/example/user.(*Service).Create
line=42
```

This is useful during development and debugging.

### Disable Metadata

Metadata can be disabled:

```go
WithoutMetadata: true,
```

Disabling metadata can reduce:

* Runtime overhead
* Stack inspection
* Telemetry size
* Exported attribute volume

For high-throughput production services, disabling metadata may be desirable if source-level metadata is not required.

## Error Handling

Calling:

```go
span.Error(err)
```

records the error and marks the span as failed.

For example:

```go
span := otx.Start(ctx)
defer span.End()

err := repository.Create(ctx, user)
if err != nil {
	span.Error(err)
	return err
}
```

The error is recorded on the OpenTelemetry span using `RecordError`, and the span status is set to `Error`.

Calling `Error(nil)` does nothing.

## Span Status

An error recorded through `Error` or `Fatal` marks the span as failed.

For example:

```go
span.Error(err)
```

results in:

```text
Span Status = Error
```

An ordinary `Info`, `Debug`, `Trace`, or `Warn` event does not automatically mark the span as failed.

## Shutdown

Telemetry providers use batching, so applications should call `Shutdown` during graceful termination.

```go
defer otx.Shutdown(ctx)
```

Shutdown flushes pending telemetry and shuts down the enabled providers.

The shutdown sequence includes:

```text
TracerProvider
    -> ForceFlush
    -> Shutdown

LoggerProvider
    -> ForceFlush
    -> Shutdown

MeterProvider
    -> ForceFlush
    -> Shutdown
```

If a provider fails to flush or shut down, the error is returned.

## Environment-Based Configuration

`Config` is designed to work well with environment variables.

For example:

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

This allows the same application binary to use different telemetry configurations across environments.

For example:

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

## OTLP Collector

The package exports telemetry using OTLP over HTTP.

A typical architecture is:

```text
                +----------------+
                |   Go Service   |
                |                |
                |  go-otx        |
                +-------+--------+
                        |
                  OTLP / HTTP
                        |
                        v
                +----------------+
                | OTel Collector |
                +-------+--------+
                        |
             +----------+----------+
             |          |          |
             v          v          v
           Traces      Logs      Metrics
```

The OTLP endpoint should normally point to an OpenTelemetry Collector or another OTLP-compatible backend.

## Example: HTTP Request

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

## Example: Outbound Request

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

## Example: Queue Producer

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

## Example: Queue Consumer

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

Using `Link` for this type of asynchronous workflow preserves the relationship between the producer and consumer without making the consumer span a direct child of the producer span.

## API Reference

### Global Functions

```go
Start(context.Context, ...SpanConfig) Span

Shutdown(context.Context) error

WithNewTimeout(context.Context, time.Duration) (context.Context, context.CancelFunc)

WithoutTimeout(context.Context) context.Context
```

### Client Interface

```go
type Client interface {
	Start(context.Context, ...SpanConfig) Span

	Link(context.Context, ...SpanConfig) Span

	Shutdown(context.Context) error

	Propagator() propagation.TextMapPropagator

	WithNewTimeout(context.Context, time.Duration) (context.Context, context.CancelFunc)

	WithoutTimeout(context.Context) context.Context

	TraceProvider() *sdkTrace.TracerProvider

	Inject(context.Context, map[string]string)

	Extract(context.Context, map[string]string) context.Context
}
```

### Span Interface

```go
type Span interface {
	End()

	Trace(string, ...map[string]any)

	Info(string, ...map[string]any)

	Debug(string, ...map[string]any)

	Warn(string, ...map[string]any)

	Error(error, ...map[string]any)

	Fatal(error, ...map[string]any)

	Ctx() context.Context
}
```

### Configuration Types

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
	Tracer          TracerConfig
	Meter           MeterConfig
	Logger          LoggerConfig
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

Create the telemetry client once during application startup.

Avoid creating a new client for every request:

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

The package follows the standard OpenTelemetry model where trace state is carried through `context.Context`.

Use:

```go
span.Ctx()
```

when continuing work inside the current span.

### Explicit Propagation for Custom Transports

For automatically instrumented transports, instrumentation may handle propagation automatically.

For custom queues or messaging systems, use:

```go
otx.Inject(...)
otx.Extract(...)
```

to explicitly propagate trace context.

### Asynchronous Work Uses Links

When work crosses an asynchronous boundary such as a queue, use `Link` when the consumer should not be represented as a direct child of the producer.

## Dependencies

The package is built around the OpenTelemetry Go SDK and uses:

* OpenTelemetry API
* OpenTelemetry SDK
* OTLP HTTP trace exporter
* OTLP HTTP log exporter
* OTLP HTTP metric exporter
* Go `log/slog`

## License

See [LICENSE](LICENSE).
