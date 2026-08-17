# go-otx

`github.com/7e3b/go-otx` is a lightweight Go wrapper around
[OpenTelemetry](https://opentelemetry.io/) that provides a unified API
for:

-   Distributed tracing
-   Structured logging
-   Metrics
-   Trace-context propagation
-   Trace/log correlation
-   Optional source-code metadata
-   JSON console logging

The package is designed around a single application-wide telemetry
client configured once during startup and reused throughout the
application.

## Installation

``` bash
go get github.com/7e3b/go-otx
```

## Quick Start

``` go
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
        Namespace:   "my-company",
        Environment: "production",
        Version:     "1.0.0",
        Endpoint:    "localhost:4318",
        Insecure:    true,

        Tracer: otx.TracerConfig{
            SamplingRatio: 1.0,
        },

        Logger: otx.LoggerConfig{
            Severity: otx.SeverityInfo,
            Console:  true,
        },

        Meter: otx.MeterConfig{
            Enabled: true,
        },
    }.Connect(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer otx.Shutdown(context.Background())

    span := otx.Start(ctx)
    defer span.End()

    span.Info("request started", map[string]any{
        "user_id": 123,
    })
}
```

## Configuration

`Config` controls service identity and which telemetry providers are
enabled.

  -----------------------------------------------------------------------
  Field                               Description
  ----------------------------------- -----------------------------------
  `Name`                              Service name used as
                                      `service.name`.

  `Namespace`                         Service namespace used as
                                      `service.namespace`.

  `Environment`                       Deployment environment, such as
                                      `development` or `production`.

  `Version`                           Service version used as
                                      `service.version`.

  `InstanceID`                        Identifier for the running service
                                      instance.

  `Endpoint`                          OTLP HTTP endpoint used by the
                                      exporters.

  `Insecure`                          Disables TLS for the OTLP HTTP
                                      exporters.

  `WithoutMetadata`                   Disables automatic
                                      file/function/line metadata.

  `Tracer`                            Distributed tracing configuration.

  `Meter`                             Metrics configuration.

  `Logger`                            Structured logging configuration.
  -----------------------------------------------------------------------

### Tracing

Tracing is enabled when `Tracer.SamplingRatio` is greater than zero.

``` go
Tracer: otx.TracerConfig{
    SamplingRatio: 1.0,
},
```

Common sampling ratios:

-   `1.0` --- sample all traces
-   `0.1` --- sample approximately 10% of traces
-   `0` --- disable tracing

The tracer uses parent-based sampling with a trace-ID ratio sampler.

### Logging

Logging is enabled when `Logger.Severity` is not empty.

Supported severities:

``` text
trace
debug
info
warn
error
fatal
```

Example:

``` go
Logger: otx.LoggerConfig{
    Severity: otx.SeverityInfo,
    Console:  true,
},
```

The configured severity is the minimum severity exported.

For example, with `SeverityError`, `Error` and `Fatal` records are
exported, while `Info`, `Debug`, `Trace`, and `Warn` records are
filtered.

`Console: true` additionally writes structured JSON logs to standard
output using Go's `log/slog`.

### Metrics

Metrics are enabled with:

``` go
Meter: otx.MeterConfig{
    Enabled: true,
},
```

The metrics provider uses an OTLP HTTP exporter and a periodic reader.

## Spans

Start a span with:

``` go
span := otx.Start(ctx)
defer span.End()
```

The returned span provides both tracing and structured logging APIs.

### Span Configuration

``` go
span := otx.Start(ctx, otx.SpanConfig{
    Name: "process-order",
    Kind: otx.KindServer,
    Attributes: map[string]any{
        "order_id": 123,
    },
})
defer span.End()
```

`SpanConfig` supports:

-   `Name` --- explicit span name.
-   `Kind` --- OpenTelemetry span kind.
-   `Attributes` --- attributes attached to the span.

Supported span kinds:

``` go
otx.KindInternal
otx.KindServer
otx.KindClient
otx.KindProducer
otx.KindConsumer
```

If `Name` is empty and metadata collection is enabled, the calling
function is used as the span name.

## Span Logging

A span can emit structured logs:

``` go
span.Trace("trace message")
span.Debug("debug message")
span.Info("user authenticated")
span.Warn("slow downstream service")
span.Error(err)
span.Fatal(err)
```

Additional attributes can be supplied as maps:

``` go
span.Info("order processed", map[string]any{
    "order_id": 123,
    "amount":   500,
})
```

The same event is correlated with the span's trace context when
OpenTelemetry logging is enabled.

### Errors

`Error` and `Fatal` require a non-nil error.

``` go
if err != nil {
    span.Error(err, map[string]any{
        "operation": "create_order",
    })
}
```

When an error is recorded:

-   The log record contains the error.
-   The span records the error.
-   The span status is set to `Error`.

`Fatal` records fatal telemetry but does **not** terminate the process.

## Source Metadata

Unless `WithoutMetadata` is enabled, telemetry generated through spans
includes:

-   `file`
-   `function`
-   `line`

Disable it with:

``` go
WithoutMetadata: true,
```

Disabling metadata reduces runtime inspection overhead and telemetry
volume.

## Context

Use `Span.Ctx()` when subsequent operations need the span's context:

``` go
span := otx.Start(ctx)
defer span.End()

ctx = span.Ctx()
```

The returned context contains the active OpenTelemetry span when tracing
is enabled.

## Trace Context Propagation

`Inject` and `Extract` provide propagation for HTTP requests, queues,
NATS messages, jobs, and other transports.

### Producer

``` go
carrier := map[string]string{}

otx.Inject(span.Ctx(), carrier)

// Send carrier with the message.
```

### Consumer

``` go
ctx := otx.Extract(context.Background(), carrier)

span := otx.Start(ctx)
defer span.End()
```

The extracted context allows the consumer span to continue the
distributed trace.

The package configures W3C Trace Context and W3C Baggage propagation.

## Linking Asynchronous Work

For asynchronous work such as queues, the consumer operation can be
represented as a linked span rather than a child span.

Producer:

``` go
carrier := map[string]string{}
otx.Inject(span.Ctx(), carrier)
```

Consumer:

``` go
ctx := otx.Extract(context.Background(), carrier)

span := otx.Link(ctx, otx.SpanConfig{
    Name: "process-message",
    Kind: otx.KindConsumer,
})
defer span.End()
```

`Link` creates a new root span and adds the extracted span context as a
span link.

This is useful when the producer and consumer do not have a meaningful
synchronous parent-child relationship.

## Context Without the Original Deadline

`WithoutTimeout` preserves the OpenTelemetry trace context while
removing the original context's deadline and cancellation.

``` go
ctx := otx.WithoutTimeout(requestCtx)

go process(ctx)
```

This is useful when work must continue after the original request has
completed.

`WithNewTimeout` provides the same behavior while applying a new
timeout:

``` go
ctx, cancel := otx.WithNewTimeout(requestCtx, 30*time.Second)
defer cancel()
```

The resulting context keeps the trace context but does not inherit the
original deadline or cancellation.

## Graceful Shutdown

Call `Shutdown` during application shutdown:

``` go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := otx.Shutdown(ctx); err != nil {
    log.Printf("telemetry shutdown failed: %v", err)
}
```

Shutdown flushes pending telemetry and shuts down the enabled tracing,
logging, and metrics providers.

## Package-Level API

The package exposes the following primary operations:

  -----------------------------------------------------------------------
  Function                            Purpose
  ----------------------------------- -----------------------------------
  `Connect`                           Initializes the global telemetry
                                      client.

  `Start`                             Starts a normal span.

  `Link`                              Starts a new root span linked to
                                      another span context.

  `Shutdown`                          Flushes and shuts down telemetry
                                      providers.

  `Inject`                            Injects trace context into a
                                      carrier.

  `Extract`                           Extracts trace context from a
                                      carrier.

  `WithNewTimeout`                    Creates a new timeout context while
                                      preserving trace context.

  `WithoutTimeout`                    Removes deadline/cancellation while
                                      preserving trace context.
  -----------------------------------------------------------------------

## Exported Types

### `Span`

Represents a unit of work within a distributed trace and provides
structured logging methods.

Methods:

-   `End()`
-   `Trace(...)`
-   `Info(...)`
-   `Debug(...)`
-   `Warn(...)`
-   `Error(...)`
-   `Fatal(...)`
-   `Ctx()`

### `SpanConfig`

Configures a span's name, kind, and attributes.

### `Config`

Configures the telemetry client and service resource information.

### `TracerConfig`

Configures trace sampling.

### `MeterConfig`

Enables or disables metrics.

### `LoggerConfig`

Configures minimum logging severity and console output.

## OpenTelemetry Resource Attributes

The configured service identity is exported as OpenTelemetry resource
attributes:

-   `service.name`
-   `service.namespace`
-   `service.version`
-   `service.instance.id`
-   `deployment.environment.name`
-   `host.name`

## Architecture

The package maintains one global `client`:

``` text
                    ┌─────────────────┐
                    │    otx.Config   │
                    └────────┬────────┘
                             │ Connect
                             ▼
                    ┌─────────────────┐
                    │     client      │
                    ├─────────────────┤
                    │ tracer          │
                    │ logger          │
                    │ meter           │
                    │ metadata        │
                    └───────┬─────────┘
                            │
          ┌─────────────────┼─────────────────┐
          ▼                 ▼                 ▼
       Tracing           Logging           Metrics
          │                 │                 │
          └─────────────────┼─────────────────┘
                            ▼
                     OTLP HTTP Collector
```

The global client is protected by a read/write mutex and is intended to
be initialized once during application startup.

## Requirements

-   Go with a version compatible with the OpenTelemetry dependencies
    used by this module.
-   An OTLP HTTP-compatible collector or backend.

The package can be used with observability backends such as SigNoz,
provided they accept OTLP over HTTP.

## License

See [LICENSE](LICENSE).
