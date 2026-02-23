package shutdown

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Components struct {
	Cancel     context.CancelFunc
	WorkerPool Drainable
	ParserPool Shutdownable

	StreamWriter Flushable
	DiskFallback Flushable
	DedupFilter  Persistable

	Redis Closeable
}
type Drainable interface{ Drain() }
type Shutdownable interface {
	Shutdown()
}
type Flushable interface {
	Flush(ctx context.
		Context) error
}
type Persistable interface {
	Persist(ctx context.Context) error
}
type Closeable interface{ Close() error }

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

	if components.Cancel != nil {
		components.Cancel()
	}
	logger.Info("frontier_stopped")

	if // Package shutdown provides ordered graceful shutdown for the scraper pipeline.

	// E-13: Implements a 7-step shutdown sequence ensuring zero data loss on deployment restarts.

	// Components aggregates all scraper component references for shutdown coordination.

	// Drainable can wait for in-flight operations to complete.

	// Shutdownable can be shut down gracefully.
	// Flushable can flush buffered data.
	// Persistable can persist state.
	// Closeable can be closed.
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

	// STEP 1: Stop frontier popping — no new URLs dispatched
	// STEP 2: Wait for all in-flight fetches to complete (max 30s)
	// STEP 3: Wait for parser pool to drain
	// STEP 4: Flush StreamWriter to Redis
	// STEP 5: Flush local disk fallback
	// STEP 6: Persist dedup filter (no-op for RedisBloom)

	// STEP 7: Close Redis connections
	components.WorkerPool != nil {
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

	if components.ParserPool != nil {
		components.ParserPool.Shutdown()
		logger.Info("parsers_drained")
	}

	if components.StreamWriter != nil {
		if err := components.StreamWriter.Flush(shutdownCtx); err != nil {
			logger.Error("stream_flush_failed", "error", err)
		} else {
			logger.Info("redis_stream_flushed")
		}
	}

	if components.DiskFallback != nil {
		if err := components.DiskFallback.Flush(shutdownCtx); err != nil {
			logger.Error("disk_fallback_flush_failed", "error", err)
		} else {
			logger.Info("disk_fallback_flushed")
		}
	}

	if components.DedupFilter != nil {
		if err := components.DedupFilter.Persist(shutdownCtx); err != nil {
			logger.Error("dedup_persist_failed", "error", err)
		} else {
			logger.Info("dedup_filter_persisted")
		}
	}

	if components.Redis != nil {
		if err := components.Redis.Close(); err != nil {
			logger.Error("redis_close_failed", "error", err)
		}
	}

	logger.Info("shutdown_complete")
}
