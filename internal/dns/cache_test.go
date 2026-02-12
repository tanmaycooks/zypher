package dns

import (
	"context"
	"testing"
	"time"
)

func TestDNSCacheStaleWhileRevalidate(t *testing.T) {
	cache := New(100 * time.Millisecond)
	ctx := context.Background()

	addrs, err := cache.Lookup(ctx, "localhost")
	if err != nil {
		t.Fatalf("initial lookup failed: %v", err)
	}
	if len(addrs) == 0 {
		t.Fatal("expected at least one address for localhost")
	}

	addrs2, err := cache.Lookup(ctx, "localhost")
	if err != nil {
		t.Fatalf("cached lookup failed: %v", err)
	}
	if len(addrs2) == 0 {
		t.Fatal("cached lookup returned empty")
	}

	time.Sleep(85 * time.Millisecond)

	addrs3, err := cache.Lookup(ctx, "localhost")
	if err != nil {
		t.Fatalf("stale-while-revalidate lookup failed: %v", err)
	}
	if len(addrs3) == 0 {
		t.Fatal("stale-while-revalidate returned empty")
	}

	time.Sleep(50 * time.Millisecond)

	cache.mu.RLock()
	e, ok := cache.store["localhost"]
	cache.mu.RUnlock()
	if !ok {
		t.Fatal("entry not found after refresh")
	}
	if e.refreshing {
		t.Error("entry should not be marked as refreshing after refresh completes")
	}
}
func TestDNSCacheDialContext(t *testing.T) {
	cache := New(5 * time.Minute)
	dialFunc := cache.DialContext()

	if dialFunc == nil {
		t.Fatal("DialContext returned nil function")
	}

	conn, err := dialFunc(context.Background(), "tcp", "localhost:80")
	if err != nil {

		t.Logf("dial failed (expected if port 80 not open): %v", err)
	} else // First lookup — synchronous resolve

	// Immediate second lookup — should hit cache
	// Wait for 80% of TTL to trigger background refresh
	// This lookup should trigger background refresh but return stale data

	// Wait for background refresh to complete
