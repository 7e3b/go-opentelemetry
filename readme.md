# go-opentelemetry

A lightweight Go wrapper around [OpenTelemetry](https://opentelemetry.io/) providing a unified API for **distributed tracing, structured logging, metrics, and trace-context propagation**.

## Installation

```bash
go get github.com/7e3b/go-opentelemetry
```

## Overview

`go-opentelemetry` provides a single application-level client for OpenTelemetry telemetry.

It supports:

* Distributed tracing
* Structured logging
* Metrics
* Trace-context propagation
* Configurable trace sampling
* OTLP/HTTP export
* Service and instance metadata
* Optional source-code metadata
* Graceful telemetry shutdown

The client is designed to be initialized **once during application startup** and shared throughout the application.

## Quick Start

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/7e3b/go-opentelemetry"
)

func main() {
	ctx := context.Background()

	client, err := (opentelemetry.Config{
		Name:            "orders-service",
		Namespace:       "commerce",
		Environment:     "production",
		Version:         "1.0.0",
		InstanceID:      os.Getenv("INSTANCE_ID"),
		Endpoint:        "localhost:4318",
		Insecure:        true,

		Tracer: opentelemetry.TracerConfig{
			SamplingRatio: 0.1,
		},

		Logger: opentelemetry.LoggerConfig{
			Severity: "info",
		},

		Meter: opentelemetry.MeterConfig{
			Enabled: true,
		},
	}).Client(ctx)

	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		if err := client.Shutdown(shutdownCtx); err != nil {
			log.Printf("OpenTelemetry shutdown failed: %v", err)
		}
	}()

	// Application...
}
```

## Configuration

`Config` controls service identity, OTLP export, and the individual telemetry providers.

```go
opentelemetry.Config{
	Name:            "orders-service",
	Namespace:       "commerce",
	Environment:     "production",
	Version:         "1.0.0",
	InstanceID:      os.Getenv("INSTANCE_ID"),
	Endpoint:        "localhost:4318",
	Insecure:        true,
	WithoutMetadata: false,

	Tracer: opentelemetry.TracerConfig{
		SamplingRatio: 0.1,
	},

	Logger: opentelemetry.LoggerConfig{
		Severity: "info",
	},

	Meter: opentelemetry.MeterConfig{
		Enabled: true,
	},
}
```

### Service Identity

| Field         | Description                                |
| ------------- | ------------------------------------------ |
| `Name`        | Service name                               |
| `Namespace`   | Logical service namespace                  |
| `Environment` | Deployment environment                     |
| `Version`     | Service version                            |
| `InstanceID`  | Identifier of the running service instance |

These values are exported as OpenTelemetry resource attributes.

### OTLP

| Field      | Description                        |
| ---------- | ---------------------------------- |
| `Endpoint` | OTLP HTTP endpoint                 |
| `Insecure` | Disables TLS for the OTLP endpoint |

A typical deployment uses an OpenTelemetry Collector:

```text
Application
    |
    | OTLP/HTTP
    v
OpenTelemetry Collector
    |
    +--> Traces
    +--> Logs
    +--> Metrics
```

For local development:

```go
Endpoint: "localhost:4318",
Insecure: true,
```

## Tracing

### Start a Span

Use `Start` when the calling function is an appropriate span name.

```go
func ProcessOrder(ctx context.Context, client opentelemetry.Client) error {
	span := client.Start(ctx)
	defer span.End()

	span.Info("processing order")

	if err := process(ctx); err != nil {
		span.Error(err)
		return err
	}

	return nil
}
```

### Explicit Span Names

Use `StartWithName` when you want a logical operation name.

```go
span := client.StartWithName(ctx, "order.checkout")
defer span.End()

span.Info("starting checkout")
```

Prefer stable, low-cardinality names:

```text
order.create
order.checkout
payment.process
user.authenticate
```

Avoid dynamic values in span names:

```text
order.create.12345
user.authenticate.827364
```

Use attributes for dynamic identifiers instead.

## Structured Logging

Logs emitted through a `Span` are correlated with its trace context.

```go
span.Info("order processed")
span.Debug("payment provider response received")
span.Warn("retrying payment request")
```

Errors should be recorded with `Error`:

```go
if err := processPayment(ctx); err != nil {
	span.Error(err)
	return err
}
```

`Error` records the error and marks the associated span as failed.

`Fatal` records a fatal error and marks the span as failed. It does **not** terminate the application.

```go
span.Fatal(err)
```

### Log Severity

Configure the minimum exported severity:

```go
Logger: opentelemetry.LoggerConfig{
	Severity: "info",
},
```

Supported values:

```text
trace
debug
info
warn
error
fatal
```

For example, `info` exports `info`, `warn`, `error`, and `fatal` records while filtering lower severities.

## Attributes

Additional attributes can be supplied to spans and log records.

```go
span.Info(
	"order processed",
	map[string]any{
		"order.id":    orderID,
		"customer.id": customerID,
		"items.count": len(items),
	},
)
```

Prefer stable attributes with bounded cardinality.

Dynamic identifiers such as order IDs are generally better represented as attribute values rather than span names.

## Error Handling

Use `Error` when an operation fails:

```go
result, err := repository.Get(ctx)
if err != nil {
	span.Error(
		err,
		map[string]any{
			"repository": "orders",
		},
	)
	return err
}
```

The error is recorded on the span and the span status is set to `Error`.

## Trace Sampling

Configure trace sampling using `SamplingRatio`:

```go
Tracer: opentelemetry.TracerConfig{
	SamplingRatio: 0.1,
},
```

Examples:

|  Ratio | Approximate sampling |
| -----: | -------------------: |
|  `1.0` |                 100% |
|  `0.5` |                  50% |
|  `0.1` |                  10% |
| `0.01` |                   1% |

A value greater than zero enables tracing.

The sampler is parent-based, so an existing sampling decision from an incoming trace is respected.

For local development, sampling everything is useful:

```go
SamplingRatio: 1.0,
```

For production, choose a sampling ratio appropriate for your traffic volume and observability requirements.

## Metrics

Enable metrics with:

```go
Meter: opentelemetry.MeterConfig{
	Enabled: true,
},
```

Metrics are exported through OTLP/HTTP.

## Trace Context

### New Timeout

`WithNewTimeout` creates a new timeout while preserving the OpenTelemetry trace context.

```go
ctx, cancel := client.WithNewTimeout(ctx, 5*time.Second)
defer cancel()

span := client.Start(ctx)
defer span.End()
```

This is useful when a downstream operation needs an independent timeout while remaining part of the current trace.

### Without Timeout

`WithoutTimeout` removes the original deadline and cancellation while preserving the OpenTelemetry trace context.

```go
backgroundCtx := client.WithoutTimeout(ctx)

go func() {
	span := client.Start(backgroundCtx)
	defer span.End()

	processBackgroundWork(backgroundCtx)
}()
```

This is useful when work needs to continue beyond the lifetime of the original request while remaining associated with the same trace.

## Context Propagation

Use `Propagator` when integrating with custom transports or libraries that do not automatically propagate OpenTelemetry context.

```go
propagator := client.Propagator()
```

Inject context into an outbound carrier:

```go
carrier := propagation.MapCarrier{}

propagator.Inject(ctx, carrier)
```

Extract context from an inbound carrier:

```go
ctx = propagator.Extract(ctx, carrier)
```

`Propagator()` returns `nil` when tracing is disabled.

## Source Metadata

Source metadata is enabled by default.

When enabled, telemetry includes:

* Source file
* Function name
* Line number

Disable it with:

```go
WithoutMetadata: true,
```

Disabling source metadata can reduce runtime overhead and telemetry volume.

## Graceful Shutdown

Always shut down the client during application termination:

```go
shutdownCtx, cancel := context.WithTimeout(
	context.Background(),
	10*time.Second,
)
defer cancel()

if err := client.Shutdown(shutdownCtx); err != nil {
	log.Printf("OpenTelemetry shutdown failed: %v", err)
}
```

`Shutdown` flushes pending telemetry and shuts down the configured OpenTelemetry providers.

This is particularly important when using batch exporters.

## Recommended Usage

### Initialize Once

Create one client per application process:

```go
client, err := config.Client(ctx)
```

Do not create a new client for every request or function.

### Start Spans Around Meaningful Operations

Good span boundaries include:

```text
HTTP request
Database operation
External API call
Message processing
Workflow/activity execution
Business operation
```

Avoid creating spans for every small internal function unless the additional visibility is useful.

### Keep Span Names Stable

Prefer:

```text
order.create
order.checkout
payment.authorize
user.authenticate
```

Avoid:

```text
order.create.12345
payment.authorize.user-92831
```

Use attributes for dynamic values.

### Use Logs for Events

Use spans to represent operations and logs/events to describe what happened during those operations.

```go
span := client.StartWithName(ctx, "payment.process")
defer span.End()

span.Info(
	"payment provider selected",
	map[string]any{
		"provider": "stripe",
	},
)

if err := charge(ctx); err != nil {
	span.Error(err)
	return err
}
```

## API

### `Config`

| API        | Purpose                                          |
| ---------- | ------------------------------------------------ |
| `Client()` | Creates and initializes the OpenTelemetry client |

### `Client`

| API                | Purpose                                                      |
| ------------------ | ------------------------------------------------------------ |
| `Start()`          | Starts a span using the calling function                     |
| `StartWithName()`  | Starts a span with an explicit name                          |
| `Shutdown()`       | Flushes and shuts down telemetry                             |
| `Propagator()`     | Returns the trace-context propagator                         |
| `WithNewTimeout()` | Creates a timeout context while preserving trace context     |
| `WithoutTimeout()` | Removes deadline/cancellation while preserving trace context |
| `TraceProvider()`  | Returns the underlying OpenTelemetry tracer provider         |

### `Span`

| API       | Purpose                                            |
| --------- | -------------------------------------------------- |
| `End()`   | Completes the span                                 |
| `Trace()` | Records a trace-level event                        |
| `Debug()` | Records a debug-level event                        |
| `Info()`  | Records an informational event                     |
| `Warn()`  | Records a warning event                            |
| `Error()` | Records an error and marks the span as failed      |
| `Fatal()` | Records a fatal error and marks the span as failed |

## Example: HTTP Handler

```go
func (handler *Handler) CreateOrder(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx := r.Context()

	span := handler.telemetry.StartWithName(ctx, "order.create")
	defer span.End()

	span.Info("creating order")

	order, err := handler.service.CreateOrder(ctx)
	if err != nil {
		span.Error(err)
		http.Error(w, "failed to create order", http.StatusInternalServerError)
		return
	}

	span.Info(
		"order created",
		map[string]any{
			"order.id": order.ID,
		},
	)

	// Write response...
}
```

## Example: Background Work

```go
func ProcessMessage(
	ctx context.Context,
	client opentelemetry.Client,
) error {
	span := client.StartWithName(ctx, "message.process")
	defer span.End()

	span.Info("processing message")

	if err := processMessage(ctx); err != nil {
		span.Error(err)
		return err
	}

	span.Info("message processed")

	return nil
}
```

## Requirements

* Go 1.XX or later
* An OpenTelemetry-compatible collector or backend

## License

See [LICENSE](LICENSE).
