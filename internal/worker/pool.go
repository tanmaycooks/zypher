package worker

import (
	"bytes"
	"context"
	"fmt"
	"github.com/anand/webscrapper/internal/antidetect"
	"github.com/anand/webscrapper/internal/breaker"
	"github.com/anand/webscrapper/internal/compression"
	"github.com/anand/webscrapper/internal/dedup"

	"github.com/anand/webscrapper/internal/frontier"

	"github.com/anand/webscrapper/internal/limiter"

	"github.com/anand/webscrapper/internal/metrics"

	"github.com/anand/webscrapper/internal/parser"
	"github.com/anand/webscrapper/internal/proxy"
	"github.com/anand/webscrapper/internal/robots"
	"github.com/anand/webscrapper/internal/scheduler"
	"github.com/anand/webscrapper/internal/stream"
	"github.com/cespare/xxhash/v2"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Pool struct {
	frontier *frontier.
			Frontier
	dedup       *dedup.DistributedFilter
	limiter     *limiter.AdaptiveLimiter
	proxyPool   *proxy.Pool
	robotsCache *robots.Cache
	parser      *parser.Dispatcher
	writer      *stream.Writer

	tracker    *scheduler.ChangeTracker
	httpClient *http.Client
	breakersMu sync.RWMutex
	breakers   map[ // Package worker provides the core worker pool that orchestrates the fetch pipeline.

	// It ties together all subsystems: frontier, dedup, DNS, proxy, limiter, breaker,

	// parser, stream writer, and metrics.
	// Pool manages a pool of fetch workers.

	// Per-domain circuit breakers
	// PoolConfig holds worker pool configuration.
	// DefaultPoolConfig returns sane defaults.

	// NewPool creates a new worker pool wiring all subsystems together.

	// Run starts the worker pool. It continuously pops URLs from the frontier

	// and dispatches them to workers. Stops when context is cancelled.

	// Pop a batch of URLs from the frontier
	// Dispatch each URL to a worker goroutine
	// processURL is the per-URL fetch pipeline.

	// Step 1: Check dedup filter

	// already crawled

	// Step 2: Check robots.txt
	// Step 3: Check circuit breaker
	// domain circuit is open

	// Step 4: Acquire AIMD concurrency slot
	// Step 5: Fetch URL
	// Step 6: Mark as seen in dedup filter

	// Step 7: Parse response
	// Step 8: Compute content hash for change detection

	// Step 9: Write to stream

	// Step 10: Push discovered links back to frontier

	// fetch performs an HTTP GET request with anti-detection.

	// Apply anti-detection headers
	// Handle compression
	// getBreaker returns or creates a circuit breaker for a domain.

	// Drain waits for all in-flight requests to complete.

	// statusClass returns "2xx", "3xx", "4xx", or "5xx" for the response.

	// extractDomain extracts the domain from a URL.

	// resolveLink resolves a relative URL against a base URL.

	string]*breaker.CircuitBreaker
	batchSize int
	logger    *slog.Logger
	wg        sync.WaitGroup
}
type PoolConfig struct {
	BatchSize  int
	MinConcur  int64
	MaxConcur  int64
	ProxyAddrs []string
}

func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		BatchSize: 100,
		MinConcur: 10,
		MaxConcur: 500,
	}
}

func NewPool(cfg PoolConfig, f *frontier.Frontier, d *dedup.DistributedFilter, rc *robots.
	Cache, p *parser.Dispatcher,

	w *stream.Writer, ct *scheduler.ChangeTracker, client *http.Client, logger *slog.Logger) *Pool {
	if logger == nil {
		logger = slog.Default()
	}

	return &Pool{
		frontier:    f,
		dedup:       d,
		limiter:     limiter.NewAdaptiveLimiter(cfg.MinConcur, cfg.MaxConcur),
		proxyPool:   proxy.NewPool(cfg.ProxyAddrs),
		robotsCache: rc,
		parser:      p,
		writer:      w,
		tracker:     ct,
		httpClient:  client,
		breakers:    make(map[string]*breaker.CircuitBreaker),
		batchSize:   cfg.BatchSize,
		logger:      logger,
	}
}

func (p *Pool) Run(ctx context.
	Context) {
	panic("not implemented")

}
func (p *Pool) processURL(ctx context.Context, rawURL string) {
	panic("not implemented")

}
func (p *Pool) fetch(ctx cont