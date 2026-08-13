package otel

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	olog "go.opentelemetry.io/otel/log"
	ometric "go.opentelemetry.io/otel/metric"
	otrace "go.opentelemetry.io/otel/trace"
)

// Global state to hold initialized OpenTelemetry components.
var global struct {
	ok struct {
		tracer bool
		meter  bool
		logger bool
	}
	tracer otrace.Tracer
	meter  ometric.Meter
	logger olog.Logger
}

// Map is a shorthand for key-value attribute collections.
type Map map[string]any

// log holds contextual information about the current function,
// including trace span, function name, file, and the original context.
type log struct {
	function string
	file     string
	ctx      context.Context
	span     otrace.Span
}

// Log interface exposes unified logging methods for different levels
// and lifecycle management.
type Log interface {
	Trace(string, ...Map)
	Info(string, ...Map)
	Debug(string, ...Map)
	Warn(string, ...Map)
	Error(error, ...Map)
	Fatal(error, ...Map)
	End()
	Ctx() context.Context
}

// Setup configures global OpenTelemetry tracer, meter, and logger.
func Setup(tracer otrace.Tracer, meter ometric.Meter, logger olog.Logger) {
	if tracer != nil {
		global.ok.tracer = true
		global.tracer = tracer
	}
	if meter != nil {
		global.ok.meter = true
		global.meter = meter
	}
	if logger != nil {
		global.ok.logger = true
		global.logger = logger
	}
}

// callerMeta retrieves the name, file, and line number of the caller.
// Used to annotate logs and spans with accurate source info.
func callerMeta() (string, string, int) {
	pc, file, line, ok := runtime.Caller(2)
	if !ok {
		return "", "", 0
	}
	return runtime.FuncForPC(pc).Name(), file, line
}

// callerLine returns only the line number from a given stack frame.
func callerLine(skip int) int {
	_, _, line, ok := runtime.Caller(skip)
	if !ok {
		return 0
	}
	return line
}

// Start begins a new trace/log session for a function, capturing metadata
// and emitting a "started" log record or span as appropriate.
func Start(ctx context.Context) Log {
	log := &log{ctx: ctx}
	if !global.ok.tracer && !global.ok.logger {
		return log
	}
	now := time.Now()
	var line int
	log.function, log.file, line = callerMeta()
	// Start tracing
	if global.ok.tracer {
		log.ctx, log.span = global.tracer.Start(
			ctx,
			log.function,
			otrace.WithTimestamp(now),
			otrace.WithAttributes(
				attribute.String("file", log.file),
				attribute.Int("line", line),
			),
		)
	}
	// Emit "started" log
	if global.ok.logger {
		record := olog.Record{}
		record.SetBody(olog.StringValue("started"))
		record.SetTimestamp(now)
		record.SetSeverity(olog.SeverityTrace)
		record.SetSeverityText(olog.SeverityTrace.String())
		record.AddAttributes(
			olog.String("function", log.function),
			olog.String("file", log.file),
			olog.Int("line", line),
		)
		global.logger.Emit(log.ctx, record)
	}
	return log
}

// Ctx returns the context associated with this log instance,
// potentially augmented with trace info.
func (log *log) Ctx() context.Context {
	return log.ctx
}

// End marks the end of a trace/log session,
// emitting an "ended" log record and closing the span.
func (log *log) End() {
	if !global.ok.tracer && !global.ok.logger {
		return
	}
	now := time.Now()
	line := callerLine(2)
	// Emit "ended" log
	if global.ok.logger {
		record := olog.Record{}
		record.SetBody(olog.StringValue("ended"))
		record.SetTimestamp(now)
		record.SetSeverity(olog.SeverityTrace)
		record.SetSeverityText(olog.SeverityTrace.String())
		record.AddAttributes(
			olog.String("function", log.function),
			olog.String("file", log.file),
			olog.Int("line", line),
		)
		global.logger.Emit(log.ctx, record)
	}
	// End the span
	if global.ok.tracer {
		log.span.End(otrace.WithTimestamp(now))
	}
}

// record creates and emits a log record and/or span event
// with the given severity level and attributes.
func (log *log) record(event string, level olog.Severity, attrsSlice ...Map) {
	now := time.Now()
	var otraceAttrs []attribute.KeyValue
	var ologAttrs []olog.KeyValue
	if global.ok.tracer {
		otraceAttrs = []attribute.KeyValue{}
	}
	if global.ok.logger {
		ologAttrs = []olog.KeyValue{}
	}
	// Convert input attributes to appropriate OT formats
	for _, attrs := range attrsSlice {
		for key, value := range attrs {
			if global.ok.tracer {
				otraceAttrs = append(otraceAttrs, attribute.String(key, fmt.Sprint(value)))
			}
			if global.ok.logger {
				ologAttrs = append(ologAttrs, olog.String(key, fmt.Sprint(value)))
			}
		}
	}
	line := callerLine(3)
	if global.ok.tracer {
		// Add file, line info to span
		otraceAttrs = append(
			otraceAttrs,
			attribute.String("file", log.file),
			attribute.Int("line", line),
		)
		options := []otrace.EventOption{
			otrace.WithTimestamp(now),
			otrace.WithAttributes(otraceAttrs...),
		}
		// Add event and optionally set error status
		if level >= olog.SeverityError {
			log.span.AddEvent(event, options...)
			log.span.SetStatus(codes.Error, event)
		} else {
			log.span.AddEvent(event, options...)
		}
	}
	// Emit log record
	if global.ok.logger {
		record := olog.Record{}
		record.SetBody(olog.StringValue(event))
		record.SetTimestamp(now)
		record.SetSeverity(level)
		record.SetSeverityText(level.String())
		// Add file, function, line info to span
		ologAttrs = append(
			ologAttrs,
			olog.String("function", log.function),
			olog.String("file", log.file),
			olog.Int("line", line),
		)
		record.AddAttributes(ologAttrs...)
		global.logger.Emit(log.ctx, record)
	}
}

// Trace logs a trace-level event.
func (log *log) Trace(event string, attributes ...Map) {
	if !global.ok.tracer && !global.ok.logger {
		return
	}
	if event == "" {
		return
	}
	log.record(event, olog.SeverityTrace, attributes...)
}

// Info logs an info-level event.
func (log *log) Info(event string, attributes ...Map) {
	if !global.ok.tracer && !global.ok.logger {
		return
	}
	if event == "" {
		return
	}
	log.record(event, olog.SeverityInfo, attributes...)
}

// Debug logs a debug-level event.
func (log *log) Debug(event string, attributes ...Map) {
	if !global.ok.tracer && !global.ok.logger {
		return
	}
	if event == "" {
		return
	}
	log.record(event, olog.SeverityDebug, attributes...)
}

// Warn logs a warning-level event.
func (log *log) Warn(event string, attributes ...Map) {
	if !global.ok.tracer && !global.ok.logger {
		return
	}
	if event == "" {
		return
	}
	log.record(event, olog.SeverityWarn, attributes...)
}

// Error logs an error-level event and marks the span as failed.
func (log *log) Error(err error, attributes ...Map) {
	if !global.ok.tracer && !global.ok.logger {
		return
	}
	if err == nil {
		return
	}
	log.record(err.Error(), olog.SeverityError, attributes...)
}

// Fatal logs a fatal-level event.
func (log *log) Fatal(err error, attributes ...Map) {
	if !global.ok.tracer && !global.ok.logger {
		return
	}
	if err == nil {
		return
	}
	log.record(err.Error(), olog.SeverityFatal, attributes...)
}
