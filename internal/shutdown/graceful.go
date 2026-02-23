// Package shutdown provides ordered graceful shutdown for the scraper pipeline.
// E-13: Implements a 7-step shutdown sequence ensuring zero data loss on deployment restarts.
package shutdown

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Components aggregates all scraper component references for shutdown coordination.
type Components struct {
	Cancel       context.CancelFunc
	WorkerPool   Drainable
	ParserPool   Shutdownable
	StreamWriter Flushable
	DiskFallback Flushable
	DedupFilter  Persistable
	Redis        Closeable
}

// Drainable can wait for in-flight operations to complete.
type Drainable interface {
	Drain()
}

// Shutdownable can be shut down gracefully.
type Shutdownable interface {
	Shutdown()
}

// Flushable can flush buffered data.
type Flushable interface {
	Flush(ctx context.Context) error
}

// Persistable can persist state.
type Persistable interface {
	Persist(ctx context.Context) error
}

// Closeable can be closed.
type Closeable interface {
	Close() error
}

// GracefulShutdown listens for SIGTERM/SIGINT and executes an ordered
// pipeline drain to ensure zero data loss.
//
// Shutdown sequence (total budget: 2 minutes):
// 1. Cancel context — stops frontier from popping new URLs
// 2. Drain worker pool — wait for in-flight fetches (30s timeout)
// 3. Shutdown parser pool — wait for in-flight parses
// 4. Flush StreamWriter — write buffered records to Redis
// 5. Flush disk fallback — write local buffer if Redis was down
// 6. Persist dedup filter — no-op for RedisBloom
// 7. Close Redis connections
func GracefulShutdown(components *Components, logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig

	logger.Info("shutdown_initiated")
	shutdownCtx, done := context.WithTimeout(context.Background(), 2*time.Minute)
	defer done()

	// STEP 1: Stop frontier popping — no new URLs dispatched
	if components.Cancel != nil {
		components.Cancel()
	}
	logger.Info("frontier_stopped")

	// STEP 2: Wait for all in-flight fetches to complete (max 30s)
	if components.WorkerPool != nil {
		fetchDone := make(chan struct{})
		go func() {
			components.WorkerPool.Drain()
			close(fetchDone)
		}()

		select {
		case <-fetchDone:
			logger.Info("fetchers_drained")
		case <-time.After(30 * time.Second):
			logger.Warn("fetch_drain_timeout_forcing_close")
		}
	}

	// STEP 3: Wait for parser pool to drain
	if components.ParserPool != nil {
		components.ParserPool.Shutdown()
		logger.Info("parsers_drained")
	}

	// STEP 4: Flush StreamWriter to Redis
	if components.StreamWriter != nil {
		if err := components.StreamWriter.Flush(shutdownCtx); err != nil {
			logger.Error("stream_flush_failed", "error", err)
		} else {
			logger.Info("redis_stream_flushed")
		}
	}

	// STEP 5: Flush local disk fallback
	if components.DiskFallback != nil {
		if err := components.DiskFallback.Flush(shutdownCtx); err != nil {
			logger.Error("disk_fallback_flush_failed", "error", err)
		} else {
			logger.Info("disk_fallback_flushed")
		}
	}

	// STEP 6: Persist dedup filter (no-op for RedisBloom)
	if components.DedupFilter != nil {
		if err := components.DedupFilter.Persist(shutdownCtx); err != nil {
			logger.Error("dedup_persist_failed", "error", err)
		} else {
			logger.Info("dedup_filter_persisted")
		}
	}

	// STEP 7: Close Redis connections
	if components.Redis != nil {
		if err := components.Redis.Close(); err != nil {
			logger.Error("redis_close_failed", "error", err)
		}
	}

	logger.Info("shutdown_complete")
}
