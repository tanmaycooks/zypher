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
		Level: slog.LevelIn