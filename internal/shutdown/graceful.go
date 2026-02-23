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

	// 7. Cl