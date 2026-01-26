package semconvtest

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const DefaultWeaverPort = 4317

var (
	ErrOptionValidation = errors.New("weaver options failed validation")
	ErrPortOutOfRange   = errors.New("port number out of valid range (1-65535)")
)

type WeaverContext struct {
	ctx             context.Context
	weaverContainer *testcontainers.DockerContainer
	opts            *WeaverOptions
	exporters       *otlpExporterContext
}

// NewWeaverContext returns a weaver context customized with the provided options.
func NewWeaverContext(ctx context.Context, opts *WeaverOptions) (*WeaverContext, error) {
	err := opts.validate()
	if err != nil {
		return nil, err
	}

	containerOpts := opts.testContainerOptions()

	weaverVersion := "latest"
	if opts.Version != "" {
		weaverVersion = opts.Version
	}

	weaverC, err := testcontainers.Run(
		ctx,
		fmt.Sprintf("otel/weaver:%s", weaverVersion),
		containerOpts...,
	)
	if err != nil {
		return nil, err
	}

	exporters, err := newOTLPExporterContext(ctx, opts.endpoint())
	if err != nil {
		return nil, err
	}
	err = exporters.start()
	if err != nil {
		return nil, err
	}

	return &WeaverContext{
		ctx:             ctx,
		weaverContainer: weaverC,
		opts:            opts,
		exporters:       exporters,
	}, nil
}

func (wc *WeaverContext) Shutdown() error {
	var errs []error
	err := wc.exporters.shutdown()
	errs = append(errs, err)
	var timeout time.Duration = 10 * time.Second
	err = wc.weaverContainer.Stop(wc.ctx, &timeout)
	errs = append(errs, err)
	return errors.Join(errs...)
}

func (wc *WeaverContext) ContainerLogs() ([]string, error) {
	reader, err := wc.weaverContainer.Logs(wc.ctx)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	logs := []string{}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		logs = append(logs, scanner.Text())
	}
	return logs, nil
}

func (wc *WeaverContext) TestLogs(logs plog.Logs) error {
	// TODO: temp
	return wc.exporters.consumeLogs(logs)
}

type WeaverOptions struct {
	Version  string
	Address  string
	Port     int
	Registry string
}

func NewDefaultWeaverOptions() *WeaverOptions {
	return &WeaverOptions{}
}

// validate will validate the provided WeaverOptions.
// This will be called before the testcontainer options are
// constructed, meaning the construction code can assume
// valid options.
func (opts *WeaverOptions) validate() error {
	errs := []error{}
	// The port being 0 means unset because Go is like that. :)
	// Luckily, we don't want to allow 0 as a valid option anyway,
	// since that functionally makes the networking stack pick a
	// random port for the request, which would be useless in this
	// scenario. So it will be ignored as if unset (even if set manually).
	//
	// Otherwise, validate that the port is within the valid port range.
	if opts.Port != 0 && (opts.Port < 0 || 65535 < opts.Port) {
		errs = append(errs, fmt.Errorf("%w: %d", ErrPortOutOfRange, opts.Port))
	}

	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("%w: %w", ErrOptionValidation, err)
	}

	return nil
}

// testContainerOptions will translate the provided
// WeaverOptions to testcontainer options.
func (opts *WeaverOptions) testContainerOptions() []testcontainers.ContainerCustomizer {
	containerOpts := []testcontainers.ContainerCustomizer{}

	// Get the command args based on the options.
	cmdArgs := opts.cmdArgs()
	if len(cmdArgs) > 0 {
		containerOpts = append(containerOpts, testcontainers.WithCmdArgs(cmdArgs...))
	}

	// Expose the required port on the container.
	exposePort := DefaultWeaverPort
	if opts.Port != 0 {
		exposePort = opts.Port
	}
	containerOpts = append(containerOpts, testcontainers.WithExposedPorts(fmt.Sprintf("%d/tcp", exposePort)))

	return containerOpts
}

// cmdArgs constructs the weaver command args to provide to the container.
// The weaver command we want to run is `weaver registry live-check`.
// Usage docs: https://github.com/open-telemetry/weaver/blob/main/docs/usage.md#registry-live-check
func (opts *WeaverOptions) cmdArgs() []string {
	args := []string{"registry", "live-check"}
	if opts.Port != 0 {
		args = append(args, "--otlp-grpc-port", fmt.Sprintf("%d", opts.Port))
	}
	if opts.Address != "" {
		args = append(args, "--otlp-grpc-address", opts.Address)
	}
	if opts.Registry != "" {
		args = append(args, "--registry", opts.Registry)
	}
	args = append(args, "--diagnostic-format", "json")
	return args
}

// endpoint will construct an endpoint usable by exporter settings
// using the default values or provided options.
func (opts *WeaverOptions) endpoint() string {
	address := "0.0.0.0"
	if opts.Address != "" {
		address = opts.Address
	}
	port := DefaultWeaverPort
	if opts.Port != 0 {
		port = opts.Port
	}
	return fmt.Sprintf("%s:%d", address, port)
}

type otlpExporterContext struct {
	ctx     context.Context
	traces  exporter.Traces
	metrics exporter.Metrics
	logs    exporter.Logs
}

func newOTLPExporterContext(ctx context.Context, endpoint string) (*otlpExporterContext, error) {
	factory := otlpexporter.NewFactory()

	config := factory.CreateDefaultConfig()
	exporterConfig := config.(*otlpexporter.Config)
	exporterConfig.ClientConfig = configgrpc.ClientConfig{
		Endpoint: endpoint,
	}
	set := exporter.Settings{
		ID:                component.NewID(component.MustNewType("otlp_grpc")),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}

	var errs []error
	var err error
	exporterCtx := otlpExporterContext{ctx: ctx}
	exporterCtx.logs, err = factory.CreateLogs(ctx, set, config)
	errs = append(errs, err)
	exporterCtx.metrics, err = factory.CreateMetrics(ctx, set, config)
	errs = append(errs, err)
	exporterCtx.traces, err = factory.CreateTraces(ctx, set, config)
	errs = append(errs, err)
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return &exporterCtx, nil
}

func (exporterCtx *otlpExporterContext) start() error {
	nopHost := componenttest.NewNopHost()
	var errs []error
	err := exporterCtx.logs.Start(exporterCtx.ctx, nopHost)
	errs = append(errs, err)
	err = exporterCtx.metrics.Start(exporterCtx.ctx, nopHost)
	errs = append(errs, err)
	err = exporterCtx.traces.Start(exporterCtx.ctx, nopHost)
	errs = append(errs, err)
	return errors.Join(errs...)
}

func (exporterCtx *otlpExporterContext) shutdown() error {
	var errs []error
	err := exporterCtx.logs.Shutdown(exporterCtx.ctx)
	errs = append(errs, err)
	err = exporterCtx.metrics.Shutdown(exporterCtx.ctx)
	errs = append(errs, err)
	err = exporterCtx.traces.Shutdown(exporterCtx.ctx)
	errs = append(errs, err)
	return errors.Join(errs...)
}

func (exporterCtx *otlpExporterContext) consumeLogs(logs plog.Logs) error {
	return exporterCtx.logs.ConsumeLogs(exporterCtx.ctx, logs)
}

func (exporterCtx *otlpExporterContext) consumeMetrics(metrics pmetric.Metrics) error {
	return exporterCtx.metrics.ConsumeMetrics(exporterCtx.ctx, metrics)
}

func (exporterCtx *otlpExporterContext) consumeTraces(traces ptrace.Traces) error {
	return exporterCtx.traces.ConsumeTraces(exporterCtx.ctx, traces)
}
