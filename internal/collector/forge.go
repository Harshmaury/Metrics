// @metrics-project: metrics
// @metrics-path: internal/collector/forge.go
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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
func (c *ForgeCollector) Collect(ctx context.Context) snapshot.ForgeMetrics {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/history?limit=%d", c.baseURL, forgeHistoryLimit), nil)
	if err != nil {
		return snapshot.ForgeMetrics{Available: false, TopTargets: map[string]int{}}
	}
	if c.serviceToken != "" {
		req.Header.Set("X-Service-Token", c.serviceToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return snapshot.ForgeMetrics{Available: false, TopTargets: map[string]int{}}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return snapshot.ForgeMetrics{Available: false, TopTargets: map[string]int{}}
	}

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

	m := snapshot.ForgeMetrics{
		Available:  true,
		TopTargets: map[string]int{},
	}

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
