// Package metrics provides Prometheus metric definitions for the scraper.
// E-11: Includes robots.txt disallow ratio as early anti-ban signal.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// PagesPerMinute tracks the rate of pages fetched.
	PagesPerMinute = promauto.NewCounter(prometheus.CounterOpts{
		Name: "scraper_pages_total",
		Help: "Total number of pages fetched",
	})

	// FetchDuration tracks fetch latency distribution.
	FetchDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "scraper_fetch_duration_seconds",
		Help:    "Histogram of fetch durations in seconds",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 15),
	}, []string{"domain", "status"})

	// ActiveGoroutines tracks the AIMD ceiling vs actual in-flight.
	ActiveGoroutines = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "scraper_active_goroutines",
		Help: "Current number of active goroutines by type",
	}, []string{"type"}) // type: "ceiling", "in_flight"

	// CircuitBreakerState tracks per-domain circuit breaker states.
	CircuitBreakerState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "scraper_circuit_breaker_state",
		Help: "Circuit breaker state per domain (0=closed, 1=open, 2=half-open)",
	}, []string{"domain"})

	// RedisMemoryUsage tracks Redis memory consumption.
	RedisMemoryUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "scraper_redis_memory_bytes",
		Help: "Redis memory usage in bytes",
	})

	// HTTPStatusCodes tracks response status codes per domain.
	HTTPStatusCodes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "scraper_http_status_total",
		Help: "Total HTTP responses by domain and status code class",
	}, []string{"domain", "status_class"}) // status_class: "2xx", "3xx", "4xx", "5xx"

	// ProxySuccessRate tracks proxy performance.
	ProxySuccessRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "scraper_proxy_success_rate",
		Help: "Success rate per proxy",
	}, []string{"proxy"})

	// GCPauseDuration tracks GC pause times.
	GCPauseDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "scraper_gc_pause_seconds",
		Help:    "GC pause duration histogram",
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 15),
	})

	// RobotsDisallowRatio tracks the ratio of disallowed URLs per domain.
	// E-11: A sudden increase in this ratio is an early warning that the site
	// has detected scraping and added new blocking rules in robots.txt.
	RobotsDisallowRatio = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "scraper_robots_disallow_ratio",
		Help: "Ratio of frontier URLs disallowed by robots.txt per domain (rolling 1h)",
	}, []string{"domain"})

	// FrontierSize tracks the number of URLs in the frontier.
	FrontierSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "scraper_frontier_size",
		Help: "Number of URLs currently in the frontier queue",
	})

	// DedupFilterSize tracks the Cuckoo filter occupancy.
	DedupFilterSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "scraper_dedup_filter_size",
		Help: "Number of URLs in the dedup filter",
	})

	// ContentChangeRate tracks content change detection results.
	ContentChangeRate = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "scraper_content_change_total",
		Help: "Content change detection results",
	}, []string{"result"}) // result: "changed", "unchanged"

	// DNSCacheHitRate tracks DNS cache performance.
	DNSCacheHitRate = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "scraper_dns_cache_total",
		Help: "DNS cache hit/miss counts",
	}, []string{"result"}) // result: "hit", "miss", "stale_revalidate"
)
