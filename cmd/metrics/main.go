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

func run(logger *log.Logger) error {
	// ── 1. CONFIG ────────────────────────────────────────────────────────────
	httpAddr     := config.EnvOrDefault("METRICS_HTTP_ADDR", config.DefaultHTTPAddr)
	nexusAddr    := config.EnvOrDefault("NEXUS_HTTP_ADDR", config.DefaultNexusAddr)
	atlasAddr    := config.EnvOrDefault("ATLAS_HTTP_ADDR", config.DefaultAtlasAddr)
	forgeAddr    := config.EnvOrDefault("FORGE_HTTP_ADDR", config.DefaultForgeAddr)
	serviceToken := config.EnvOrDefault("METRICS_SERVICE_TOKEN", "")
	if serviceToken == "" {
		logger.Println("WARNING: METRICS_SERVICE_TOKEN not set — upstream auth disabled")
	}

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// ── 2. COLLECTORS ────────────────────────────────────────────────────────
	nexusColl := collector.NewNexusCollector(nexusAddr, serviceToken)
	forgeColl := collector.NewForgeCollector(forgeAddr, serviceToken)
	atlasColl := collector.NewAtlasCollector(atlasAddr, serviceToken)

	// ── 3. SNAPSHOT STORE ─────────────────────────────────────────────────────
	snapStore := handler.NewSnapshotStore()

	// ── 4. INITIAL COLLECTION ────────────────────────────────────────────────
	collectAll(ctx, nexusColl, forgeColl, atlasColl, snapStore, logger)
	logger.Printf("✓ Metrics ready — http=%s nexus=%s atlas=%s forge=%s",
		httpAddr, nexusAddr, atlasAddr, forgeAddr)

	// ── 5. HTTP SERVER ───────────────────────────────────────────────────────
	srv := api.NewServer(httpAddr, snapStore, logger)

	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Run(ctx); err != nil && ctx.Err() == nil {
			errCh <- fmt.Errorf("api server: %w", err)
		}
	}()

	// ── 6. POLLING LOOPS ─────────────────────────────────────────────────────
	wg.Add(1)
	go func() {
		defer wg.Done()
		nexusTick  := time.NewTicker(5 * time.Second)
		forgeTick  := time.NewTicker(10 * time.Second)
		atlasTick  := time.NewTicker(30 * time.Second)
		defer nexusTick.Stop()
		defer forgeTick.Stop()
		defer atlasTick.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-nexusTick.C:
				collectAll(ctx, nexusColl, forgeColl, atlasColl, snapStore, logger)
			case <-forgeTick.C:
				collectAll(ctx, nexusColl, forgeColl, atlasColl, snapStore, logger)
			case <-atlasTick.C:
				collectAll(ctx, nexusColl, forgeColl, atlasColl, snapStore, logger)
			}
		}
	}()

	// ── WAIT FOR SHUTDOWN ─────────────────────────────────────────────────────
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

// collectAll runs all collectors and updates the snapshot store.
func collectAll(ctx context.Context, nexusColl *collector.NexusCollector, forgeColl *collector.ForgeCollector, atlasColl *collector.AtlasCollector, store *handler.SnapshotStore, logger *log.Logger) {
	snap := &snapshot.Snapshot{
		CollectedAt: time.Now().UTC(),
		Nexus:       nexusColl.CollectMetrics(ctx),
		Events:      nexusColl.CollectEvents(ctx),
		Forge:       forgeColl.Collect(ctx),
		Atlas:       atlasColl.Collect(ctx),
	}
	store.Set(snap)
	logger.Printf("snapshot collected — nexus=%v atlas=%v forge=%v",
		snap.Nexus.Available, snap.Atlas.Available, snap.Forge.Available)
}
