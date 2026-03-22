// @metrics-project: metrics
// @metrics-path: cmd/metrics/main.go
// metrics is the platform Metrics observer daemon.
//
// Startup sequence:
//  1. Config
//  2. Collectors (Nexus, Atlas, Forge)
//  3. Snapshot store
//  4. Initial collection pass
//  5. HTTP server (:8083)
//  6. Polling loops
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Harshmaury/Metrics/internal/api"
	"github.com/Harshmaury/Metrics/internal/api/handler"
	"github.com/Harshmaury/Metrics/internal/collector"
	"github.com/Harshmaury/Metrics/internal/config"
	"github.com/Harshmaury/Metrics/internal/snapshot"
)

const metricsVersion = "0.1.0"

func main() {
	logger := log.New(os.Stdout, "[metrics] ", log.LstdFlags)
	logger.Printf("Metrics v%s starting", metricsVersion)
	if err := run(logger); err != nil {
		logger.Fatalf("fatal: %v", err)
	}
	logger.Println("Metrics stopped cleanly")
}

// metricsConfig holds resolved runtime configuration.
type metricsConfig struct {
	httpAddr     string
	nexusAddr    string
	atlasAddr    string
	forgeAddr    string
	serviceToken string
}

// loadConfig reads environment variables and logs warnings.
func loadConfig(logger *log.Logger) metricsConfig {
	cfg := metricsConfig{
		httpAddr:     config.EnvOrDefault("METRICS_HTTP_ADDR", config.DefaultHTTPAddr),
		nexusAddr:    config.EnvOrDefault("NEXUS_HTTP_ADDR", config.DefaultNexusAddr),
		atlasAddr:    config.EnvOrDefault("ATLAS_HTTP_ADDR", config.DefaultAtlasAddr),
		forgeAddr:    config.EnvOrDefault("FORGE_HTTP_ADDR", config.DefaultForgeAddr),
		serviceToken: config.EnvOrDefault("METRICS_SERVICE_TOKEN", ""),
	}
	if cfg.serviceToken == "" {
				if os.Getenv("ENGX_AUTH_REQUIRED") == "true" {
			logger.Fatalf("FATAL: ENGX_AUTH_REQUIRED=true but METRICS_SERVICE_TOKEN not set — refusing to start insecurely. Set METRICS_SERVICE_TOKEN in ~/.nexus/service-tokens or disable with ENGX_AUTH_REQUIRED=false")
		}
		logger.Println("WARNING: METRICS_SERVICE_TOKEN not set — inter-service auth disabled. Set ENGX_AUTH_REQUIRED=true to enforce strict mode.")
	}
	return cfg
}

func run(logger *log.Logger) error {
	cfg := loadConfig(logger)

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	nexusColl := collector.NewNexusCollector(cfg.nexusAddr, cfg.serviceToken)
	forgeColl := collector.NewForgeCollector(cfg.forgeAddr, cfg.serviceToken)
	atlasColl := collector.NewAtlasCollector(cfg.atlasAddr, cfg.serviceToken)
	snapStore  := handler.NewSnapshotStore()

	collectAll(ctx, nexusColl, forgeColl, atlasColl, snapStore, logger)
	logger.Printf("✓ Metrics ready — http=%s nexus=%s atlas=%s forge=%s",
		cfg.httpAddr, cfg.nexusAddr, cfg.atlasAddr, cfg.forgeAddr)

	return serveAndWait(ctx, cancel, sigCh, cfg.httpAddr,
		snapStore, nexusColl, forgeColl, atlasColl, logger)
}

// serveAndWait starts the HTTP server and polling loop, blocks until shutdown.
func serveAndWait(
	ctx context.Context,
	cancel context.CancelFunc,
	sigCh <-chan os.Signal,
	httpAddr string,
	snapStore *handler.SnapshotStore,
	nexusColl *collector.NexusCollector,
	forgeColl *collector.ForgeCollector,
	atlasColl *collector.AtlasCollector,
	logger *log.Logger,
) error {
	srv  := api.NewServer(httpAddr, snapStore, logger)
	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Run(ctx); err != nil && ctx.Err() == nil {
			errCh <- fmt.Errorf("api server: %w", err)
		}
	}()

	wg.Add(1)
	go startPollingLoop(ctx, &wg, nexusColl, forgeColl, atlasColl, snapStore, logger)

	select {
	case sig := <-sigCh:
		logger.Printf("received %s — shutting down", sig)
	case err := <-errCh:
		logger.Printf("component error: %v — shutting down", err)
	}

	cancel()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	<-done
	return nil
}

// startPollingLoop runs staggered collection tickers until ctx is cancelled.
func startPollingLoop(
	ctx context.Context,
	wg *sync.WaitGroup,
	nexusColl *collector.NexusCollector,
	forgeColl *collector.ForgeCollector,
	atlasColl *collector.AtlasCollector,
	store *handler.SnapshotStore,
	logger *log.Logger,
) {
	defer wg.Done()
	nexusTick := time.NewTicker(5 * time.Second)
	forgeTick := time.NewTicker(10 * time.Second)
	atlasTick := time.NewTicker(30 * time.Second)
	defer nexusTick.Stop()
	defer forgeTick.Stop()
	defer atlasTick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-nexusTick.C:
			collectAll(ctx, nexusColl, forgeColl, atlasColl, store, logger)
		case <-forgeTick.C:
			collectAll(ctx, nexusColl, forgeColl, atlasColl, store, logger)
		case <-atlasTick.C:
			collectAll(ctx, nexusColl, forgeColl, atlasColl, store, logger)
		}
	}
}

// collectAll runs all collectors under a single trace ID and atomically
// replaces the snapshot store (FEAT-002).
func collectAll(
	ctx context.Context,
	nexusColl *collector.NexusCollector,
	forgeColl *collector.ForgeCollector,
	atlasColl *collector.AtlasCollector,
	store *handler.SnapshotStore,
	logger *log.Logger,
) {
	traceID := newTraceID()
	snap := &snapshot.Snapshot{
		CollectedAt: time.Now().UTC(),
		Nexus:       nexusColl.CollectMetrics(ctx, traceID),
		Events:      nexusColl.CollectEvents(ctx, traceID),
		Forge:       forgeColl.Collect(ctx, traceID),
		Atlas:       atlasColl.Collect(ctx, traceID),
	}
	store.Set(snap)
	logger.Printf("snapshot trace=%s nexus=%v atlas=%v forge=%v",
		traceID, snap.Nexus.Available, snap.Atlas.Available, snap.Forge.Available)
}

// newTraceID generates a random trace ID for collection cycles (FEAT-002).
func newTraceID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("mt-%d", time.Now().UnixNano())
	}
	return "mt-" + hex.EncodeToString(b)
}
