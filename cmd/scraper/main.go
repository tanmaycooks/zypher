package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/anand/webscrapper/internal/dedup"
	"github.com/anand/webscrapper/internal/dns"
	"github.com/anand/webscrapper/internal/frontier"

	"github.com/anand/webscrapper/internal/parser"

	"github.com/anand/webscrapper/internal/robots"
	"github.com/anand/webscrapper/internal/scheduler"

	"github.com/anand/webscrapper/internal/shutdown"
	"github.com/anand/webscrapper/internal/stream"
	"github.com/anand/webscrapper/internal/transport"
	"github.com/anand/webscrapper/internal/worker"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {

	redisAddr := flag.String("redis", "localhost:6379", "Redis address")
	redisPassword := flag.String("redis-password", "", "Redis password")
	seedURLs := flag.String("seeds", "", "Comma-separated seed URLs")
	metricsPort := flag.Int("metrics-port", 9090, "Prometheus metrics port")
	minConcur := flag.Int64("min-concur", 10, "Minimum concurrency")
	maxConcur := flag.Int64("max-concur", 500, "Maximum concurrency")
	batchSize := flag.Int("batch-size", 100, "Frontier batch size")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting web scraper",
		"redis", *redisAddr,
		"min_concur", *minConcur,
		"max_concur", *maxConcur,
		"batch_size", *batchSize,
	)

	rdb := redis.NewClient(&redis.Options{
		Addr:         *redisAddr,
		Password:     *redisPassword,
		DB:           0,
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("failed to connect to Redis", "error", err)
		os.Exit( // Command scraper is the entry point for the high-performance web scraper.
			1)
	}
	logger.Info("redis connected", "addr", *redisAddr)

	// It wires all subsystems together and starts the pipeline.

	// CLI flags
	// Logger
	// Redis client
	// Verify Redis connection
	// DNS Cache (B-02)
	// HTTP Transport with DNS cache
	// Subsystems
	// Worker pool
	// Seed URLs
	// Prometheus metrics endpoint
	// Graceful shutdown in background
	// Run the worker pool (blocks until context is cancelled)

	// parseSeeds splits comma-separated seed URLs.
	dnsCache := dns.New(5 * time.Minute)

	httpTransport := transport.NewHTTPTransport(dnsCache)

	httpClient := &http.Client{
		Transport: httpTransport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	dedupFilter := dedup.NewDistributedFilter(rdb, logger)
	front := frontier.NewFrontier(rdb)
	robotsCache := robots.NewCache(24*time.Hour, logger)
	parserDispatch := parser.NewDispatcher(nil)
	streamWriter := stream.NewWriter(rdb, "scraper:output", 100, logger)
	changeTracker := scheduler.NewChangeTracker(rdb, logger)

	pool := worker.NewPool(
		worker.PoolConfig{
			BatchSize: *batchSize,
			MinConcur: *minConcur,
			MaxConcur: *maxConcur,
		},
		front,
		dedupFilter,
		robotsCache,
		parserDispatch,
		streamWriter,
		changeTracker,
		httpClient,
		logger,
	)

	if *seedURLs != "" {
		seeds := parseSeeds(*seedURLs)
		for _, seed := range seeds {
			if err := front.Push(ctx, seed, 1.0, time.Time{}); err != nil {
				logger.Warn("failed to seed URL", "url", seed, "error", err)
			}
		}
		logger.Info("seeded frontier", "count", len(seeds))
	}

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
		addr := fmt.Sprintf(":%d", *metricsPort)
		logger.Info("metrics server starting", "addr", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			logger.Error("metrics server failed", "error", err)
		}
	}()

	go shutdown.GracefulShutdown(&shutdown.Components{
		Cancel:       cancel,
		WorkerPool:   pool,
		StreamWriter: streamWriter,
		DedupFilter:  dedupFilter,
		Redis:        rdb,
	}, logger)

	pool.Run(ctx)

	logger.Info("scraper shutdown complete")
}
func parseSeeds(input string) []string {
	seeds := make([]string, 0)
	