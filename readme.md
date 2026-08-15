# go-otel

A lightweight Go wrapper around [OpenTelemetry](https://opentelemetry.io/) providing a unified API for **distributed tracing, structured logging, metrics, and trace-context propagation**.

The package provides a simple application-level telemetry client while retaining access to the underlying OpenTelemetry SDK when required.

## Features

* Distributed tracing with OTLP/HTTP export
* Parent-based trace sampling
* Structured logging with severity filtering
* Metrics with OTLP/HTTP export
* W3C Trace Context propagation
* W3C Baggage propagation
* Span-level structured logging
* Automatic source-code metadata
* Service and instance resource attributes
* Graceful telemetry shutdown
* Context utilities for independent background work
* Access to the underlying OpenTelemetry `TracerProvider`

## Installation

```bash
go get github.com/7e3b/go-otel
```

## Quick Start

Create a single `Client` during application startup and reuse it throughout the application.

```go
package main

import (
	"context"
	"log"

	otel "github.com/7e3b/go-otel"
)

func main() {
	ctx := context.Background()

	client, err := (otel.Config{
		Name:        "my-service",
		Namespace:   "my-application",
		Environment: "production",
		Version:     "1.0.0",
		InstanceID:  "instance-1",
		Endpoint:    "localhost:4318",

		Tracer: otel.TracerConfig{
			SamplingRatio: 1.0,
		},

		Logger: otel.LoggerConfig{
			Severity: otel.SeverityInfo,
		},

		Meter: otel.MeterConfig{
			Enabled: true,
		},
	}).Client(ctx)

	if err != nil {
		log.Fatal(err)
	}

	defer client.Shutdown(ctx)

	span := client.Start(ctx)
	defer span.End()

	span.Info("processing request")
}
```

## Configuration

`Config` controls service identity and the telemetry providers.

```go
otel.Config{
	Name:        "my-service",
	Namespace:   "my-application",
	Environment: "production",
	Version:     "1.0.0",
	InstanceID:  "instance-1",
	Endpoint:    "localhost:4318",

	Tracer: otel.TracerConfig{
		SamplingRatio: 1.0,
	},

	Logger: otel.LoggerConfig{
		Severity: otel.SeverityInfo,
	},

	Meter: otel.MeterConfig{
		Enabled: true,
	},
}
```

### Service Identity

The following configuration fields are mapped to OpenTelemetry resource attributes:

| Config        | OpenTelemetry attribute       |
| ------------- | ----------------------------- |
| `Name`        | `service.name`                |
| `Namespace`   | `service.namespace`           |
| `Version`     | `service.version`             |
| `InstanceID`  | `service.instance.id`         |
| `Environment` | `deployment.environment.name` |
| `Hostname`    | `host.name`                   |

`InstanceID` and `HostName` represent different concepts. `InstanceID` identifies the specific service instance, while `HostName` is automatically obtained from the operating system.

### OTLP Endpoint

`Endpoint` specifies the OTLP HTTP exporter endpoint.

For example:

```text
localhost:4318
```

The same endpoint is used by the enabled tracing, logging, and metrics exporters.

For a local collector that does not use TLS:

```go
otel.Config{
	Endpoint: "localhost:4318",
	Insecure: true,
}
```

## Distributed Tracing

### Starting a Span

```go
span := client.Start(ctx)
defer span.End()
```

A span represents a unit of work within a distributed trace.

### Naming Spans

A span name can be explicitly provided:

```go
span := client.Start(ctx, otel.SpanConfig{
	Name: "process-user",
})
defer span.End()
```

When no name is provided, the client derives the name from the operation's source metadata.

### Span Attributes

Attributes can be supplied when the span is created:

```go
span := client.Start(ctx, otel.SpanConfig{
	Name: "process-user",
	Attributes: map[string]any{
		"user.id":    userID,
		"user.type":  "premium",
		"request.id": requestID,
	},
})
defer span.End()
```

These attributes are attached to the span and are also available on log records emitted through that span.

### Span Kinds

The package supports the standard OpenTelemetry span kinds:

```go
otel.KindInternal
otel.KindServer
otel.KindClient
otel.KindProducer
otel.KindConsumer
```

For example, an outbound database or HTTP operation can be represented as a client span:

```go
span := client.Start(ctx, otel.SpanConfig{
	Name: "postgres.query",
	Kind: otel.KindClient,
})
defer span.End()
```

When no kind is specified, the span kind is left unspecified.

## Structured Logging

Logging is performed through the span:

```go
span.Info("user authenticated")
span.Debug("cache lookup completed")
span.Warn("cache miss")
span.Error(err)
```

Supported severities are:

```go
otel.SeverityTrace
otel.SeverityDebug
otel.SeverityInfo
otel.SeverityWarn
otel.SeverityError
otel.SeverityFatal
```

Configure the minimum exported severity:

```go
Logger: otel.LoggerConfig{
	Severity: otel.SeverityInfo,
},
```

With this configuration, `Trace` and `Debug` records are not exported, while `Info`, `Warn`, `Error`, and `Fatal` records are.

### Structured Attributes

Additional attributes can be attached to individual log records:

```go
span.Info(
	"user authenticated",
	map[string]any{
		"user.id":     userID,
		"auth.method": "otp",
	},
)
```

### Errors

Use `Error` when an error is available:

```go
if err != nil {
	span.Error(err)
	return err
}
```

The error is recorded as an OpenTelemetry error and the associated span status is set to `Error`.

Additional attributes can also be provided:

```go
span.Error(
	err,
	map[string]any{
		"user.id":   userID,
		"operation": "database.query",
	},
)
```

### Fatal

`Fatal` records an event at fatal severity and marks the span as failed.

It does **not** terminate the application:

```go
span.Fatal(err)
```

## Span and Log Correlation

Logs emitted through a `Span` use the span's context.

This allows an observability backend such as SigNoz to correlate logs with traces:

```text
Trace
 └── Span
      ├── Log: request received
      ├── Log: database query
      ├── Log: cache miss
      └── Log: request completed
```

This makes it possible to navigate from a trace to the logs generated during that operation.

## Source Metadata

Source metadata collection is enabled by default.

Telemetry can include:

* source file
* function
* line number

For example:

```text
file:     user/service.go
function: user.(*Service).Create
line:     42
```

To disable metadata collection:

```go
otel.Config{
	WithoutMetadata: true,
}
```

Disabling metadata reduces the runtime overhead associated with collecting source information.

## Trace Context Propagation

The client provides an OpenTelemetry text-map propagator supporting:

* W3C Trace Context
* W3C Baggage

Retrieve the propagator with:

```go
propagator := client.Propagator()
```

### HTTP Propagation

For transports that are not automatically instrumented, trace context can be injected into outbound requests:

```go
carrier := propagation.HeaderCarrier(req.Header)

client.Propagator().Inject(ctx, carrier)
```

Incoming trace context can similarly be extracted:

```go
ctx = client.Propagator().Extract(ctx, propagation.HeaderCarrier(req.Header))
```

This allows traces to continue across service boundaries.

## Context Utilities

The client provides utilities for creating contexts that preserve OpenTelemetry propagation information.

### WithNewTimeout

Creates a new context with a new timeout without inheriting the cancellation or deadline of the supplied context.

```go
ctx, cancel := client.WithNewTimeout(ctx, 30*time.Second)
defer cancel()
```

The OpenTelemetry trace and baggage propagation information is preserved.

### WithoutTimeout

Creates a context that does not inherit the deadline or cancellation of the original context:

```go
ctx = client.WithoutTimeout(ctx)
```

The OpenTelemetry trace and baggage propagation information is preserved.

This is useful for background work that should continue after the original request context is cancelled.

Note that arbitrary values stored in the original `context.Context` are not copied.

## Sampling

Tracing uses a parent-based ratio sampler.

```go
Tracer: otel.TracerConfig{
	SamplingRatio: 0.1,
},
```

| Sampling Ratio | Behavior                            |
| -------------: | ----------------------------------- |
|    `0` or less | Tracing disabled                    |
|          `0.1` | Approximately 10% of traces sampled |
|          `0.5` | Approximately 50% of traces sampled |
|          `1.0` | All traces sampled                  |

Because the sampler is parent-based, child spans follow the sampling decision of a valid parent span.

## Metrics

Metrics can be enabled independently of tracing and logging:

```go
Meter: otel.MeterConfig{
	Enabled: true,
},
```

When enabled, the client initializes an OpenTelemetry `MeterProvider` and exports metrics through OTLP/HTTP.

## Provider Access

The client can expose the underlying SDK tracer provider when direct SDK integration is required:

```go
provider := client.TraceProvider()
```

For example, this can be passed to libraries that accept a `*trace.TracerProvider`.

`TraceProvider()` returns `nil` when tracing is disabled.

## Graceful Shutdown

Telemetry providers should be flushed and shut down during application shutdown:

```go
ctx, cancel := context.WithTimeout(
	context.Background(),
	10*time.Second,
)
defer cancel()

if err := client.Shutdown(ctx); err != nil {
	log.Printf("failed to shutdown telemetry: %v", err)
}
```

`Shutdown` flushes pending telemetry before shutting down the enabled providers.

## Recommended Application Lifecycle

A typical application should follow this lifecycle:

```text
Application startup
       │
       ▼
Create Client
       │
       ▼
Initialize HTTP / DB / Messaging clients
       │
       ▼
Handle requests
       │
       ├── Start Span
       │      ├── Attributes
       │      ├── Info / Debug / Warn
       │      ├── Error
       │      └── End
       │
       ▼
Application shutdown
       │
       ▼
Client.Shutdown()
```

The telemetry `Client` should normally be created once and reused throughout the application's lifetime.

## Example: Nested Operations

```go
func CreateUser(ctx context.Context, client otel.Client) error {
	span := client.Start(ctx, otel.SpanConfig{
		Name: "user.create",
		Kind: otel.KindInternal,
	})
	defer span.End()

	span.Info("creating user")

	if err := validateUser(); err != nil {
		span.Error(err)
		return err
	}

	if err := saveUser(ctx); err != nil {
		span.Error(err)
		return err
	}

	span.Info("user created")

	return nil
}
```

A downstream operation can create its own child span using the context associated with the parent operation.

## Example: HTTP Request

For an HTTP server, the request context should first contain the extracted incoming trace context. The request handler can then create a server span:

```go
func Handler(ctx context.Context, client otel.Client) error {
	span := client.Start(ctx, otel.SpanConfig{
		Name: "GET /users",
		Kind: otel.KindServer,
	})
	defer span.End()

	span.Info("request received")

	// Application logic...

	return nil
}
```

An outbound request can use a client span:

```go
span := client.Start(ctx, otel.SpanConfig{
	Name: "GET users-service",
	Kind: otel.KindClient,
})
defer span.End()

// Inject the trace context into the outbound request.
```

## OpenTelemetry Model

This package follows the OpenTelemetry model while providing a simpler application-facing API:

```text
Resource
  │
  ├── Service identity
  ├── Environment
  ├── Instance
  └── Host
       │
       ├── Traces
       │    └── Spans
       │         ├── Attributes
       │         └── Events
       │
       ├── Logs
       │    ├── Severity
       │    ├── Body
       │    └── Attributes
       │
       └── Metrics
```

The underlying OpenTelemetry SDK remains accessible when deeper integration is required.

## License

See [LICENSE](LICENSE).
