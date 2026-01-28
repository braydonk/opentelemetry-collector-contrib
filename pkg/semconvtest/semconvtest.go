package semconvtest

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const DefaultWeaverOTLPListenerPort = "4317/tcp"

var (
	ErrOptionValidation = errors.New("weaver options failed validation")
)

type WeaverContext struct {
	ctx             context.Context
	weaverContainer *testcontainers.DockerContainer
	opts            *WeaverOptions
	exporters       *otlpExporterContext
	clients         *pdataClientContext
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

	// Upon creating the container, we can gather the host and the mapped port
	// it chose and use that to construct the pdata clients.
	host, err := weaverC.Host(ctx)
	if err != nil {
		return nil, err
	}
	mappedPort, err := weaverC.MappedPort(ctx, nat.Port(DefaultWeaverOTLPListenerPort))
	if err != nil {
		return nil, err
	}

	clients, err := newPdataClientContext(ctx, fmt.Sprintf("%s:%s", host, mappedPort.Port()))
	if err != nil {
		return nil, err
	}

	return &WeaverContext{
		ctx:             ctx,
		weaverContainer: weaverC,
		opts:            opts,
		clients:         clients,
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
	// return wc.exporters.consumeLogs(logs)
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	response, err := wc.clients.consumeLogs(ctx, logs)
	if err != nil {
		return err
	}
	resJson, err := response.MarshalJSON()
	if err != nil {
		return err
	}
	fmt.Println(string(resJson))
	return nil
}

type WeaverOptions struct {
	Version  string
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
	containerOpts = append(containerOpts, testcontainers.WithExposedPorts(DefaultWeaverOTLPListenerPort))

	return containerOpts
}

// cmdArgs constructs the weaver command args to provide to the container.
// The weaver command we want to run is `weaver registry live-check`.
// Usage docs: https://github.com/open-telemetry/weaver/blob/main/docs/usage.md#registry-live-check
func (opts *WeaverOptions) cmdArgs() []string {
	args := []string{"registry", "live-check"}
	if opts.Registry != "" {
		args = append(args, "--registry", opts.Registry)
	}
	args = append(args, "--format", "json")
	fmt.Println(args)
	return args
}

type pdataClientContext struct {
	ctx        context.Context
	clientConn *grpc.ClientConn
	logs       plogotlp.GRPCClient
	metrics    pmetricotlp.GRPCClient
	traces     ptraceotlp.GRPCClient
}

func newPdataClientContext(ctx context.Context, endpoint string) (*pdataClientContext, error) {
	clientConn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	clientContext := pdataClientContext{
		ctx:        ctx,
		clientConn: clientConn,
	}
	clientContext.logs = plogotlp.NewGRPCClient(clientContext.clientConn)
	clientContext.metrics = pmetricotlp.NewGRPCClient(clientContext.clientConn)
	clientContext.traces = ptraceotlp.NewGRPCClient(clientContext.clientConn)

	return &clientContext, nil
}

func (clientCtx *pdataClientContext) consumeLogs(ctx context.Context, logs plog.Logs) (plogotlp.ExportResponse, error) {
	plogReq := plogotlp.NewExportRequestFromLogs(logs)
	return clientCtx.logs.Export(ctx, plogReq)
}

func (clientCtx *pdataClientContext) consumeMetrics(ctx context.Context, metrics pmetric.Metrics) (pmetricotlp.ExportResponse, error) {
	pmetricReq := pmetricotlp.NewExportRequestFromMetrics(metrics)
	return clientCtx.metrics.Export(ctx, pmetricReq)
}

func (clientCtx *pdataClientContext) consumeTraces(ctx context.Context, traces ptrace.Traces) (ptraceotlp.ExportResponse, error) {
	ptraceReq := ptraceotlp.NewExportRequestFromTraces(traces)
	return clientCtx.traces.Export(ctx, ptraceReq)
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
