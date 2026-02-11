// Package dedup provides a distributed URL deduplication filter using RedisBloom.
// B-01: Replaced in-memory Cuckoo filter with RedisBloom server-side Cuckoo filter.
// This ensures persistence across restarts and enables horizontal scaling.
package dedup

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

const (
	// filterKey is the Redis key for the Cuckoo filter.
	filterKey = "dedup:cuckoo"
	// filterCapacity is the maximum number of URLs the filter can hold.
	// 1 billion URLs ≈ 1 GB Redis RAM with RedisBloom.
	filterCapacity = 1_000_000_000
)

// DistributedFilter uses RedisBloom's Cuckoo filter commands for URL deduplication.
// All filter state lives in Redis — no in-process memory. Every Go process shares
// the same filter via standard Redis commands.
type DistributedFilter struct {
	client *redis.Client
	logger *slog.Logger
}

// NewDistributedFilter creates a new distributed Cuckoo filter.
// CF.RESERVE is called to initialize the filter if it doesn't exist.
// Safe to call on restart — "key exists" error is ignored (idempotent).
func NewDistributedFilter(client *redis.Client, logger *slog.Logger) *DistributedFilter {
	if logger == nil {
		logger = slog.Default()
	}

	// CF.RESERVE creates the filter if it doesn't already exist
	err := client.Do(context.Background(), "CF.RESERVE", filterKey, filterCapacity,
		"BUCKETSIZE", 2, "MAXITERATIONS", 500, "EXPANSION", 1).Err()
	if err != nil {
		// "key exists" is expected on restart — not an error
		logger.Info("cuckoo filter reserve result", "error", err)
	}

	return &DistributedFilter{client: client, logger: logger}
}

// Contains checks if a URL exists in the Cuckoo filter.
// Returns true if the URL has been seen before.
func (f *DistributedFilter) Contains(ctx context.Context, url string) (bool, error) {
	result, err := f.client.Do(ctx, "CF.EXISTS", filterKey, url).Bool()
	if err != nil {
		return false, fmt.Errorf("CF.EXISTS failed: %w", err)
	}
	return result, nil
}

// Insert adds a URL to the Cuckoo filter.
// Uses CF.ADDNX which is atomic add-if-not-present.
func (f *DistributedFilter) Insert(ctx context.Context, url string) error {
	err := f.client.Do(ctx, "CF.ADDNX", filterKey, url).Err()
	if err != nil {
		return fmt.Errorf("CF.ADDNX failed: %w", err)
	}
	return nil
}

// Delete removes a URL from the Cuckoo filter.
// Unlike Bloom filters, Cuckoo filters support deletion.
func (f *DistributedFilter) Delete(ctx context.Context, url string) error {
	err := f.client.Do(ctx, "CF.DEL", filterKey, url).Err()
	if err != nil {
		return fmt.Errorf("CF.DEL failed: %w", err)
	}
	return nil
}

// BulkInsert adds multiple URLs to the Cuckoo filter using a pipeline for efficiency.
// Used for seeding the filter with known URLs.
func (f *DistributedFilter) BulkInsert(ctx context.Context, urls []string) error {
	pipe := f.client.Pipeline()
	for _, url := range urls {
		pipe.Do(ctx, "CF.ADDNX", filterKey, url)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("bulk insert pipeline failed: %w", err)
	}
	return nil
}

// Persist is a no-op when using RedisBloom — the filter is already persistent
// in Redis with AOF. This method exists to satisfy the shutdown interface.
func (f *DistributedFilter) Persist(ctx context.Context) error {
	f.logger.Info("dedup filter persist: no-op (RedisBloom is already persistent)")
	return nil
}
