// @metrics-project: metrics
// @metrics-path: internal/api/handler/snapshot.go
// SnapshotHandler handles GET /metrics/snapshot.
// Returns the latest aggregated platform health snapshot.
// No auth required — read-only dashboard endpoint (ADR-011).
package handler

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/Harshmaury/Metrics/internal/snapshot"
)

// SnapshotStore holds the latest computed snapshot — in-memory, no SQLite.
type SnapshotStore struct {
	mu       sync.RWMutex
	current  *snapshot.Snapshot
}

// NewSnapshotStore creates an empty SnapshotStore.
func NewSnapshotStore() *SnapshotStore {
	return &SnapshotStore{}
}

// Set updates the stored snapshot atomically.
func (s *SnapshotStore) Set(snap *snapshot.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = snap
}

// Get returns the latest snapshot, or a zero snapshot if none yet.
func (s *SnapshotStore) Get() *snapshot.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.current == nil {
		return &snapshot.Snapshot{CollectedAt: time.Now().UTC()}
	}
	return s.current
}

// SnapshotHandler handles GET /metrics/snapshot.
type SnapshotHandler struct {
	store *SnapshotStore
}

// NewSnapshotHandler creates a SnapshotHandler.
func NewSnapshotHandler(s *SnapshotStore) *SnapshotHandler {
	return &SnapshotHandler{store: s}
}

// Get handles GET /metrics/snapshot.
func (h *SnapshotHandler) Get(w http.ResponseWriter, r *http.Request) {
	snap := h.store.Get()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"ok":   true,
		"data": snap,
	})
}
