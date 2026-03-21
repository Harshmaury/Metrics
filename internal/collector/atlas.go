// @metrics-project: metrics
// @metrics-path: internal/collector/atlas.go
// ADR-039: full Herald migration — Atlas workspace calls now use typed client.
// Replaces: raw http.NewRequestWithContext + anonymous struct decode.
package collector

import (
	"context"

	herald "github.com/Harshmaury/Herald/client"
	"github.com/Harshmaury/Metrics/internal/snapshot"
)

// AtlasCollector polls Atlas GET /workspace/projects via Herald.
type AtlasCollector struct {
	atlas *herald.Client
}

// NewAtlasCollector creates an AtlasCollector.
func NewAtlasCollector(baseURL, serviceToken string) *AtlasCollector {
	return &AtlasCollector{
		atlas: herald.NewForService(baseURL, serviceToken),
	}
}

// Collect fetches all projects and computes AtlasMetrics.
func (c *AtlasCollector) Collect(ctx context.Context, traceID string) snapshot.AtlasMetrics {
	empty := snapshot.AtlasMetrics{Available: false, ByLanguage: map[string]int{}}

	projects, err := c.atlas.Atlas().Projects(ctx)
	if err != nil {
		return empty
	}

	m := snapshot.AtlasMetrics{Available: true, ByLanguage: map[string]int{}}
	for _, p := range projects {
		m.TotalProjects++
		if p.Status == "verified" {
			m.VerifiedCount++
		} else {
			m.UnverifiedCount++
		}
		if p.Language != "" {
			m.ByLanguage[p.Language]++
		}
	}
	return m
}
