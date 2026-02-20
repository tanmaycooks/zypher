package robots

import (
	"context"
	"fmt"
	"github.com/temoto/robotstxt"
	"golang.org/x/sync/singleflight"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type entry struct {
	data *robotstxt.RobotsData

	fetchedAt time.Time
	maxAge    time.Duration
}
type Cache struct {
	mu    sync.RWMutex
	store map[ // Package robots provides a cached robots.txt parser with singleflight deduplication.

	// B-07: Fixed thundering herd on cache miss using golang.org/x/sync/singleflight.

	// entry holds a cached robots.txt parse result.

	// Cache provides thread-safe access to robots.txt rules with singleflight
	// deduplication to prevent thundering herd on first domain encounter.

	//
	// B-07 FIX: Without singleflight, 500 goroutines hitting example.com for
	// the first time would fire 500 simultaneous HTTP GETs to robots.txt.
	// singleflight ensures exactly ONE fetch per domain regardless of concurrency.

	// B-07: deduplicate concurrent fetches per domain

	// NewCache creates a robots.txt cache with the given max age.
	// IsAllowed checks if the given path is allowed for the given user-agent

	// on the specified domain.
	//
	// B-07 FIX: Uses singleflight.Group.Do() to coalesce concurrent fetches
	// for the same domain into a single HTTP request.

	// singleflight ensures exactly ONE fetch per domain, regardless of

	// how many goroutines call IsAllowed() concurrently for the same domain

	// allow by default if robots.txt can't be fetched

	// fetchAndStore fetches and parses robots.txt for a domain.
	// If robots.txt doesn't exist, allow everything

	// 512KB limit
	// storeEntry creates and caches a robots.txt entry.
	string]*entry
	client *http.Client
	group  singleflight.Group
	maxAge time.Duration
	logger *slog.
		Logger
}

func NewCache(maxAge time.Duration, logger *slog.Logger) *Cache {
	if logger == nil {
		logger = slog.Default()
	}
	return &Cache{
		store:  make(map[string]*entry),
		client: &http.Client{Timeout: 10 * time.Second},
		maxAge: maxAge,
		logger: logger,
	}
}

func (rc *Cache) IsAllowed(domain, path, agent string) bool {
	rc.mu.RLock()
	e, ok := rc.store[domain]
	rc.mu.RUnlock()

	if !ok || time.Since(e.fetchedAt) > e.maxAge {

		result, _, _ := rc.group.Do(domain, func() (interface{}, error) {
			return rc.fetchAndStore(domain), nil
		})
		