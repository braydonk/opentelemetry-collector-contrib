package googlecloudloggingexporter

import (
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
)

type Config struct {
	config.ExporterSettings `mapstructure:",squash"`
	ProjectID               string `mapstructure:"project"`
	UserAgent               string `mapstructure:"user_agent"`
	LogName                 string `mapstructure:"log_name"`

	Endpoint string `mapstructure:"endpoint"`
	// Only has effect if Endpoint is not ""
	UseInsecure bool `mapstructure:"use_insecure"`

	// Timeout for all API calls. If not set, defaults to 12 seconds.
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`

	logger *zap.Logger
}

func (c Config) Validate() error {
	return nil
}
