// @metrics-project: metrics
// @metrics-path: internal/collector/forge.go
// ADR-039: full Herald migration — Forge history calls now use typed client.
// Replaces: raw http.NewRequestWithContext + anonymous struct decode.
package collector

import (
	"context"

	herald "github.com/Harshmaury/Herald/client"
	"github.com/Harshmaury/Metrics/internal/snapshot"
)

const forgeHistoryLimit = 50

// ForgeCollector polls Forge GET /history via Herald.
type ForgeCollector struct {
	forge *herald.Client
}

// NewForgeCollector creates a ForgeCollector.
func NewForgeCollector(baseURL, serviceToken string) *ForgeCollector {
	return &ForgeCollector{
		forge: herald.NewForService(baseURL, serviceToken),
	}
}

// Collect fetches recent execution records and computes ForgeMetrics.
func (c *ForgeCollector) Collect(ctx context.Context, traceID string) snapshot.ForgeMetrics {
	empty := snapshot.ForgeMetrics{Available: false, TopTargets: map[string]int{}}

	records, err := c.forge.Forge().History(ctx, forgeHistoryLimit)
	if err != nil {
		return empty
	}

	m := snapshot.ForgeMetrics{Available: true, TopTargets: map[string]int{}}
	var totalDuration int64
	for _, r := range records {
		m.TotalExecutions++
		totalDuration += r.DurationMS
		m.TopTargets[r.Target]++
		switch r.Status {
		case "success":
			m.SuccessCount++
		case "failure":
			m.FailureCount++
		case "denied":
			m.DeniedCount++
		}
	}
	if m.TotalExecutions > 0 {
		m.AvgDurationMS = float64(totalDuration) / float64(m.TotalExecutions)
	}
	return m
}
