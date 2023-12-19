// Code generated by mdatagen. DO NOT EDIT.

package metadata

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestMetricsBuilderConfig(t *testing.T) {
	tests := []struct {
		name string
		want MetricsBuilderConfig
	}{
		{
			name: "default",
			want: DefaultMetricsBuilderConfig(),
		},
		{
			name: "all_set",
			want: MetricsBuilderConfig{
				Metrics: MetricsConfig{
					SystemNetworkConnections:    MetricConfig{Enabled: true},
					SystemNetworkConntrackCount: MetricConfig{Enabled: true},
					SystemNetworkConntrackMax:   MetricConfig{Enabled: true},
					SystemNetworkDropped:        MetricConfig{Enabled: true},
					SystemNetworkErrors:         MetricConfig{Enabled: true},
					SystemNetworkIo:             MetricConfig{Enabled: true},
					SystemNetworkPackets:        MetricConfig{Enabled: true},
				},
			},
		},
		{
			name: "none_set",
			want: MetricsBuilderConfig{
				Metrics: MetricsConfig{
					SystemNetworkConnections:    MetricConfig{Enabled: false},
					SystemNetworkConntrackCount: MetricConfig{Enabled: false},
					SystemNetworkConntrackMax:   MetricConfig{Enabled: false},
					SystemNetworkDropped:        MetricConfig{Enabled: false},
					SystemNetworkErrors:         MetricConfig{Enabled: false},
					SystemNetworkIo:             MetricConfig{Enabled: false},
					SystemNetworkPackets:        MetricConfig{Enabled: false},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := loadMetricsBuilderConfig(t, tt.name)
			if diff := cmp.Diff(tt.want, cfg, cmpopts.IgnoreUnexported(MetricConfig{})); diff != "" {
				t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
			}
		})
	}
}

func TestDefaultAllDisabledMetricsConfig(t *testing.T) {
	want := MetricsConfig{
		SystemNetworkConnections:    MetricConfig{Enabled: false},
		SystemNetworkConntrackCount: MetricConfig{Enabled: false},
		SystemNetworkConntrackMax:   MetricConfig{Enabled: false},
		SystemNetworkDropped:        MetricConfig{Enabled: false},
		SystemNetworkErrors:         MetricConfig{Enabled: false},
		SystemNetworkIo:             MetricConfig{Enabled: false},
		SystemNetworkPackets:        MetricConfig{Enabled: false},
	}
	cfg := DefaultAllDisabledMetricsConfig()
	if diff := cmp.Diff(want, cfg, cmpopts.IgnoreUnexported(MetricConfig{})); diff != "" {
		t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
	}
}

func loadMetricsBuilderConfig(t *testing.T, name string) MetricsBuilderConfig {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	sub, err := cm.Sub(name)
	require.NoError(t, err)
	cfg := DefaultMetricsBuilderConfig()
	require.NoError(t, component.UnmarshalConfig(sub, &cfg))
	return cfg
}
