// @metrics-project: metrics
// @metrics-path: internal/api/handler/prometheus.go
// PrometheusHandler serves GET /metrics/prometheus in standard Prometheus
// text exposition format. No external dependencies — plain text output
// generated directly from the existing Snapshot struct.
//
// Phase 2 (ADR-011 amendment): same data as GET /metrics/snapshot,
// in a format compatible with Prometheus, Grafana Agent, VictoriaMetrics,
// and any scraper that speaks the text exposition format.
//
// All metric names use the engx_ prefix.
// Gauges = current state. Counters = monotonically increasing totals.
package handler

import (
	"fmt"
	"io"
	"net/http"

	"github.com/Harshmaury/Metrics/internal/snapshot"
)

const prometheusContentType = "text/plain; version=0.0.4; charset=utf-8"

// PrometheusHandler serves GET /metrics/prometheus.
type PrometheusHandler struct {
	store *SnapshotStore
}

// NewPrometheusHandler creates a PrometheusHandler.
func NewPrometheusHandler(s *SnapshotStore) *PrometheusHandler {
	return &PrometheusHandler{store: s}
}

// Get handles GET /metrics/prometheus.
func (h *PrometheusHandler) Get(w http.ResponseWriter, r *http.Request) {
	snap := h.store.Get()
	w.Header().Set("Content-Type", prometheusContentType)
	w.WriteHeader(http.StatusOK)
	writeNexusMetrics(w, snap.Nexus)
	writeForgeMetrics(w, snap.Forge)
	writeAtlasMetrics(w, snap.Atlas)
	writeEventMetrics(w, snap.Events)
}

// writeNexusMetrics emits engxd runtime counters.
func writeNexusMetrics(w io.Writer, n snapshot.NexusMetrics) {
	gauge(w, "engx_nexus_up", "1 if Nexus is reachable", boolToFloat(n.Available))
	gauge(w, "engx_uptime_seconds", "Nexus daemon uptime in seconds", n.UptimeSeconds)
	gauge(w, "engx_services_running", "Services in running state", float64(n.ServicesRunning))
	gauge(w, "engx_services_in_maintenance", "Services in maintenance", float64(n.ServicesInMaintenance))
	counter(w, "engx_reconcile_cycles_total", "Total reconciler cycles", float64(n.ReconcileCyclesTotal))
	counter(w, "engx_services_started_total", "Total service starts", float64(n.ServicesStartedTotal))
	counter(w, "engx_services_stopped_total", "Total service stops", float64(n.ServicesStoppedTotal))
	counter(w, "engx_services_crashed_total", "Total service crashes", float64(n.ServicesCrashedTotal))
}

// writeForgeMetrics emits Forge execution stats.
func writeForgeMetrics(w io.Writer, f snapshot.ForgeMetrics) {
	gauge(w, "engx_forge_up", "1 if Forge is reachable", boolToFloat(f.Available))
	counter(w, "engx_forge_executions_total", "Total commands executed", float64(f.TotalExecutions))
	counter(w, "engx_forge_success_total", "Total successful executions", float64(f.SuccessCount))
	counter(w, "engx_forge_failure_total", "Total failed executions", float64(f.FailureCount))
	counter(w, "engx_forge_denied_total", "Total denied executions", float64(f.DeniedCount))
	gauge(w, "engx_forge_avg_duration_ms", "Average execution duration ms", f.AvgDurationMS)
}

// writeAtlasMetrics emits workspace knowledge stats.
func writeAtlasMetrics(w io.Writer, a snapshot.AtlasMetrics) {
	gauge(w, "engx_atlas_up", "1 if Atlas is reachable", boolToFloat(a.Available))
	gauge(w, "engx_atlas_projects_total", "Total registered projects", float64(a.TotalProjects))
	gauge(w, "engx_atlas_verified_total", "Projects with valid nexus.yaml", float64(a.VerifiedCount))
	gauge(w, "engx_atlas_unverified_total", "Projects without valid nexus.yaml", float64(a.UnverifiedCount))
}

// writeEventMetrics emits Nexus event activity.
func writeEventMetrics(w io.Writer, e snapshot.EventMetrics) {
	gauge(w, "engx_events_total", "Events seen in this collection", float64(e.TotalSeen))
	gauge(w, "engx_recent_crashes", "SERVICE_CRASHED events in last 10 min", float64(e.RecentCrashes))
	gauge(w, "engx_recent_routed_files", "FILE_ROUTED events in last 10 min", float64(e.RecentRoutedFiles))
}

// ── FORMAT HELPERS ────────────────────────────────────────────────────────────

func gauge(w io.Writer, name, help string, value float64) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %g\n", name, help, name, name, value)
}

func counter(w io.Writer, name, help string, value float64) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %g\n", name, help, name, name, value)
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}
