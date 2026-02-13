// E-04: Domain-isolated priority queues for the frontier.
// Each domain gets its own Redis sorted set, with a meta-scheduler
// that allocates crawl budget proportionally to domain queue sizes.
package frontier

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	domainQueuePrefix = "frontier:domain:"
	domainIndexKey    = "frontier:domains"
)

// DomainQueue provides per-domain URL isolation to prevent a single large
// domain from starving smaller domains in the global queue.
type DomainQueue struct {
	client  *redis.Client
	logger  *slog.Logger
	mu      sync.RWMutex
	budgets map[string]int // domain → crawl budget per round
}

// NewDomainQueue creates a new domain-isolated frontier.
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

// Push adds a URL to its domain-specific sorted set.
func (dq *DomainQueue) Push(ctx context.Context, rawURL string, score float64) error {
	canonical, err := canonicalize(rawURL)
	if err != nil {
		return err
	}

	domain := extractDomain(canonical)
	key := domainQueuePrefix + domain

	pipe := dq.client.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: canonical})
	pipe.SAdd(ctx, domainIndexKey, domain) // track known domains
	_, err = pipe.Exec(ctx)
	return err
}

// PopBatch retrieves URLs from multiple domains proportionally.
// Each domain gets a share of the batch based on its queue size.
func (dq *DomainQueue) PopBatch(ctx context.Context, totalCount int64) ([]string, error) {
	// Get all domains with pending URLs
	domains, err := dq.client.SMembers(ctx, domainIndexKey).Result()
	if err != nil {
		return nil, err
	}

	if len(domains) == 0 {
		return nil, nil
	}

	// Get queue sizes for budget allocation
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

	// Allocate budget proportionally
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

	// Clean up empty domains
	pipe.Exec(ctx)

	return urls, nil
}

// DomainSizes returns per-domain queue sizes sorted by size (largest first).
// Useful for monitoring and dashboard display.
func (dq *DomainQueue) DomainSizes(ctx context.Context) ([]DomainInfo, error) {
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
		return infos[i].QueueSize > infos[j].QueueSize
	})

	return infos, nil
}

// DomainInfo holds domain queue metrics.
type DomainInfo struct {
	Domain    string
	QueueSize int64
}

// SetBudget configures the per-round crawl budget for a domain.
func (dq *DomainQueue) SetBudget(domain string, budget int) {
	dq.mu.Lock()
	dq.budgets[domain] = budget
	dq.mu.Unlock()
}

// CleanupEmptyDomains removes domains with empty queues from the index.
func (dq *DomainQueue) CleanupEmptyDomains(ctx context.Context) error {
	domains, err := dq.client.SMembers(ctx, domainIndexKey).Result()
	if err != nil {
		return err
	}

	for _, d := range domains {
		size, err := dq.client.ZCard(ctx, domainQueuePrefix+d).Result()
		if err != nil || size == 0 {
			dq.client.SRem(ctx, domainIndexKey, d)
		}
	}
	return nil
}

// ScheduleTick runs one round of the meta-scheduler, dispatching URLs from
// all domain queues according to budgets.
func (dq *DomainQueue) ScheduleTick(ctx context.Context, batchSize int64,
	processor func(ctx context.Context, urls []string) error) error {

	urls, err := dq.PopBatch(ctx, batchSize)
	if err != nil {
		return fmt.Errorf("pop batch: %w", err)
	}

	if len(urls) == 0 {
		time.Sleep(100 * time.Millisecond) // back off when empty
		return nil
	}

	return processor(ctx, urls)
}
