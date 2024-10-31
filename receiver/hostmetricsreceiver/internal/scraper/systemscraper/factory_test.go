package systemscraper

import (
	"context"
	"testing"

	"github.com/tj/assert"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateSystemScraper(t *testing.T) {
	factory := &Factory{}
	cfg := &Config{}

	scraper, err := factory.CreateMetricsScraper(context.Background(), receivertest.NewNopSettings(), cfg)

	assert.NoError(t, err)
	assert.NotNil(t, scraper)
	assert.Equal(t, scraperType.String(), scraper.ID().String())
}
