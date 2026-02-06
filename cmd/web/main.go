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
		jsonError(w, "No search results found for: "+req.Query, http.StatusNotFound)
		return
	}

	results := scrapeSearchResults(r.Context(), searchResults)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":          req.Query,
		"search_results": searchResults,
		"results":        results,
		"count":          len(results),
	})
}

func searchDuckDuckGo(
	ctx context.
		Context, query string, count int) ([]SearchResult, error) {
	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", antidetect.Pick())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DuckDuckGo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("DuckDuckGo returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse DuckDuckGo HTML: %w", err)
	}

	var results []SearchResult
	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if len(results) >= count {
			return
		}

		title := strings.TrimSpace(s.Find(".result__title .result__a").Text())
		href, _ := s.Find(".result__title .result__a").Attr("href")
		snippet := strings.TrimSpace(s.Find(".result__snippet").Text())

		actualURL := extractDDGURL(href)
		if actualURL == "" {
			actualURL = href
		}

		if actualURL != "" && title != "" {
			results = append(results, SearchResult{
				Title:   title,
				URL:     actualURL,
				Snippet: snippet,
			})
		}
	})

	return results, nil
}

func searchGoogle(ctx context.
	Context, query string, count int) ([]SearchResult, error) {
	searchURL := "https://www.google.com/search?q=" + url.QueryEscape(query) + "&num=" + fmt.Sprintf("%d", count)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", antidetect.Pick())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	doc.Find("div.g").Each(func(i int, s *goquery.Selection) {
		if len(results) >= count {
			return
		}

		title := strings.TrimSpace(s.Find("h3").First().Text())
		href, _ := s.Find("a").First().Attr("href")
		snippet := strings.TrimSpace(s.Find(".VwiC3b").Text())

		if href != "" && title != "" && strings.HasPrefix(href, "http") {
			results = append(results, SearchResult{
				Title:   title,
				URL:     href,
				Snippet: snippet,
			})
		}
	})

	return results, nil
}

func extractDDGURL(href string) string {
	if href == "" {
		return ""
	}

	if strings.Contains(href, "uddg=") {
		parsed, err := url.Parse(href)
		if err != nil {
			return href
		}
		uddg := parsed.Query().Get("uddg")
		if uddg != "" {
			decoded, err := url.QueryUnescape(uddg)
			if err == nil {
				return decoded
			}
			return uddg
		}
	}

	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}

	return ""
}

func scrapeSearchResults(ctx context.Context, searchResults []SearchResult) []PageResult {
	results := make([]PageResult, 0, len(searchResults))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	for _, sr := range searchResults {
		wg.Add(1)
		go func(s SearchResult) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := fetchAndParse(ctx, s.URL)
			result.Snippet = s.Snippet

			if result.Title == "" || strings.HasPrefix(result.Title, "Error:") || strings.HasPrefix(result.Title, "Fetch error:") {
				result.Title = s.Title
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(sr)
	}

	wg.Wait()
	return results
}

func handleQuickScrape(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL   string `json:"url"`
		Depth int    `json:"depth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		jsonError(w, "url is required", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		req.URL = "https://" + req.URL
	}

	if req.Depth < 1 {
		req.Depth = 1
	}
	if req.Depth > 3 {
		req.Depth = 3
	}

	results := scrapeURL(r.Context(), req.URL, req.Depth)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"url":     req.URL,
		"results": results,
		"count":   len(results),
	})
}

func scrapeURL(ctx context.
	Context, targetURL string, depth int) []PageResult {
	visited := &sync.Map{}
	results := make([]PageResult, 0)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)

	var crawl func(url string, d int)
	crawl = func(u string, d int) {
		defer wg.Done()
		if d <= 0 {
			return
		}
		if _, loaded := visited.LoadOrStore(u, true); loaded {
			return
		}

		sem <- struct{}{}
		defer func() { <-sem }()

		result := fetchAndParse(ctx, u)
		mu.Lock()
		results = append(results, result)
		mu.Unlock()

		if d > 1 && result.StatusCode >= 200 && result.StatusCode < 400 {
			for _, link := range result.Links {
				if len(results) >= 50 {
					break
				}
				absLink := resolveLink(u, link)
				if absLink == "" || !strings.HasPrefix(absLink, "http") {
					continue
				}
				if extractDomain(absLink) != extractDomain(u) {
					continue
				}
				wg.Add(1)
				go crawl(absLink, d-1)
			}
		}
	}

	wg.Add(1)
	go crawl(targetURL, depth)
	wg.Wait()
	return results
}

func fetchAndParse(ctx context.Context, pageURL string) PageResult {
	start := time.Now()

	result := PageResult{
		URL:       pageURL,
		ScrapedAt: time.Now().Format(time.RFC3339),
		Extras:    make(map[string]string),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		result.StatusCode = 0
		result.Title = "Error: " + err.Error()
		return result
	}

	req.Header.Set("User-Agent", antidetect.Pick())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := httpClient.Do(req)
	if err != nil {
		result.StatusCode = 0
		result.Title = "Fetch error: " + err.Error()
		result.FetchTimeMs = time.Since(start).Milliseconds()
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.FetchTimeMs = time.Since(start).Milliseconds()

	if resp.StatusCode >= 400 {
		result.Title = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return result
	}

	body := io.LimitReader(resp.Body, 5*1024*1024)
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		result.Title = "Parse error: " + err.Error()
		return result
	}

	result.Title = strings.TrimSpace(doc.Find("title").First().Text())

	doc.Find("meta[name='description']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists {
			result.MetaDesc = content
		}
	})

	doc.Find("meta[property]").Each(func(i int, s *goquery.Selection) {
		prop, _ := s.Attr("property")
		content, _ := s.Attr("content")
		if content != "" && (strings.HasPrefix(prop, "og:") || strings.HasPrefix(prop, "product:")) {
			result.Extras[prop] = content
		}
	})

	doc.Find("h1, h2, h3").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" && len(result.Headings) < 20 {
			result.Headings = append(result.Headings, text)
		}
	})

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && href != "" && !strings.HasPrefix(href, "#") && !strings.HasPrefix(href, "javascript:") {
			if len(result.Links) < 100 {
				result.Links = append(result.Links, href)
			}
		}
	})

	doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if exists && src != "" && len(result.Images) < 30 {
			result.Images = append(result.Images, src)
		}
	})

	cleanText := extractCleanText(doc)
	result.ContentLen = len(cleanText)
	if len(cleanText) > 50*1024 {
		result.Content = cleanText[:50*1024]
	} else {
		result.Content = cleanText
	}

	result.WordCount = len(strings.Fields(cleanText))

	doc.Find("[class*='price'], [itemprop='price'], .a-price-whole, .product-price").Each(func(i int, s *goquery.Selection) {
		price := strings.TrimSpace(s.Text())
		if price != "" && result.Extras["price"] == "" {
			result.Extras["price"] = price
		}
	})

	doc.Find("[class*='rating'], [itemprop='ratingValue']").Each(func(i int, s *goquery.Selection) {
		rating := strings.TrimSpace(s.Text())
		if rating != "" && len(rating) < 20 && result.Extras["rating"] == "" {
			result.Extras["rating"] = rating
		}
	})

	return result
}

var collapseNL = regexp.MustCompile(`\n{3,}`)
var collapseSP = regexp.MustCompile(`[^\S\n]{2,}`)

func extractCleanText(doc *goquery.
	Document) string {

	clone, _ := goquery.NewDocumentFromReader(strings.NewReader(""))
	clone.Selection = doc.Selection.Clone()

	clone.Find("script, style, nav, footer, header, aside, noscript, [role='complementary'], [role='navigation'], svg, iframe").Remove()

	var textSrc *goquery.Selection
	if article := clone.Find("article"); article.Length() > 0 {
		textSrc = article.First()
	} else if main := clone.Find("main"); main.Length() > 0 {
		textSrc = main.First()
	} else {
		textSrc = clone.Find("body")
	}

	raw := textSrc.Text()

	raw = collapseSP.ReplaceAllString(raw, " ")
	raw = collapseNL.ReplaceAllString(raw, "\n\n")
	raw = strings.TrimSpace(raw)

	return raw
}

func handleMCPTools(w http.
	ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}

	tools := []map[string]interface{}{
		{
			"name":        "search",
			"description": "Search the web by keyword and scrape each result page. Returns structured data including full page text, metadata, prices, ratings, links, images, and headings.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query (keyword, product name, topic, etc.)",
					},
					"count": map[string]interface{}{
						"type":        "integer",
						"description": "Number of result pages to scrape (1-15, default 10)",
						"default":     10,
						"minimum":     1,
						"maximum":     15,
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "scrape",
			"description": "Scrape a specific URL and optionally follow links to the given depth. Returns full page text, metadata, links, images, and headings.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The URL to scrape",
					},
					"depth": map[string]interface{}{
						"type":        "integer",
						"description": "How many levels of links to follow (1-3, default 1)",
						"default":     1,
						"minimum":     1,
						"maximum":     3,
					},
				},
				"required": []string{"url"},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tools": tools,
	})
}
func handleMCPCall(w http.ResponseWriter,
	r *http.
		Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Tool      string          `json:"tool"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch req.Tool {
	case "search":
		var args struct {
			Query string `json:"query"`
			Count int    `json:"count"`
		}
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			jsonError(w, "invalid arguments for search: "+err.Error(), http.StatusBadRequest)
			return
		}
		if args.Query == "" {
			jsonError(w, "query is required", http.StatusBadRequest)
			return
		}
		if args.Count < 1 {
			args.Count = 10
		}
		if args.Count > 15 {
			args.Count = 15
		}

		logger.Info("mcp search", "query", args.Query, "count", args.Count)

		searchResults, err := searchDuckDuckGo(r.Context(), args.Query, args.Count)
		if err != nil {
			searchResults, err = searchGoogle(r.Context(), args.Query, args.Count)
			if err != nil {
				jsonError(w, "search failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		results := scrapeSearchResults(r.Context(), searchResults)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tool":    "search",
			"query":   args.Query,
			"results": results,
			"count":   len(results),
		})

	case "scrape":
		var args struct {
			URL   string `json:"url"`
			Depth int    `json:"depth"`
		}
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			jsonError(w, "invalid arguments for scrape: "+err.Error(), http.StatusBadRequest)
			return
		}
		if args.URL == "" {
			jsonError(w, "url is required", http.StatusBadRequest)
			return
		}
		if !strings.HasPrefix(args.URL, "http://") && !strings.HasPrefix(args.URL, "https://") {
			args.URL = "https://" + args.URL
		}
		if args.Depth < 1 {
			args.Depth = 1
		}
		if args.Depth > 3 {
			args.Depth = 3
		}

		logger.Info("mcp scrape", "url", args.URL, "depth", args.Depth)

		results := scrapeURL(r.Context(), args.URL, args.Depth)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tool":    "scrape",
			"url":     args.URL,
			"results": results,
			"count":   len(results),
		})

	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": fmt.Sprintf("unknown tool: %q — available tools: search, scrape", req.Tool),
			"code":  "unknown_tool",
		})
	}
}
func extractDomain(rawURL string) string {
	parts := strings.SplitN(rawURL, "://", 2)
	if len(parts) < 2 {
		return ""
	}
	host := strings.SplitN(parts[1], "/", 2)[0]
	host = strings.SplitN(host, ":", 2)[0]
	return strings.ToLower(host)
