package opentelemetry

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

// Span represents a unit of work within a distributed trace.
//
// A Span also provides structured logging methods. Logs emitted through a
// Span are associated with the Span's trace context, allowing logs and traces
// to be correlated in an observability backend.
//
// A Span should normally be ended exactly once using defer immediately after
// it is created.
type Span interface {
	// End completes the span.
	//
	// End should normally be called using:
	//
	//	span := client.Start(ctx)
	//	defer span.End()
	End()

	// Trace records a trace-level diagnostic event.
	Trace(string, ...map[string]any)

	// Info records an informational event.
	Info(string, ...map[string]any)

	// Debug records a debug-level diagnostic event.
	Debug(string, ...map[string]any)

	// Warn records a warning event.
	Warn(string, ...map[string]any)

	// Error records an error event and marks the associated span as failed.
	//
	// A nil error is ignored.
	Error(error, ...map[string]any)

	// Fatal records a fatal error event and marks the associated span as failed.
	//
	// A nil error is ignored. This method records telemetry but does not
	// terminate the application.
	Fatal(error, ...map[string]any)
}

type span struct {
	client      *client
	packageName string
	name        string
	function    string
	file        string
	ctx         context.Context
	span        trace.Span
}

func callerLine(skip int) int {
	_, _, line, ok := runtime.Caller(skip)
	if !ok {
		return 0
	}
	return line
}

func meta() (string, string, int) {
	pc, file, line, ok := runtime.Caller(3)
	if !ok {
		return "", "", 0
	}
	return file, runtime.FuncForPC(pc).Name(), line
}

func (span *span) start(ctx context.Context, extras ...map[string]any) {
	now := time.Now()
	client := span.client
	attributes := []attribute.KeyValue{}
	for _, object := range extras {
		for key, value := range object {
			attributes = append(attributes, attribute.String(key, fmt.Sprint(value)))
		}
	}
	if client.metadata {
		var line int
		span.file, span.function, line = meta()
		attributes = append(
			attributes,
			attribute.String("file", span.file),
			attribute.String("function", span.function),
			attribute.Int("line", line),
		)
		span.name = span.function
	}
	tracer := client.tracer
	if tracer != nil {
		span.ctx, span.span = tracer.tracer.Start(
			ctx,
			span.name,
			trace.WithTimestamp(now),
			trace.WithAttributes(attributes...),
		)
	}
	logger := client.logger
	if logger != nil {
		record := log.Record{}
		record.SetBody(attribute.StringValue("started"))
		record.SetTimestamp(now)
		record.SetObservedTimestamp(now)
		record.SetSeverity(log.SeverityTrace)
		record.SetSeverityText(log.SeverityTrace.String())
		record.AddAttributes(attributes...)
		logger.logger.Emit(span.ctx, record)
	}
}

func (span *span) startWithName(ctx context.Context, name string, extras ...map[string]any) {
	now := time.Now()
	client := span.client
	span.name = name
	attributes := []attribute.KeyValue{}
	for _, object := range extras {
		for key, value := range object {
			attributes = append(attributes, attribute.String(key, fmt.Sprint(value)))
		}
	}
	if client.metadata {
		var line int
		span.file, span.function, line = meta()
		attributes = append(
			attributes,
			attribute.String("file", span.file),
			attribute.String("function", span.function),
			attribute.Int("line", line),
		)
	}
	tracer := client.tracer
	if tracer != nil {
		span.ctx, span.span = tracer.tracer.Start(
			ctx,
			span.name,
			trace.WithTimestamp(now),
			trace.WithAttributes(attributes...),
		)
	}
	logger := client.logger
	if logger != nil {
		record := log.Record{}
		record.SetBody(attribute.StringValue("started"))
		record.SetTimestamp(now)
		record.SetObservedTimestamp(now)
		record.SetSeverity(log.SeverityTrace)
		record.SetSeverityText(log.SeverityTrace.String())
		record.AddAttributes(attributes...)
		logger.logger.Emit(span.ctx, record)
	}
}

func (span *span) End() {
	client := span.client
	tracer := client.tracer
	logger := client.logger
	if tracer == nil && logger == nil {
		return
	}
	now := time.Now()
	if logger != nil {
		attributes := []attribute.KeyValue{}
		if span.client.metadata {
			line := callerLine(2)
			attributes = append(
				attributes,
				attribute.String("file", span.file),
				attribute.String("function", span.function),
				attribute.Int("line", line),
			)
		}
		record := log.Record{}
		record.SetBody(attribute.StringValue("ended"))
		record.SetTimestamp(now)
		record.SetObservedTimestamp(now)
		record.SetSeverity(log.SeverityTrace)
		record.SetSeverityText(log.SeverityTrace.String())
		record.AddAttributes(attributes...)
		logger.logger.Emit(span.ctx, record)
	}
	if tracer != nil {
		span.span.End(trace.WithTimestamp(now))
	}
}

func (span *span) record(event string, err error, severity log.Severity, extras ...map[string]any) {
	tracer := span.client.tracer
	logger := span.client.logger
	if tracer == nil && logger == nil {
		return
	}
	now := time.Now()
	attributes := []attribute.KeyValue{}
	for _, object := range extras {
		for key, value := range object {
			attributes = append(attributes, attribute.String(key, fmt.Sprint(value)))
		}
	}
	if span.client.metadata {
		line := callerLine(3)
		attributes = append(
			attributes,
			attribute.String("file", span.file),
			attribute.String("function", span.function),
			attribute.Int("line", line),
		)
	}
	if err != nil && severity < log.SeverityError {
		severity = log.SeverityError
	}
	if tracer != nil {
		opts := []trace.EventOption{
			trace.WithTimestamp(now),
			trace.WithAttributes(attributes...),
		}
		if err != nil {
			span.span.RecordError(err, opts...)
			span.span.SetStatus(codes.Error, "")
		} else {
			if severity >= log.SeverityError {
				span.span.AddEvent(event, opts...)
				span.span.SetStatus(codes.Error, "")
			} else {
				span.span.AddEvent(event, opts...)
			}
		}
	}
	if logger != nil && severity >= logger.severity {
		record := log.Record{}
		if err != nil {
			record.SetErr(err)
		} else {
			record.SetBody(attribute.StringValue(event))
		}
		record.SetSeverity(severity)
		record.SetSeverityText(severity.String())
		record.SetTimestamp(now)
		record.SetObservedTimestamp(now)
		record.AddAttributes(attributes...)
		logger.logger.Emit(span.ctx, record)
	}
}

func (span *span) Trace(event string, attributes ...map[string]any) {
	if event == "" {
		return
	}
	span.record(event, nil, log.SeverityTrace, attributes...)
}

func (span *span) Info(event string, attributes ...map[string]any) {
	if event == "" {
		return
	}
	span.record(event, nil, log.SeverityInfo, attributes...)
}

func (span *span) Debug(event string, attributes ...map[string]any) {
	if event == "" {
		return
	}
	span.record(event, nil, log.SeverityDebug, attributes...)
}

func (span *span) Warn(event string, attributes ...map[string]any) {
	if event == "" {
		return
	}
	span.record(event, nil, log.SeverityWarn, attributes...)
}

func (span *span) Error(err error, attributes ...map[string]any) {
	if err == nil {
		return
	}
	span.record("", err, log.SeverityError, attributes...)
}

func (span *span) Fatal(err error, attributes ...map[string]any) {
	if err == nil {
		return
	}
	span.record("", err, log.SeverityFatal, attributes...)
}
