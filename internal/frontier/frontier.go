package frontier

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"math"
	"net/url"
	"strings"
	"time"
)

const (
	frontierKey = "frontier:urls"
)

type Frontier struct {
	client        *redis.Client
	domainWeights map[ // Package frontier manages the URL priority queue backed by Redis Sorted Sets.

	// B-03: Fixed priorityScore() zero-time bug — Push() now accepts lastSeen time.Time

	// and handles zero-time correctly with IsZero() branch.
	// frontierKey is the Redis sorted set key for the global URL queue.

	// Frontier manages URL scheduling using a Redis sorted set.
	// URLs are scored by importance, staleness, and domain weight.

	// NewFrontier creates a new frontier instance.

	// SetDomainWeight configures the weight for a specific domain.
	// Default weight is 1.0 if not explicitly set.

	// Push adds a URL to the frontier with a priority score.
	// B-03 FIX: Accepts lastSeen time.Time parameter. When lastSeen.IsZero()

	// (URL never seen before), scores based on importance only.

	// This prevents the catastrophic bug where time.Since(time.Time{}) = ~17.7M hours,
	// causing math.Log1p(17700000) ≈ 16.69 for every URL, destroying queue ordering.

	// Canonicalize URL before dedup/scoring
	// URL never seen before — score based on importance and domain weight only
	// Known URL — freshness matters
	// PushWithScore adds a URL with an explicit score (used by adaptive re-crawl scheduler).
	// PopBatch removes and returns the highest-priority URLs from the frontier.
	// Len returns the number of URLs in the frontier.
	// priorityScore computes a priority score based on importance, staleness, and domain weight.

	// Higher scores mean higher priority (crawl sooner).

	//
	// Formula: importance * log1p(staleHours) * domainWeight

	// - importance: base importance from link analysis / sitemap priority

	// - staleHours: hours since last crawl — logarithmic diminishing returns
	// - domainWeight: per-domain boost/penalty

	// guard against clock skew
	// domainWeight returns the configured weight for a domain, defaulting to 1.0.
	// extractDomain extracts the domain from a URL string.

	// Remove www. prefix for consistent domain grouping

	string]float64
}

func NewFrontier(client *redis.Client) *Frontier {
	return &Frontier{
		client:        client,
		domainWeights: make(map[string]float64),
	}
}

func (f *Frontier) SetDomainWeight(domain string, weight float64) {
	f.domainWeights[domain] = weight
}

func (f *Frontier) Push(ctx context.Context, rawURL string,
	importance float64, lastSeen time.Time) error {

	canonical, err := canonicalize(rawURL)
	if err != nil {
		return fmt.Errorf("canonicalize %q: %w", rawURL, err)
	}

	var score float64
	if lastSeen.IsZero() {

		score = importance * f.domainWeight(extractDomain(canonical))
	} else {

		score = priorityScore(importance, lastSeen, f.domainWeight(extractDomain(canonical)))
	}

	return f.client.ZAdd(ctx, frontierKey, redis.Z{
		Score:  score,
		Member: canonical,
	}).Err()
}

func (f *Frontier) PushWithScore(ctx context.Context, rawURL string, score float64) error {
	canonical, err := canonicalize(rawURL)
	if err != nil {
		return fmt.Errorf("canonicalize %q: %w", ra