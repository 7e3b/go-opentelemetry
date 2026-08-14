package opentelemetry

import (
	"context"
	"errors"
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

var ErrInvalidSeverity = errors.New("invalid severity")

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

type TracerConfig struct {
	SamplingRatio float64
}

type MeterConfig struct {
	Enabled bool
}

type LoggerConfig struct {
	Severity string
}

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
	case "trace":
		return log.SeverityTrace
	case "debug":
		return log.SeverityDebug
	case "info":
		return log.SeverityInfo
	case "warn":
		return log.SeverityWarn
	case "error":
		return log.SeverityError
	case "fatal":
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
	tracer.propagator = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	return tracer, nil
}
