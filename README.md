# go-sig

A unified Go package for structured logging with OpenTelemetry integration, combining tracing, metrics, and logging.

## Installation

```bash
go get github.com/7e3b/go-sig
```

## Usage

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "os"
    "time"
    
    "github.com/7e3b/go-sig"
    "github.com/7e3b/go-log"
    "github.com/7e3b/go-trace"
    "github.com/7e3b/go-metric"
)

func setup() error {
    ctx := context.Background()
    
    // Setup trace
    traceConfig := trace.Config{
        Ok:          true,
        Name:        "my-service",
        Environment: "production",
        URL:         "http://localhost:4318/v1/traces",
        Sampling:    1.0,
    }
    tracer, err := trace.Setup(ctx, traceConfig)
    if err != nil {
        return err
    }
    
    // Setup metrics
    metricConfig := metric.Config{
        Ok:          true,
        Name:        "my-service",
        Environment: "production",
        URL:         "http://localhost:4318/v1/metrics",
    }
    meter, err := metric.Setup(ctx, metricConfig)
    if err != nil {
        return err
    }
    
    // Setup logging
    logConfig := log.Config{
        Ok:          true,
        Name:        "my-service",
        Environment: "production",
        URL:         "http://localhost:4318/v1/logs",
    }
    logger, err := log.Setup(ctx, logConfig)
    if err != nil {
        return err
    }
    
    sig.Setup(tracer, meter, logger)
    return nil
}

func businessLogic(ctx context.Context) {
    // Start a new log context
    log := sig.Start(ctx)
    defer log.End()
    
    // Log various events with attributes
    log.Info("Processing request", sig.Map{"user_id": 123})
    log.Debug("Cache hit", sig.Map{"key": "user:123", "ttl": 300})
    log.Warn("Rate limit approaching", sig.Map{"current": 95, "limit": 100})
    
    // Handle errors
    if err := someOperation(); err != nil {
        log.Error(err, sig.Map{"operation": "database_query"})
        return
    }
    
    log.Info("Request completed successfully")
}

func someOperation() error {
    return errors.New("database connection failed")
}

func main() {
    if err := setup(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    
    defer trace.Shutdown(5 * time.Second)
    defer metric.Shutdown(5 * time.Second)
    defer log.Shutdown(5 * time.Second)
    
    ctx := context.Background()
    businessLogic(ctx)
}
```

## Features

- **Unified Interface**: Single API for logging, tracing, and metrics
- **Automatic Function Names**: Captures function names for spans and logs
- **Structured Attributes**: Type-safe attribute handling across all signals
- **Context Propagation**: Maintains trace context throughout the call chain
- **Flexible Setup**: Optional components - use only what you need
- **OpenTelemetry Integration**: Full compatibility with OTLP exporters

## API

### Setup
```go
sig.Setup(tracer, meter, logger) // All parameters are optional (can be nil)
```

### Logging Interface
```go
log := sig.Start(ctx)    // Start new log context
defer log.End()          // End log context

log.Trace(msg, attrs...)  // Trace level
log.Info(msg, attrs...)   // Info level  
log.Debug(msg, attrs...)  // Debug level
log.Warn(msg, attrs...)   // Warning level
log.Error(err, attrs...)  // Error level
log.Fatal(err, attrs...)  // Fatal level

ctx := log.Ctx()         // Get context with trace info
```

### Attributes
Use `sig.Map` for structured attributes:
```go
log.Info("User login", sig.Map{
    "user_id": 123,
    "ip": "192.168.1.1", 
    "success": true,
})
```