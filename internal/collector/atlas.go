// @metrics-project: metrics
// @metrics-path: internal/collector/atlas.go
package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Harshmaury/Canon/identity"
	"github.com/Harshmaury/Metrics/internal/snapshot"
)

// AtlasCollector polls Atlas GET /workspace/projects.
type AtlasCollector struct {
	baseURL      string
	serviceToken string
	httpClient   *http.Client
}

// NewAtlasCollector creates an AtlasCollector.
func NewAtlasCollector(baseURL, serviceToken string) *AtlasCollector {
	return &AtlasCollector{
		baseURL:      baseURL,
		serviceToken: serviceToken,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Collect fetches all projects and computes AtlasMetrics.
// traceID is the collection-cycle trace ID for X-Trace-ID propagation (FEAT-002).
func (c *AtlasCollector) Collect(ctx context.Context, traceID string) snapshot.AtlasMetrics {
	empty := snapshot.AtlasMetrics{Available: false, ByLanguage: map[string]int{}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/workspace/projects", nil)
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
	return computeAtlasMetrics(resp)
}

// computeAtlasMetrics decodes the Atlas /workspace/projects response and aggregates stats.
func computeAtlasMetrics(resp *http.Response) snapshot.AtlasMetrics {
	var envelope struct {
		OK   bool `json:"ok"`
		Data []struct {
			Language string `json:"language"`
			Status   string `json:"status"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return snapshot.AtlasMetrics{Available: false, ByLanguage: map[string]int{}}
	}
	m := snapshot.AtlasMetrics{Available: true, ByLanguage: map[string]int{}}
	for _, p := range envelope.Data {
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
