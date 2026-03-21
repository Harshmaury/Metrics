// @metrics-project: metrics
// @metrics-path: internal/collector/nexus.go
// ADR-039: complete Herald migration — /metrics endpoint now uses typed client.
// Previously hybrid: events used Herald, /metrics used raw HTTP.
// Now: both use Herald. Raw httpClient removed entirely.
package collector

import (
	"context"
	"time"

	"github.com/Harshmaury/Canon/events"
	herald "github.com/Harshmaury/Herald/client"
	"github.com/Harshmaury/Metrics/internal/snapshot"
)

const (
	nexusEventLimit = 500
	crashWindowMins = 10
)

// NexusCollector polls Nexus /events and /metrics via Herald.
type NexusCollector struct {
	nexus       *herald.Client
	lastEventID int64
}

// NewNexusCollector creates a NexusCollector.
func NewNexusCollector(baseURL, serviceToken string) *NexusCollector {
	return &NexusCollector{
		nexus: herald.New(baseURL, herald.WithToken(serviceToken)),
	}
}

// CollectMetrics fetches Nexus runtime counters via Herald.
func (c *NexusCollector) CollectMetrics(ctx context.Context, traceID string) snapshot.NexusMetrics {
	m, err := c.nexus.NexusMetrics().Get(ctx)
	if err != nil {
		return snapshot.NexusMetrics{Available: false}
	}
	return snapshot.NexusMetrics{
		Available:             true,
		UptimeSeconds:         m.UptimeSeconds,
		ReconcileCyclesTotal:  m.ReconcileCyclesTotal,
		ServicesRunning:       m.ServicesRunning,
		ServicesInMaintenance: m.ServicesInMaintenance,
		ServicesStartedTotal:  m.ServicesStartedTotal,
		ServicesStoppedTotal:  m.ServicesStoppedTotal,
		ServicesCrashedTotal:  m.ServicesCrashedTotal,
	}
}

// CollectEvents fetches recent events via Herald and computes EventMetrics.
func (c *NexusCollector) CollectEvents(ctx context.Context, traceID string) snapshot.EventMetrics {
	empty := snapshot.EventMetrics{ByComponent: map[string]int{}, ByOutcome: map[string]int{}}

	evts, err := c.nexus.Events().Since(ctx, c.lastEventID, nexusEventLimit)
	if err != nil {
		return empty
	}

	m := snapshot.EventMetrics{ByComponent: map[string]int{}, ByOutcome: map[string]int{}}
	cutoff := time.Now().UTC().Add(-time.Duration(crashWindowMins) * time.Minute)

	for _, e := range evts {
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
			case events.EventServiceCrashed:
				m.RecentCrashes++
			case events.EventFileDropped:
				m.RecentDrops++
			case events.EventFileRouted:
				m.RecentRoutedFiles++
			}
		}
	}
	return m
}
