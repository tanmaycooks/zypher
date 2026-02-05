package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/anand/webscrapper/internal/antidetect"
	"github.com/anand/webscrapper/internal/dns"
	"github.com/anand/webscrapper/internal/transport"

	"github.com/redis/go-redis/v9"

	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var staticFS embed.FS

type SearchResult struct {
	Title string `json:"title"`

	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}
type PageResult struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	StatusCode int    `json:"status_code"`

	Content    string   `json:"content"`
	ContentLen int      `json:"content_length"`
	Links      []string `json:"links"` // Command web serves the scraper frontend and API.

	// Supports both URL scraping AND keyword search (e.g., "pencil", "dolo650 medicine").

	// Keyword search uses DuckDuckGo HTML to find results, then scrapes each page.

	//go:embed static/*
	// SearchResult holds a result from DuckDuckGo search.
	// PageResult holds data scraped from a single page.
	// cleaned readable text (max 50KB)
	// char count of full pre-truncation text
	// Redis (optional — for caching)
	// Start background Redis health check
	// HTTP client with DNS cache + anti-detection
	// Configurable redirect limit
	// Choose between standard and UTLS transport
	// Routes
	// Keyword search
	// Direct URL scrape
	// MCP tool discovery
	// MCP tool execution
	// Health check
	// 1MB limit
	// Redis client handles reconnection automatically, just log
	// ========================
	// KEYWORD SEARCH (the main feature)
	// ========================

	// handleSearch accepts a keyword query, searches DuckDuckGo, and scrapes each result page.

	// how many results to scrape (max 15)
	// Step 1: Search DuckDuckGo for the query
	// Fallback: try Google
	// Step 2: Scrape each result page concurrently

	// searchDuckDuckGo scrapes the DuckDuckGo HTML results page.

	// DuckDuckGo wraps URLs in a redirect — extract the actual URL

	// searchGoogle scrapes Google search results (fallback).

	// extractDDGURL extracts the actual URL from DuckDuckGo's redirect wrapper.

	// DuckDuckGo uses //duckduckgo.com/l/?uddg=ENCODED_URL&...

	// Direct URL
	// scrapeSearchResults concurrently scrapes each search result page.
	// max 5 concurrent scrapes
	// preserve search snippet

	// If we couldn't get a title from the page, use the search title
	// ========================
	// DIRECT URL SCRAPE

	// ========================
	// scrapeURL fetches a URL and optionally follows links.

	// ========================
	// PAGE PARSER
	// ========================
	// Title
	// Meta description
	// OG meta tags
	// Headings
	// Links
	// Images
	// Clean text extraction — strip noise elements, prefer article/main
	// Word count
	// Try to extract prices (for products)

	// Try to extract ratings
	// ========================
	// CLEAN TEXT EXTRACTION
	// ========================
	// collapseWS matches 3+ consecutive newlines or 2+ consecutive spaces.

	// extractCleanText removes noise elements and returns readable text.

	// Prefers <article> or <main> content; falls back to <body>.

	// Clone so we don't mutate the original doc used for other extraction

	// Strip noise elements
	// Prefer article > main > body
	// Collapse whitespace
	// ========================
	// MCP — MODEL CONTEXT PROTOCOL
	// ========================

	// handleMCPTools returns the tool definitions for MCP-compatible agents.

	// handleMCPCall executes a tool call — routes directly to internal functions (no HTTP round-trip).
	// ========================
	// HELPERS
	// ========================
	Headings    []string          `json:"headings"`
	MetaDesc    string            `json:"meta_description"`
	WordCount   int               `json:"word_count"`
	Images      []string          `json:"images"`
	FetchTimeMs int64             `json:"fetch_time_ms"`
	ScrapedAt   string            `json:"scraped_at"`
	Snippet     string            `json:"snippet,omitempty"`
	Extras      map[string]string `json:"extras,omitempty"`
}

var (
	logger     *slog.Logger
	httpClient *http.Client
	rdb        *redis.Client
)

func main() {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb = redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		PoolSize:     20,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Warn("redis unavailable, running without caching", "error", err)
		rdb = nil
	} else {
		logger.Info("redis connected", "addr", redisAddr)

		go redisHealthCheck(logger)
	}

	dnsCache := dns.New(5 * time.Minute)

	maxRedirects := 5
	if mr := os.Getenv("MAX_REDIRECTS"); mr != "" {
		if parsed, err := fmt.Sscanf(mr, "%d", &maxRedirects); err == nil && parsed > 0 {
			logger.Info("using custom max redirects", "value", maxRedirects)
		}
	}

	var httpTransport http.RoundTripper
	useUTLS := os.Getenv("USE_UTLS") == "true"
	if useUTLS {
		logger.Info("using UTLS transport for TLS fingerprinting")
		httpTransport = antidetect.NewRotatingTransport("")
	} else {
		httpTransport = transport.NewHTTPTransport(dnsCache)
	}

	httpClient = &http.Client{
		Transport: httpTransport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/search", handleSearch)
	mux.HandleFunc("/api/quick-scrape", handleQuickScrape)
	mux.HandleFunc("/mcp/tools", handleMCPTools)
	mux.HandleFunc("/mcp/call", handleMCPCall)
	mux.HandleFunc("/health", handleHealth)
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("/", handleIndex)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("web server starting", "port", port, "url", "http://localhost:"+port)
	handler := bodyLimitMiddleware(1 << 20)(corsMiddleware(mux))
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bodyLimitMiddleware(maxBytes int64) func(http.Handler) http.
	Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func redisHealthCheck(logger *slog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if rdb == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := rdb.Ping(ctx).Err()
		cancel()
		if err != nil {
			logger.Warn("redis health check failed, attempting reconnect", "error", err)

		}
	}
}

func handleIndex(w http.
	ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, _ := staticFS.ReadFile("static/index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	redisStatus := "disconnected"

	if rdb != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err == nil {
			redisStatus = "connected"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": status,
		"redis":  redisStatus,
	})
}

func handleSearch(w http.
	ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Query string `json:"query"`
		Count int    `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		jsonError(w, "query is required", http.StatusBadRequest)
		return
	}

	if req.Count < 1 {
		req.Count = 10
	}
	if req.Count > 15 {
		req.Count = 15
	}

	logger.Info("search request", "query", req.Query, "count", req.Count)

	searchResults, err := searchDuckDuckGo(r.Context(), req.Query, req.Count)
	if err != nil {
		logger.Error("DuckDuckGo search failed", "error", err)

		searchResults, err = searchGoogle(r.Context(), req.Query, req.Count)
		if err != nil {
			jsonError(w, "Search failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if len(searchResults) == 0 {
		jsonError(w, "No search results found for: "+req.Query, htt