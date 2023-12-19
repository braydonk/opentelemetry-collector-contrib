// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

func TestCreateNewConfig(t *testing.T) {
	testCases := []struct {
		name            string
		processConfig   *Config
		processesConfig *ProcessesConfig
		expectedConfig  *Config
	}{
		{
			name:            "just process config",
			processConfig:   CreateNewDefaultConfig(),
			processesConfig: nil,
			expectedConfig:  CreateNewDefaultConfig(),
		},
		{
			name:            "just processes config",
			processConfig:   nil,
			processesConfig: createTestProcessesConfig(true, true),
			expectedConfig: func() *Config {
				cfg := CreateNewAllDisabledConfig()
				cfg.MetricsBuilderConfig.Metrics.SystemProcessesCount.Enabled = true
				cfg.MetricsBuilderConfig.Metrics.SystemProcessesCreated.Enabled = true
				return cfg
			}(),
		},
		{
			name:            "both scraper configs",
			processConfig:   CreateNewDefaultConfig(),
			processesConfig: createTestProcessesConfig(true, true),
			expectedConfig: func() *Config {
				cfg := CreateNewDefaultConfig()
				cfg.MetricsBuilderConfig.Metrics.SystemProcessesCount.Enabled = true
				cfg.MetricsBuilderConfig.Metrics.SystemProcessesCreated.Enabled = true
				return cfg
			}(),
		},
		{
			name:            "just system.processes.count enabled",
			processConfig:   CreateNewDefaultConfig(),
			processesConfig: createTestProcessesConfig(false, true),
			expectedConfig: func() *Config {
				cfg := CreateNewDefaultConfig()
				cfg.MetricsBuilderConfig.Metrics.SystemProcessesCount.Enabled = true
				cfg.MetricsBuilderConfig.Metrics.SystemProcessesCreated.Enabled = false
				return cfg
			}(),
		},
		{
			name:            "just system.processes.created enabled",
			processConfig:   CreateNewDefaultConfig(),
			processesConfig: createTestProcessesConfig(true, false),
			expectedConfig: func() *Config {
				cfg := CreateNewDefaultConfig()
				cfg.MetricsBuilderConfig.Metrics.SystemProcessesCount.Enabled = false
				cfg.MetricsBuilderConfig.Metrics.SystemProcessesCreated.Enabled = true
				return cfg
			}(),
		},
		{
			name:            "both nil",
			processConfig:   nil,
			processesConfig: nil,
			expectedConfig:  CreateNewAllDisabledConfig(),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := CreateNewConfig(tc.processConfig, tc.processesConfig)
			if diff := cmp.Diff(tc.expectedConfig, cfg, cmpopts.IgnoreUnexported(metadata.MetricConfig{}, metadata.ResourceAttributeConfig{})); diff != "" {
				t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
			}
		})
	}
}

func createTestProcessesConfig(enabledProcessesCreated, enableProcessesCount bool) *ProcessesConfig {
	cfg := &ProcessesConfig{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()}
	cfg.MetricsBuilderConfig.Metrics.SystemProcessesCount.Enabled = enableProcessesCount
	cfg.MetricsBuilderConfig.Metrics.SystemProcessesCreated.Enabled = enabledProcessesCreated
	return cfg
}
