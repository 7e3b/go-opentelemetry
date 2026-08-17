package otx

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdkLog "go.opentelemetry.io/otel/sdk/log"
	sdkMetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkTrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// ErrInvalidSeverity indicates that LoggerConfig.Severity contains an
// unsupported severity value.
var ErrInvalidSeverity = errors.New("invalid severity")

const (
	// SeverityTrace represents trace-level logging.
	SeverityTrace = "trace"

	// SeverityDebug represents debug-level logging.
	SeverityDebug = "debug"

	// SeverityInfo represents informational logging.
	SeverityInfo = "info"

	// SeverityWarn represents warning-level logging.
	SeverityWarn = "warn"

	// SeverityError represents error-level logging.
	SeverityError = "error"

	// SeverityFatal represents fatal-level logging.
	//
	// Fatal severity records a critical error but does not terminate the
	// application.
	SeverityFatal = "fatal"
)

// Config configures the OpenTelemetry client.
//
// A Config controls service identity, OTLP export settings, and the
// individual tracing, logging, and metrics providers.
//
// Create the Client once during application startup and reuse it throughout
// the application.
type Config struct {
	// Name identifies the service emitting telemetry.
	//
	// This value is used as the OpenTelemetry service.name resource attribute.
	Name string `json:"name"`

	// Namespace identifies the logical namespace of the service.
	Namespace string `json:"namespace"`

	// Environment identifies the deployment environment, such as
	// "development", "staging", or "production".
	Environment string `json:"environment"`

	// Version identifies the version of the running service.
	Version string `json:"version"`

	// InstanceID identifies the specific running instance of the service.
	//
	// In a containerized environment this can be set to a pod, task, or
	// other instance identifier.
	InstanceID string `json:"instance_id"`

	// Endpoint specifies the OTLP HTTP endpoint used to export telemetry.
	//
	// The configured endpoint is used by the enabled tracing, logging,
	// and metrics exporters.
	Endpoint string `json:"endpoint"`

	// Insecure disables TLS when communicating with the OTLP endpoint.
	//
	// This is generally useful for local development or environments where
	// the collector endpoint does not require TLS.
	Insecure bool `json:"insecure"`

	// WithoutMetadata disables automatic source-code metadata.
	//
	// When metadata is enabled, telemetry includes the source file,
	// function, and line number associated with the operation.
	//
	// Disabling metadata can reduce runtime overhead and telemetry volume.
	WithoutMetadata bool `json:"without_metadata"`

	// Tracer configures distributed tracing.
	Tracer TracerConfig `json:"tracer"`

	// Meter configures metrics collection.
	Meter MeterConfig `json:"meter"`

	// Logger configures structured logging.
	Logger LoggerConfig `json:"logger"`
}

// TracerConfig configures distributed tracing.
type TracerConfig struct {
	// SamplingRatio specifies the proportion of traces to sample.
	//
	// For example:
	//
	//	1.0  - sample all traces
	//	0.1  - sample approximately 10% of traces
	//
	// A value greater than zero enables tracing.
	SamplingRatio float64 `json:"sampling_ratio"`
}

// MeterConfig configures metrics collection.
type MeterConfig struct {
	// Enabled determines whether metrics collection is enabled.
	Enabled bool `json:"enabled"`
}

// LoggerConfig configures structured logging.
type LoggerConfig struct {
	// Severity specifies the minimum severity exported by the logger.
	//
	// Supported values are:
	//
	//	trace
	//	debug
	//	info
	//	warn
	//	error
	//	fatal
	//
	// An empty value disables logging.
	Severity string `json:"severity"`

	// Console determines whether log records are also written as structured
	// JSON to the process's standard output.
	Console bool `json:"console"`
}

// Client initializes the OpenTelemetry client using the supplied
// configuration.
//
// Only the telemetry providers enabled by the configuration are initialized.
//
// The returned Client should be shared across the application rather than
// creating a new client for each request or operation.
//
// Call Client.Shutdown during application shutdown to flush pending
// telemetry.
func (config Config) Client(ctx context.Context) (Client, error) {
	resource, err := config.resource()
	if err != nil {
		return nil, err
	}
	client := &client{
		metadata: !config.WithoutMetadata,
	}
	if config.Tracer.SamplingRatio > 0 {
		client.tracer, err = config.tracer(ctx, resource)
		if err != nil {
			return nil, err
		}
	}
	if config.Logger.Severity != "" {
		client.logger, err = config.logger(ctx, resource)
		if err != nil {
			return nil, err
		}
	}
	if config.Meter.Enabled {
		client.meter, err = config.meter(ctx, resource)
		if err != nil {
			return nil, err
		}
	}
	return client, nil
}

// Connect initializes and configures the package-level OpenTelemetry client.
//
// Connect must be called successfully before using the package-level
// functions Start, Shutdown, WithNewTimeout, or WithoutTimeout.
//
// Only the telemetry providers enabled by the configuration are initialized.
//
// Connect is intended to be called during application startup.
//
// Call Shutdown during graceful application shutdown to flush pending
// telemetry and shut down the configured providers.
func (config Config) Connect(ctx context.Context) error {
	resource, err := config.resource()
	if err != nil {
		return err
	}
	client := &client{
		metadata: !config.WithoutMetadata,
	}
	if config.Tracer.SamplingRatio > 0 {
		client.tracer, err = config.tracer(ctx, resource)
		if err != nil {
			return err
		}
	}
	if config.Logger.Severity != "" {
		client.logger, err = config.logger(ctx, resource)
		if err != nil {
			return err
		}
	}
	if config.Meter.Enabled {
		client.meter, err = config.meter(ctx, resource)
		if err != nil {
			return err
		}
	}
	mu.Lock()
	global = client
	mu.Unlock()
	return nil
}

func (config Config) resource() (*resource.Resource, error) {
	hostName, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	base := resource.Default()
	resource, err := resource.Merge(
		base,
		resource.NewWithAttributes(
			base.SchemaURL(),
			semconv.ServiceName(config.Name),
			semconv.ServiceNamespace(config.Namespace),
			semconv.ServiceVersion(config.Version),
			semconv.ServiceInstanceID(config.InstanceID),
			semconv.DeploymentEnvironmentName(config.Environment),
			semconv.HostName(hostName),
		),
	)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

func parseSeverity(severity string) log.Severity {
	switch severity {
	case SeverityTrace:
		return log.SeverityTrace
	case SeverityDebug:
		return log.SeverityDebug
	case SeverityInfo:
		return log.SeverityInfo
	case SeverityWarn:
		return log.SeverityWarn
	case SeverityError:
		return log.SeverityError
	case SeverityFatal:
		return log.SeverityFatal
	default:
		return log.SeverityUndefined
	}
}

func (config Config) logger(ctx context.Context, resource *resource.Resource) (*logger, error) {
	severity := parseSeverity(config.Logger.Severity)
	if severity == log.SeverityUndefined {
		return nil, ErrInvalidSeverity
	}
	opts := []otlploghttp.Option{
		otlploghttp.WithEndpoint(config.Endpoint),
		otlploghttp.WithCompression(otlploghttp.GzipCompression),
	}
	if config.Insecure {
		opts = append(opts, otlploghttp.WithInsecure())
	}
	exporter, err := otlploghttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}
	processor := sdkLog.NewBatchProcessor(exporter)
	providerOpts := []sdkLog.LoggerProviderOption{
		sdkLog.WithResource(resource),
		sdkLog.WithProcessor(processor),
	}
	logger := &logger{}
	logger.severity = severity
	logger.provider = sdkLog.NewLoggerProvider(providerOpts...)
	logger.logger = logger.provider.Logger(config.Name)
	if config.Logger.Console {
		logger.console = slog.New(
			slog.NewJSONHandler(
				os.Stdout,
				&slog.HandlerOptions{
					Level: slog.Level(severity),
				},
			),
		)
	}
	return logger, nil
}

func (config Config) meter(ctx context.Context, resource *resource.Resource) (*meter, error) {
	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(config.Endpoint),
		otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression),
	}
	if config.Insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}
	exporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}
	reader := sdkMetric.NewPeriodicReader(exporter)
	providerOpts := []sdkMetric.Option{
		sdkMetric.WithReader(reader),
		sdkMetric.WithResource(resource),
	}
	meter := &meter{}
	meter.provider = sdkMetric.NewMeterProvider(providerOpts...)
	meter.meter = meter.provider.Meter(config.Name)
	return meter, nil
}

func (config Config) tracer(ctx context.Context, resource *resource.Resource) (*tracer, error) {
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(config.Endpoint),
		otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
	}
	if config.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}
	processor := sdkTrace.NewBatchSpanProcessor(exporter)
	sampler := sdkTrace.ParentBased(sdkTrace.TraceIDRatioBased(config.Tracer.SamplingRatio))
	providerOpts := []sdkTrace.TracerProviderOption{
		sdkTrace.WithBatcher(exporter),
		sdkTrace.WithResource(resource),
		sdkTrace.WithSampler(sampler),
		sdkTrace.WithSpanProcessor(processor),
	}
	tracer := &tracer{}
	tracer.provider = sdkTrace.NewTracerProvider(providerOpts...)
	tracer.tracer = tracer.provider.Tracer(config.Name)
	tracer.propagator = propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	return tracer, nil
}
