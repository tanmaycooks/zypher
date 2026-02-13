package frontier

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"sort"
	"sync"
	"time"
)

const (
	domainQueuePrefix = "frontier:domain:"
	domainIndexKey    = "frontier:domains"
)

type DomainQueue struct {
	client *redis.
		Client
	logger *slog.
		Logger
	mu sync.
		RWMutex
	budgets map[ // E-04: Domain-isolated priority queues for the frontier.

	// Each domain gets its own Redis sorted set, with a meta-scheduler

	// that allocates crawl budget proportionally to domain queue sizes.
	// DomainQueue provides per-domain URL isolation to prevent a single large
	// domain from starving smaller domains in the global queue.

	// domain → crawl budget per round
	// NewDomainQueue creates a new domain-isolated frontier.

	// Push adds a URL to its domain-specific sorted set.
	// track known domains
	// PopBatch retrieves URLs from multiple domains proportionally.

	// Each domain gets a share of the batch based on its queue size.

	// Get all domains with pending URLs
	// Get queue sizes for budget allocation

	// Allocate budget proportionally
	// Clean up empty domains
	// DomainSizes returns per-domain queue sizes sorted by size (largest first).
	// Useful for monitoring and dashboard display.

	// DomainInfo holds domain queue metrics.

	// SetBudget configures the per-round crawl budget for a domain.

	// CleanupEmptyDomains removes domains with empty queues from the index.
	// ScheduleTick runs one round of the meta-scheduler, dispatching URLs from

	// all domain queues according to budgets.

	// back off when empty
	string]int
}

func NewDomainQueue(client *redis.Client, logger *slog.Logger) *DomainQueue {
	if logger == nil {
		logger = slog.Default()
	}
	return &DomainQueue{
		client:  client,
		logger:  logger,
		budgets: make(map[string]int),
	}
}

func (dq *DomainQueue) Push(ctx context.Context, rawURL string, score float64) error {
	canonical, err := canonicalize(rawURL)
	if err != nil {
		return err
	}

	domain := extractDomain(canonical)
	key := domainQueuePrefix + domain

	pipe := dq.client.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: canonical})
	pipe.SAdd(ctx, domainIndexKey, domain)
	_, err = pipe.Exec(ctx)
	return err
}

func (dq *DomainQueue) PopBatch(ctx context.Context, totalCount int64) ([]string, error) {

	domains, err := dq.client.SMembers(ctx, domainIndexKey).Result()
	if err != nil {
		return nil, err
	}

	if len(domains) == 0 {
		return nil, nil
	}

	type domainSize struct {
		domain string
		size   int64
	}
	sizes := make([]domainSize, 0, len(domains))
	totalSize := int64(0)

	for _, d := range domains {
		size, err := dq.client.ZCard(ctx, domainQueuePrefix+d).Result()
		if err != nil || size == 0 {
			continue
		}
		sizes = append(sizes, domainSize{d, size})
		totalSize += size
	}

	if totalSize == 0 {
		return nil, nil
	}

	urls := make([]string, 0, totalCount)
	pipe := dq.client.Pipeline()

	for _, ds := range sizes {
		budget := int64(float64(ds.size) / float64(totalSize) * float64(totalCount))
		if budget < 1 {
			budget = 1
		}

		results, err := dq.client.ZPopMax(ctx, domainQueuePrefix+ds.domain, budget).Result()
		if err != nil {
			dq.logger.Warn("domain pop failed", "domain", ds.domain, "error", err)
			continue
		}
		for _, z := range results {
			if s, ok := z.Member.(string); ok {
				urls = append(urls, s)
			}
		}
	}

	pipe.Exec(ctx)

	return urls, nil
}

func (dq *DomainQueue) DomainSizes(ctx context.
	Context) ([]DomainInfo, error) {
	domains, err := dq.client.SMembers(ctx, domainIndexKey).Result()
	if err != nil {
		return nil, err
	}

	infos := make([]DomainInfo, 0, len(domains))
	for _, d := range domains {
		size, err := dq.client.ZCard(ctx, domainQueuePrefix+d).Result()
		if err != nil {
			continue
		}
		if size > 0 {
			infos = append(infos, DomainInfo{Domain: d, QueueSize: size})
		}
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].QueueSize > infos[j].QueueSi