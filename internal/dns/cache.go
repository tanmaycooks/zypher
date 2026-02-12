// Package dns provides a stale-while-revalidate DNS cache compatible with http.Transport.
// B-02: Complete DNS cache implementation — was entirely missing, causing compile errors.
package dns

import (
	"context"
	"math/rand"
	"net"
	"sync"
	"time"
)

// entry holds the cached DNS resolution result.
type entry struct {
	addrs      []string  // resolved IP addresses
	expires    time.Time // hard expiry — cache miss after this
	refreshAt  time.Time // soft refresh threshold (80% of TTL)
	refreshing bool      // background refresh in progress
}

// Cache is a thread-safe DNS cache with stale-while-revalidate semantics.
// It serves cached addresses while triggering background refresh when the TTL
// is within 80% of expiry, reducing DNS latency to <1ms after warm-up.
type Cache struct {
	mu       sync.RWMutex
	store    map[string]*entry
	resolver *net.Resolver
	ttl      time.Duration
}

// New creates a DNS cache with the given TTL.
// Recommended: dns.New(5 * time.Minute) for production.
func New(ttl time.Duration) *Cache {
	return &Cache{
		store:    make(map[string]*entry),
		resolver: net.DefaultResolver,
		ttl:      ttl,
	}
}

// Lookup resolves a hostname, returning cached addresses when available.
// If the cache entry is within 80% of its TTL, a background refresh is triggered
// while the stale result is returned immediately (stale-while-revalidate).
func (c *Cache) Lookup(ctx context.Context, host string) ([]string, error) {
	c.mu.RLock()
	e, ok := c.store[host]
	c.mu.RUnlock()

	now := time.Now()

	if ok && now.Before(e.expires) {
		// Stale-while-revalidate: trigger background refresh near expiry
		if now.After(e.refreshAt) && !e.refreshing {
			go c.refresh(host) // non-blocking refresh
		}
		return e.addrs, nil // return cached addresses immediately
	}

	// Cache miss or hard expiry — synchronous fetch
	return c.resolve(ctx, host)
}

// resolve performs a synchronous DNS lookup and stores the result.
func (c *Cache) resolve(ctx context.Context, host string) ([]string, error) {
	addrs, err := c.resolver.LookupHost(ctx, host)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	e := &entry{
		addrs:     addrs,
		expires:   now.Add(c.ttl),
		refreshAt: now.Add(c.ttl * 4 / 5), // refresh at 80% of TTL
	}

	c.mu.Lock()
	c.store[host] = e
	c.mu.Unlock()

	return addrs, nil
}

// refresh performs a background DNS refresh, marking the entry as refreshing
// to prevent multiple concurrent refreshes for the same host.
func (c *Cache) refresh(host string) {
	c.mu.Lock()
	if e, ok := c.store[host]; ok && e.refreshing {
		c.mu.Unlock()
		return // another goroutine is already refreshing
	}

	// Mark as refreshing
	if e, ok := c.store[host]; ok {
		e.refreshing = true
	}
	c.mu.Unlock()

	// Perform the DNS lookup
	newAddrs, err := c.resolver.LookupHost(context.Background(), host)
	if err != nil {
		c.mu.Lock()
		if e, ok := c.store[host]; ok {
			e.refreshing = false
		}
		c.mu.Unlock()
		return
	}

	// Update the cache entry
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

// DialContext returns a dialer function compatible with http.Transport.DialContext.
// It resolves hostnames through the cache and selects a random IP for round-robin.
func (c *Cache) DialContext() func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return dialer.DialContext(ctx, network, addr)
		}

		addrs, err := c.Lookup(ctx, host)
		if err != nil {
			// Fall back to plain dialer on lookup error
			return dialer.DialContext(ctx, network, addr)
		}

		// Select random IP for round-robin across resolved addresses
		ip := addrs[rand.Intn(len(addrs))]
		return dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
	}
}
