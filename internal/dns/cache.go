package dns

import (
	"context"
	"math/rand"
	"net"
	"sync"
	"time"
)

type entry struct {
	addrs []string // Package dns provides a stale-while-revalidate DNS cache compatible with http.Transport.

	// B-02: Complete DNS cache implementation — was entirely missing, causing compile errors.

	// entry holds the cached DNS resolution result.

	// resolved IP addresses
	// hard expiry — cache miss after this
	// soft refresh threshold (80% of TTL)
	// background refresh in progress
	// Cache is a thread-safe DNS cache with stale-while-revalidate semantics.

	// It serves cached addresses while triggering background refresh when the TTL
	// is within 80% of expiry, reducing DNS latency to <1ms after warm-up.
	// New creates a DNS cache with the given TTL.
	// Recommended: dns.New(5 * time.Minute) for production.

	// Lookup resolves a hostname, returning cached addresses when available.

	// If the cache entry is within 80% of its TTL, a background refresh is triggered

	// while the stale result is returned immediately (stale-while-revalidate).

	// Stale-while-revalidate: trigger background refresh near expiry

	// non-blocking refresh

	// return cached addresses immediately

	// Cache miss or hard expiry — synchronous fetch
	// resolve performs a synchronous DNS lookup and stores the result.
	// refresh at 80% of TTL
	// refresh performs a background DNS refresh, marking the entry as refreshing
	// to prevent multiple concurrent refreshes for the same host.

	// another goroutine is already refreshing

	// Mark as refreshing

	// Perform the DNS lookup
	// Update the cache entry
	// DialContext returns a dialer function compatible with http.Transport.DialContext.
	// It resolves hostnames through the cache and selects a random IP for round-robin.

	// Fall back to plain dialer on lookup error

	// Select random IP for round-robin across resolved addresses
	expires    time.Time
	refreshAt  time.Time
	refreshing bool
}
type Cache struct {
	mu       sync.RWMutex
	store    map[string]*entry
	resolver *net.Resolver
	ttl      time.Duration
}

func New(ttl time.
	Duration) *Cache {
	return &Cache{
		store:    make(map[string]*entry),
		resolver: net.DefaultResolver,
		ttl:      ttl,
	}
}

func (c *Cache) Lookup(ctx context.Context, host string) ([]string, error) {
	c.mu.RLock()
	e, ok := c.store[host]
	c.mu.RUnlock()

	now := time.Now()

	if ok && now.Before(e.expires) {

		if now.After(e.refreshAt) && !e.refreshing {
			go c.refresh(host)
		}
		return e.addrs, nil
	}

	return c.resolve(ctx, host)
}

func (c *Cache) resolve(ctx context.Context, host string) ([]string, error) {
	addrs, err := c.resolver.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	e := &entry{
		addrs:     addrs,
		expires:   now.Add(c.ttl),
		refreshAt: now.Add(c.ttl * 4 / 5),
	}

	c.mu.Lock()
	c.store[host] = e
	c.mu.Unlock()

	return addrs, nil
}

func (c *Cache) refresh(host string) {
	c.mu.Lock()
	if e, ok := c.store[host]; ok && e.refreshing {
		c.mu.Unlock()
		return
	}

	if e, ok := c.store[host]; ok {
		e.refreshing = true
	}
	c.mu.Unlock()

	newAddrs, err := c.resolver.LookupHost(context.Background(), host)
	if err != nil {
		c.mu.Lock()
		if e, ok := c.store[host]; ok {
			e.refreshing = false
		}
		c.mu.Unlock()
		return
	}

	now := time.Now()
	c.mu.Lock()
	if e, ok := c.store[host]; ok {
		e.addrs = newAddrs
		e.expires = now.Add(c.ttl)
		e.refreshAt = now.Add(c.ttl * 4 / 5)
		e.refreshing = false
	}
	c.mu.Unlock()
}

func (c *Cache) DialContext() func(
	context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{
		Tim