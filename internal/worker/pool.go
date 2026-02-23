// Package worker provides the core worker pool that orchestrates the fetch pipeline.
// It ties together all subsystems: frontier, dedup, DNS, proxy, limiter, breaker,
// parser, stream writer, and metrics.
package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"

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
)

// Pool manages a pool of fetch workers.
type Pool struct {
	frontier    *frontier.Frontier
	dedup       *dedup.DistributedFilter
	limiter     *limiter.AdaptiveLimiter
	proxyPool   *proxy.Pool
	robotsCache *robots.Cache
	parser      *parser.Dispatcher
	writer      *stream.Writer
	tracker     *scheduler.ChangeTracker
	httpClient  *http.Client

	// Per-domain circuit breakers
	breakersMu sync.RWMutex
	breakers   map[string]*breaker.CircuitBreaker

	batchSize int
	logger    *slog.Logger
	wg        sync.WaitGroup
}

// PoolConfig holds worker pool configuration.
type PoolConfig struct {
	BatchSize  int
	MinConcur  int64
	MaxConcur  int64
	ProxyAddrs []string
}

// DefaultPoolConfig returns sane defaults.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		BatchSize: 100,
		MinConcur: 10,
		MaxConcur: 500,
	}
}

// NewPool creates a new worker pool wiring all subsystems together.
func NewPool(
	cfg PoolConfig,
	f *frontier.Frontier,
	d *dedup.DistributedFilter,
	rc *robots.Cache,
	p *parser.Dispatcher,
	w *stream.Writer,
	ct *scheduler.ChangeTracker,
	client *http.Client,
	logger *slog.Logger,
) *Pool {
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

// Run starts the worker pool. It continuously pops URLs from the frontier
// and dispatches them to workers. Stops when context is cancelled.
func (p *Pool) Run(ctx context.Context) {
	p.logger.Info("worker pool started",
		"min_concur", p.limiter.Ceiling(),
		"batch_size", p.batchSize)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("worker pool stopping, draining in-flight requests")
			p.wg.Wait()
			return
		default:
		}

		// Pop a batch of URLs from the frontier
		urls, err := p.frontier.PopBatch(ctx, int64(p.batchSize))
		if err != nil {
			p.logger.Error("frontier pop failed", "error", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if len(urls) == 0 {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		metrics.FrontierSize.Set(float64(len(urls)))

		// Dispatch each URL to a worker goroutine
		for _, url := range urls {
			p.wg.Add(1)
			go func(u string) {
				defer p.wg.Done()
				p.processURL(ctx, u)
			}(url)
		}
	}
}

// processURL is the per-URL fetch pipeline.
func (p *Pool) processURL(ctx context.Context, rawURL string) {
	domain := extractDomain(rawURL)

	// Step 1: Check dedup filter
	seen, err := p.dedup.Contains(ctx, rawURL)
	if err != nil {
		p.logger.Warn("dedup check failed", "url", rawURL, "error", err)
	}
	if seen {
		return // already crawled
	}

	// Step 2: Check robots.txt
	if !p.robotsCache.IsAllowed(domain, rawURL, "ScraperBot/1.0") {
		metrics.RobotsDisallowRatio.WithLabelValues(domain).Inc()
		return
	}

	// Step 3: Check circuit breaker
	cb := p.getBreaker(domain)
	if !cb.Allow() {
		return // domain circuit is open
	}

	// Step 4: Acquire AIMD concurrency slot
	p.limiter.Acquire()
	defer p.limiter.Release()

	metrics.ActiveGoroutines.WithLabelValues("in_flight").Set(float64(p.limiter.InFlight()))
	metrics.ActiveGoroutines.WithLabelValues("ceiling").Set(float64(p.limiter.Ceiling()))

	// Step 5: Fetch URL
	start := time.Now()
	resp, fetchErr := p.fetch(ctx, rawURL)
	duration := time.Since(start)

	metrics.FetchDuration.WithLabelValues(domain,
		statusClass(resp)).Observe(duration.Seconds())

	if fetchErr != nil {
		cb.Record(false)
		p.limiter.OnFailure()
		p.logger.Warn("fetch failed", "url", rawURL, "error", fetchErr)
		return
	}

	cb.Record(true)
	p.limiter.OnSuccess()

	// Step 6: Mark as seen in dedup filter
	if err := p.dedup.Insert(ctx, rawURL); err != nil {
		p.logger.Warn("dedup insert failed", "url", rawURL, "error", err)
	}

	// Step 7: Parse response
	contentType := parser.DetectContentType(resp)
	parsed, err := p.parser.Dispatch(contentType, resp.Body, rawURL)
	resp.Body.Close()

	if err != nil {
		p.logger.Warn("parse failed", "url", rawURL, "error", err)
		return
	}

	// Step 8: Compute content hash for change detection
	contentHash := fmt.Sprintf("%x", xxhash.Sum64String(parsed.Text))

	// Step 9: Write to stream
	writeErr := p.writer.Write(ctx, stream.Record{
		URL:         rawURL,
		Domain:      domain,
		ContentHash: contentHash,
		StatusCode:  resp.StatusCode,
		FetchedAt:   time.Now(),
		Fields:      parsed.Fields,
	})
	if writeErr != nil {
		p.logger.Error("stream write failed", "url", rawURL, "error", writeErr)
	}

	// Step 10: Push discovered links back to frontier
	for _, link := range parsed.Links {
		absLink := resolveLink(rawURL, link)
		if absLink != "" {
			p.frontier.Push(ctx, absLink, 0.5, time.Time{})
		}
	}

	metrics.PagesPerMinute.Inc()
}

// fetch performs an HTTP GET request with anti-detection.
func (p *Pool) fetch(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	// Apply anti-detection headers
	req.Header.Set("User-Agent", antidetect.Pick())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, br, deflate")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Handle compression
	encoding := resp.Header.Get("Content-Encoding")
	if encoding != "" && encoding != "identity" {
		decompressed, err := compression.Decompress(encoding, resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decompress: %w", err)
		}
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(decompressed))
	}

	return resp, nil
}

// getBreaker returns or creates a circuit breaker for a domain.
func (p *Pool) getBreaker(domain string) *breaker.CircuitBreaker {
	p.breakersMu.RLock()
	cb, ok := p.breakers[domain]
	p.breakersMu.RUnlock()

	if ok {
		return cb
	}

	p.breakersMu.Lock()
	defer p.breakersMu.Unlock()

	if cb, ok := p.breakers[domain]; ok {
		return cb
	}

	cb = breaker.NewCircuitBreaker(5, 30*time.Second)
	p.breakers[domain] = cb
	return cb
}

// Drain waits for all in-flight requests to complete.
func (p *Pool) Drain() {
	p.wg.Wait()
}

// statusClass returns "2xx", "3xx", "4xx", or "5xx" for the response.
func statusClass(resp *http.Response) string {
	if resp == nil {
		return "err"
	}
	switch {
	case resp.StatusCode < 300:
		return "2xx"
	case resp.StatusCode < 400:
		return "3xx"
	case resp.StatusCode < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

// extractDomain extracts the domain from a URL.
func extractDomain(rawURL string) string {
	parts := strings.SplitN(rawURL, "://", 2)
	if len(parts) < 2 {
		return ""
	}
	host := strings.SplitN(parts[1], "/", 2)[0]
	host = strings.SplitN(host, ":", 2)[0]
	return strings.TrimPrefix(strings.ToLower(host), "www.")
}

// resolveLink resolves a relative URL against a base URL.
func resolveLink(base, href string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "/") {
		parts := strings.SplitN(base, "://", 2)
		if len(parts) < 2 {
			return ""
		}
		hostPart := strings.SplitN(parts[1], "/", 2)[0]
		return parts[0] + "://" + hostPart + href
	}
	return ""
}
