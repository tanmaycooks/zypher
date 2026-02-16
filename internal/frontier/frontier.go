// Package frontier manages the URL priority queue backed by Redis Sorted Sets.
// B-03: Fixed priorityScore() zero-time bug — Push() now accepts lastSeen time.Time
// and handles zero-time correctly with IsZero() branch.
package frontier

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// frontierKey is the Redis sorted set key for the global URL queue.
	frontierKey = "frontier:urls"
)

// Frontier manages URL scheduling using a Redis sorted set.
// URLs are scored by importance, staleness, and domain weight.
type Frontier struct {
	client        *redis.Client
	domainWeights map[string]float64
}

// NewFrontier creates a new frontier instance.
func NewFrontier(client *redis.Client) *Frontier {
	return &Frontier{
		client:        client,
		domainWeights: make(map[string]float64),
	}
}

// SetDomainWeight configures the weight for a specific domain.
// Default weight is 1.0 if not explicitly set.
func (f *Frontier) SetDomainWeight(domain string, weight float64) {
	f.domainWeights[domain] = weight
}

// Push adds a URL to the frontier with a priority score.
// B-03 FIX: Accepts lastSeen time.Time parameter. When lastSeen.IsZero()
// (URL never seen before), scores based on importance only.
// This prevents the catastrophic bug where time.Since(time.Time{}) = ~17.7M hours,
// causing math.Log1p(17700000) ≈ 16.69 for every URL, destroying queue ordering.
func (f *Frontier) Push(ctx context.Context, rawURL string, importance float64, lastSeen time.Time) error {
	// Canonicalize URL before dedup/scoring
	canonical, err := canonicalize(rawURL)
	if err != nil {
		return fmt.Errorf("canonicalize %q: %w", rawURL, err)
	}

	var score float64
	if lastSeen.IsZero() {
		// URL never seen before — score based on importance and domain weight only
		score = importance * f.domainWeight(extractDomain(canonical))
	} else {
		// Known URL — freshness matters
		score = priorityScore(importance, lastSeen, f.domainWeight(extractDomain(canonical)))
	}

	return f.client.ZAdd(ctx, frontierKey, redis.Z{
		Score:  score,
		Member: canonical,
	}).Err()
}

// PushWithScore adds a URL with an explicit score (used by adaptive re-crawl scheduler).
func (f *Frontier) PushWithScore(ctx context.Context, rawURL string, score float64) error {
	canonical, err := canonicalize(rawURL)
	if err != nil {
		return fmt.Errorf("canonicalize %q: %w", rawURL, err)
	}

	return f.client.ZAdd(ctx, frontierKey, redis.Z{
		Score:  score,
		Member: canonical,
	}).Err()
}

// PopBatch removes and returns the highest-priority URLs from the frontier.
func (f *Frontier) PopBatch(ctx context.Context, count int64) ([]string, error) {
	results, err := f.client.ZPopMax(ctx, frontierKey, count).Result()
	if err != nil {
		return nil, fmt.Errorf("pop batch: %w", err)
	}

	urls := make([]string, 0, len(results))
	for _, z := range results {
		if s, ok := z.Member.(string); ok {
			urls = append(urls, s)
		}
	}
	return urls, nil
}

// Len returns the number of URLs in the frontier.
func (f *Frontier) Len(ctx context.Context) (int64, error) {
	return f.client.ZCard(ctx, frontierKey).Result()
}

// priorityScore computes a priority score based on importance, staleness, and domain weight.
// Higher scores mean higher priority (crawl sooner).
//
// Formula: importance * log1p(staleHours) * domainWeight
// - importance: base importance from link analysis / sitemap priority
// - staleHours: hours since last crawl — logarithmic diminishing returns
// - domainWeight: per-domain boost/penalty
func priorityScore(importance float64, lastSeen time.Time, weight float64) float64 {
	staleHours := time.Since(lastSeen).Hours()
	if staleHours < 0 {
		staleHours = 0 // guard against clock skew
	}
	return importance * math.Log1p(staleHours) * weight
}

// domainWeight returns the configured weight for a domain, defaulting to 1.0.
func (f *Frontier) domainWeight(domain string) float64 {
	if w, ok := f.domainWeights[domain]; ok {
		return w
	}
	return 1.0
}

// extractDomain extracts the domain from a URL string.
func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	// Remove www. prefix for consistent domain grouping
	host = strings.TrimPrefix(host, "www.")
	return strings.ToLower(host)
}
