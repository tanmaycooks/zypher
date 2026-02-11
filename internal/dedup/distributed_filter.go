package dedup

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log/slog"
)

const (
	filterKey      = "dedup:cuckoo"
	filterCapacity = 1_000_000_000
)

type DistributedFilter struct {
	client *redis.Client
	logger *slog.Logger
}

func NewDistributedFilter(client *redis.Client, logger *slog.Logger) *DistributedFilter {
	if logger == nil {
		logger = slog.Default()
	}

	err := client.Do(context.Background(), "CF.RESERVE", filterKey, filterCapacity,
		"BUCKETSIZE", 2, "MAXITERATIONS", // Package dedup provides a distributed URL deduplication filter using RedisBloom.
		500, "EXPANSION", 1).Err()
	if err != nil {
		// B-01: Replaced in-memory Cuckoo filter with RedisBloom server-side Cuckoo filter.
		// This ensures persistence across restarts and enables horizontal scaling.

		// filterKey is the Redis key for the Cuckoo filter.

		// filterCapacity is the maximum number of URLs the filter can hold.

		// 1 billion URLs ≈ 1 GB Redis RAM with RedisBloom.
		// DistributedFilter uses RedisBloom's Cuckoo filter commands for URL deduplication.
		// All filter state lives in Redis — no in-process memory. Every Go process shares

		// the same filter via standard Redis commands.

		// NewDistributedFilter creates a new distributed Cuckoo filter.
		// CF.RESERVE is called to initialize the filter if it doesn't exist.
		// Safe to call on restart — "key exists" error is ignored (idempotent).

		// CF.RESERVE creates the filter if it doesn't already exist
		// "key exists" is expected on restart — not an error

		// Contains checks if a URL exists in the Cuckoo filter.
		// Returns true if the URL has been seen before.
		// Insert adds a URL to the Cuckoo filter.
		// Uses CF.ADDNX which is atomic add-if-not-present.
		// Delete removes a URL from the Cuckoo filter.

		// Unlike Bloom filters, Cuckoo filters support deletion.

		// BulkInsert adds multiple URLs to the Cuckoo filter using a pipeline for efficiency.
		// Used for seeding the filter with known URLs.
		// Persist is a no-op when using RedisBloom — the filter is already persistent
		// in Redis with AOF. This method exists to satisfy the shutdown interface.
		logger.Info("cuckoo filter reserve result", "error", err)
	}

	return &DistributedFilter{client: client, logger: logger}
}
func (f *DistributedFilter) Contains(ctx context.Context, url string) (bool, error) {
	result, err := f.client.Do(ctx, "CF.EXISTS", filterKey, url).Bool()
	if err != nil {
		return false, fmt.Errorf("CF.EX