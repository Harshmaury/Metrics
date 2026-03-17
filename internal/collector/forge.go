// @metrics-project: metrics
// @metrics-path: internal/collector/forge.go
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Harshmaury/Canon/identity"
	"github.com/Harshmaury/Metrics/internal/snapshot"
)

const forgeHistoryLimit = 50

// ForgeCollector polls Forge GET /history.
type ForgeCollector struct {
	baseURL      string
	serviceToken string
	httpClient   *http.Client
}

// NewForgeCollector creates a ForgeCollector.
func NewForgeCollector(baseURL, serviceToken string) *ForgeCollector {
	return &ForgeCollector{
		baseURL:      baseURL,
		serviceToken: serviceToken,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Collect fetches recent execution records and computes ForgeMetrics.
// traceID is the collection-cycle trace ID for X-Trace-ID propagation (FEAT-002).
func (c *ForgeCollector) Collect(ctx context.Context, traceID string) snapshot.ForgeMetrics {
	empty := snapshot.ForgeMetrics{Available: false, TopTargets: map[string]int{}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/history?limit=%d", c.baseURL, forgeHistoryLimit), nil)
	if err != nil {
		return empty
	}
	if c.serviceToken != "" {
		req.Header.Set(identity.ServiceTokenHeader, c.serviceToken)
	}
	if traceID != "" {
		req.Header.Set(identity.TraceIDHeader, traceID)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return empty
	}
	defer resp.Body.Close()
	return computeForgeMetrics(resp)
}

// computeForgeMetrics decodes the Forge /history response and aggregates stats.
func computeForgeMetrics(resp *http.Response) snapshot.ForgeMetrics {
	var envelope struct {
		OK   bool `json:"ok"`
		Data []struct {
			Target     string `json:"target"`
			Status     string `json:"status"`
			DurationMS int64  `json:"duration_ms"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return snapshot.ForgeMetrics{Available: false, TopTargets: map[string]int{}}
	}
	m := snapshot.ForgeMetrics{Available: true, TopTargets: map[string]int{}}
	var totalDuration int64
	for _, r := range envelope.Data {
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
