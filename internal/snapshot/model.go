// @metrics-project: metrics
// @metrics-path: internal/snapshot/model.go
// Package snapshot defines the Snapshot type — the single aggregated view
// of platform health that GET /metrics/snapshot returns.
//
// Fields may be added freely. Removing or renaming fields requires ADR-011 amendment.
package snapshot

import "time"

// Snapshot is the complete platform health view at a point in time.
type Snapshot struct {
	CollectedAt time.Time `json:"collected_at"`

	// Nexus runtime metrics (from GET /metrics)
	Nexus NexusMetrics `json:"nexus"`

	// Nexus event activity (from GET /events)
	Events EventMetrics `json:"events"`

	// Forge execution history (from GET /history)
	Forge ForgeMetrics `json:"forge"`

	// Atlas workspace state (from GET /workspace/projects)
	Atlas AtlasMetrics `json:"atlas"`
}

// NexusMetrics holds runtime counters from Nexus GET /metrics.
type NexusMetrics struct {
	Available             bool    `json:"available"`
	UptimeSeconds         float64 `json:"uptime_seconds"`
	ReconcileCyclesTotal  int64   `json:"reconcile_cycles_total"`
	ServicesRunning       int64   `json:"services_running"`
	ServicesInMaintenance int64   `json:"services_in_maintenance"`
	ServicesStartedTotal  int64   `json:"services_started_total"`
	ServicesStoppedTotal  int64   `json:"services_stopped_total"`
	ServicesCrashedTotal  int64   `json:"services_crashed_total"`
}

// EventMetrics holds activity derived from Nexus GET /events.
type EventMetrics struct {
	TotalSeen         int            `json:"total_seen"`
	ByComponent       map[string]int `json:"by_component"`
	ByOutcome         map[string]int `json:"by_outcome"`
	RecentCrashes     int            `json:"recent_crashes"`      // SERVICE_CRASHED in last 10 min
	RecentDrops       int            `json:"recent_drops"`         // FILE_DROPPED in last 10 min
	RecentRoutedFiles int            `json:"recent_routed_files"`  // FILE_ROUTED in last 10 min
}

// ForgeMetrics holds execution stats from Forge GET /history.
type ForgeMetrics struct {
	Available        bool           `json:"available"`
	TotalExecutions  int            `json:"total_executions"`
	SuccessCount     int            `json:"success_count"`
	FailureCount     int            `json:"failure_count"`
	DeniedCount      int            `json:"denied_count"`
	AvgDurationMS    float64        `json:"avg_duration_ms"`
	TopTargets       map[string]int `json:"top_targets"`
}

// AtlasMetrics holds workspace state from Atlas GET /workspace/projects.
type AtlasMetrics struct {
	Available        bool           `json:"available"`
	TotalProjects    int            `json:"total_projects"`
	VerifiedCount    int            `json:"verified_count"`
	UnverifiedCount  int            `json:"unverified_count"`
	ByLanguage       map[string]int `json:"by_language"`
}
