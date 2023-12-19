// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"
	"errors"
	"runtime"

	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
)

// This file implements Factory for Process scraper.

const (
	// TypeStr the value of "type" key in configuration.
	TypeStr = "process"
)

// Factory is the Factory for scraper.
type Factory struct{}

// CreateDefaultConfig creates the default configuration for the Scraper.
func (f *Factory) CreateDefaultConfig() internal.Config {
	return CreateNewDefaultConfig()
}

// CreateMetricsScraper creates a resource scraper based on provided config.
func (f *Factory) CreateMetricsScraper(
	_ context.Context,
	settings receiver.CreateSettings,
	cfg internal.Config,
) (scraperhelper.Scraper, error) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		return nil, errors.New("process scraper only available on Linux, Windows, or MacOS")
	}

	s, err := newProcessScraper(settings, cfg.(*Config))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraper(
		TypeStr,
		s.scrape,
		scraperhelper.WithStart(s.start),
	)
}

type ProcessesFactory struct{}

// CreateDefaultConfig creates the default configuration for the Scraper.
func (f *ProcessesFactory) CreateDefaultConfig() internal.Config {
	return CreateNewDefaultProcessesConfig()
}

// CreateMetricsScraper creates a resource scraper based on provided config.
func (f *ProcessesFactory) CreateMetricsScraper(
	_ context.Context,
	settings receiver.CreateSettings,
	cfg internal.Config,
) (scraperhelper.Scraper, error) {
	return nil, errors.New("processes factory error: this factory cannot create a metric scraper")
}
