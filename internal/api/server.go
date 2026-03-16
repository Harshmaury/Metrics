// @metrics-project: metrics
// @metrics-path: internal/api/server.go
// Metrics HTTP API server on 127.0.0.1:8083 (ADR-011).
// Read-only — no auth required on snapshot endpoint.
package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Harshmaury/Metrics/internal/api/handler"
)

// Server is the Metrics HTTP server.
type Server struct {
	http   *http.Server
	logger *log.Logger
}

// NewServer creates the Metrics HTTP server.
func NewServer(addr string, store *handler.SnapshotStore, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}

	snapshotH := handler.NewSnapshotHandler(store)
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health",           handleHealth)
	mux.HandleFunc("GET /metrics/snapshot", snapshotH.Get)

	return &Server{
		http: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		logger: logger,
	}
}

// Run starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Printf("Metrics API listening on %s", s.http.Addr)
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("metrics http: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.logger.Println("Metrics API shutting down...")
	return s.http.Shutdown(shutdownCtx)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true,"status":"healthy","service":"metrics"}`)) //nolint:errcheck
}
