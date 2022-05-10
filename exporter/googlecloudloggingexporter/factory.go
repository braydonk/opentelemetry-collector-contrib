package googlecloudloggingexporter

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

var (
	typeStr        config.Type = "googlecloudlogging"
	defaultTimeout             = 12 * time.Second
)

func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithLogsExporter(createLogsExporter))
}

// createDefaultConfig creates the default configuration for exporter.
func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings:  exporterhelper.TimeoutSettings{Timeout: defaultTimeout},
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),
		UserAgent:        "opentelemetry-collector-contrib {{version}}",
	}
}

// createLogsExporter creates the Google Cloud Logging exporter
func createLogsExporter(_ context.Context, params component.ExporterCreateSettings, config config.Exporter) (component.LogsExporter, error) {
	expConfig, ok := config.(*Config)
	if !ok {
		return nil, errors.New("invalid configuration type; can't cast to googlecloudloggingexporter.Config")
	}
	return newCloudLoggingExporter(expConfig, params)
}
