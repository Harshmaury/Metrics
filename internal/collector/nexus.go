// @metrics-project: metrics
// @metrics-path: internal/collector/nexus.go
// Package collector provides pollers for each upstream platform service.
// All collectors are read-only (ADR-011).
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

const (
	nexusEventLimit  = 500 // increased from 100 (ISSUE-006)
	crashWindowMins  = 10
	dropWindowMins   = 10
)

// NexusCollector polls Nexus /events and /metrics.
type NexusCollector struct {
	baseURL      string
	serviceToken string
	httpClient   *http.Client
	lastEventID  int64
}

// NewNexusCollector creates a NexusCollector.
func NewNexusCollector(baseURL, serviceToken string) *NexusCollector {
	return &NexusCollector{
		baseURL:      baseURL,
		serviceToken: serviceToken,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// CollectMetrics fetches Nexus runtime counters from GET /metrics.
// traceID is the collection-cycle trace ID for X-Trace-ID propagation (FEAT-002).
func (c *NexusCollector) CollectMetrics(ctx context.Context, traceID string) snapshot.NexusMetrics {
	resp, err := c.get(ctx, "/metrics", traceID)
	if err != nil {
		return snapshot.NexusMetrics{Available: false}
	}
	defer resp.Body.Close()
	return parseNexusMetrics(resp)
}

// parseNexusMetrics decodes the Nexus /metrics response body.
func parseNexusMetrics(resp *http.Response) snapshot.NexusMetrics {
	var raw struct {
		UptimeSeconds         float64 `json:"uptime_seconds"`
		ReconcileCyclesTotal  int64   `json:"reconcile_cycles_total"`
		ServicesRunning       int64   `json:"services_running"`
		ServicesInMaintenance int64   `json:"services_in_maintenance"`
		ServicesStartedTotal  int64   `json:"services_started_total"`
		ServicesStoppedTotal  int64   `json:"services_stopped_total"`
		ServicesCrashedTotal  int64   `json:"services_crashed_total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return snapshot.NexusMetrics{Available: false}
	}
	return snapshot.NexusMetrics{
		Available:             true,
		UptimeSeconds:         raw.UptimeSeconds,
		ReconcileCyclesTotal:  raw.ReconcileCyclesTotal,
		ServicesRunning:       raw.ServicesRunning,
		ServicesInMaintenance: raw.ServicesInMaintenance,
		ServicesStartedTotal:  raw.ServicesStartedTotal,
		ServicesStoppedTotal:  raw.ServicesStoppedTotal,
		ServicesCrashedTotal:  raw.ServicesCrashedTotal,
	}
}

// CollectEvents fetches recent events from GET /events?since=<id>.
// traceID is the collection-cycle trace ID for X-Trace-ID propagation (FEAT-002).
func (c *NexusCollector) CollectEvents(ctx context.Context, traceID string) snapshot.EventMetrics {
	empty := snapshot.EventMetrics{ByComponent: map[string]int{}, ByOutcome: map[string]int{}}
	path := fmt.Sprintf("/events?since=%d&limit=%d", c.lastEventID, nexusEventLimit)
	resp, err := c.get(ctx, path, traceID)
	if err != nil {
		return empty
	}
	defer resp.Body.Close()

	var envelope struct {
		OK   bool `json:"ok"`
		Data []struct {
			ID        int64  `json:"id"`
			Type      string `json:"type"`
			Component string `json:"component"`
			Outcome   string `json:"outcome"`
			CreatedAt string `json:"created_at"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return empty
	}
	return c.aggregateEvents(envelope.Data)
}

// aggregateEvents computes EventMetrics from decoded event rows.
// Updates lastEventID as a side effect.
func (c *NexusCollector) aggregateEvents(rows []struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Component string `json:"component"`
	Outcome   string `json:"outcome"`
	CreatedAt string `json:"created_at"`
}) snapshot.EventMetrics {
	m := snapshot.EventMetrics{ByComponent: map[string]int{}, ByOutcome: map[string]int{}}
	cutoff := time.Now().UTC().Add(-time.Duration(crashWindowMins) * time.Minute)

	for _, e := range rows {
		if e.ID > c.lastEventID {
			c.lastEventID = e.ID
		}
		m.TotalSeen++
		if e.Component != "" {
			m.ByComponent[e.Component]++
		}
		if e.Outcome != "" {
			m.ByOutcome[e.Outcome]++
		}
		ts, err := time.Parse(time.RFC3339Nano, e.CreatedAt)
		if err != nil {
			ts, err = time.Parse(time.RFC3339, e.CreatedAt)
			if err != nil {
				ts = time.Time{}
			}
		}
		if ts.After(cutoff) {
			switch e.Type {
			case "SERVICE_CRASHED":
				m.RecentCrashes++
			case "FILE_DROPPED":
				m.RecentDrops++
			case "FILE_ROUTED":
				m.RecentRoutedFiles++
			}
		}
	}
	return m
}

// get performs an authenticated GET against the Nexus API.
func (c *NexusCollector) get(ctx context.Context, path, traceID string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.serviceToken != "" && path != "/health" {
		req.Header.Set(identity.ServiceTokenHeader, c.serviceToken)
	}
	if traceID != "" {
		req.Header.Set(identity.TraceIDHeader, traceID)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("nexus: HTTP %d for %s", resp.StatusCode, path)
	}
	return resp, nil
}

