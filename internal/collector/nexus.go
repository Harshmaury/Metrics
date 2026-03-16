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

	"github.com/Harshmaury/Metrics/internal/snapshot"
)

const (
	nexusEventLimit  = 100
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
func (c *NexusCollector) CollectMetrics(ctx context.Context) snapshot.NexusMetrics {
	resp, err := c.get(ctx, "/metrics")
	if err != nil {
		return snapshot.NexusMetrics{Available: false}
	}
	defer resp.Body.Close()

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
func (c *NexusCollector) CollectEvents(ctx context.Context) snapshot.EventMetrics {
	path := fmt.Sprintf("/events?since=%d&limit=%d", c.lastEventID, nexusEventLimit)
	resp, err := c.get(ctx, path)
	if err != nil {
		return snapshot.EventMetrics{
			ByComponent: map[string]int{},
			ByOutcome:   map[string]int{},
		}
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
		return snapshot.EventMetrics{
			ByComponent: map[string]int{},
			ByOutcome:   map[string]int{},
		}
	}

	m := snapshot.EventMetrics{
		ByComponent: map[string]int{},
		ByOutcome:   map[string]int{},
	}

	cutoff := time.Now().UTC().Add(-time.Duration(crashWindowMins) * time.Minute)

	for _, e := range envelope.Data {
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
			ts, _ = time.Parse(time.RFC3339, e.CreatedAt)
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
func (c *NexusCollector) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.serviceToken != "" && path != "/health" {
		req.Header.Set("X-Service-Token", c.serviceToken)
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

