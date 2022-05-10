package googlecloudloggingexporter

import (
	"context"
	"fmt"

	"cloud.google.com/go/logging"
	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

var httpRequestKey = "http://www.googleapis.com/logging/httpRequest"

type exporter struct {
	config             *Config
	logger             *zap.Logger
	collectorID        string
	cloudLoggingClient *logging.Client
	cloudLogger        *logging.Logger
}

func newCloudLoggingExporter(config *Config, params component.ExporterCreateSettings) (component.LogsExporter, error) {
	loggingExporter, err := newCloudLoggingLogExporter(config, params)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogsExporter(
		config,
		params,
		loggingExporter.ConsumeLogs,
		exporterhelper.WithQueue(config.QueueSettings),
		exporterhelper.WithRetry(config.RetrySettings))
}

func newCloudLoggingLogExporter(config *Config, params component.ExporterCreateSettings) (component.LogsExporter, error) {
	// Validate the passed config.
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Generate a Collector ID.
	collectorIdentifier, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	// Read project ID from Metadata if not specified by config.
	if config.ProjectID == "" {
		projectId, err := readProjectIdMetadata()
		if err != nil {
			return nil, fmt.Errorf("failed to read Google Cloud project ID: %v", err)
		}
		config.ProjectID = projectId
	}

	// Create Cloud Logging logger with project ID.
	client, err := logging.NewClient(context.Background(), config.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google Cloud Logging client: %v", err)
	}
	logger := client.Logger(config.LogName)

	// Create the logging exporter.
	loggingExporter := &exporter{
		config:             config,
		logger:             params.Logger,
		collectorID:        collectorIdentifier.String(),
		cloudLoggingClient: client,
		cloudLogger:        logger,
	}
	return loggingExporter, nil
}

func (e *exporter) ConsumeLogs(ctx context.Context, ld pdata.Logs) error {
	logEntries, dropped := logsToEntries(e.logger, ld)
	if len(logEntries) == 0 {
		return nil
	}
	if dropped > 0 {
		e.logger.Debug("Dropped logs", zap.Any("logsDropped", dropped))
	}

	for _, logEntry := range logEntries {
		e.logger.Debug("Adding log entry", zap.Any("entry", logEntry))
		e.cloudLogger.Log(logEntry)
	}
	e.logger.Debug("Log entries successfully buffered")
	err := e.cloudLogger.Flush()
	if err != nil {
		e.logger.Error("error force flushing logs. Skipping to next logPusher.", zap.Error(err))
	}
	return nil
}

func (e *exporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *exporter) Shutdown(ctx context.Context) error {
	// Flush the remaining logs before shutting down the exporter.
	if e.cloudLogger != nil {
		err := e.cloudLogger.Flush()
		if err != nil {
			return err
		}
	}
	e.cloudLoggingClient.Close()
	return nil
}

func (e *exporter) Start(ctx context.Context, host component.Host) error {
	return nil
}
