package dns

import (
	"context"
	"testing"
	"time"
)

func TestDNSCacheStaleWhileRevalidate(t *testing.T) {
	cache := New(100 * time.Millisecond)
	ctx := context.Background()

	// First lookup — synchronous resolve
	addrs, err := cache.Lookup(ctx, "localhost")
	if err != nil {
		t.Fatalf("initial lookup failed: %v", err)
	}
	if len(addrs) == 0 {
		t.Fatal("expected at least one address for localhost")
	}

	// Immediate second lookup — should hit cache
	addrs2, err := cache.Lookup(ctx, "localhost")
	if err != nil {
		t.Fatalf("cached lookup failed: %v", err)
	}
	if len(addrs2) == 0 {
		t.Fatal("cached lookup returned empty")
	}

	// Wait for 80% of TTL to trigger background refresh
	time.Sleep(85 * time.Millisecond)

	// This lookup should trigger background refresh but return stale data
	addrs3, err := cache.Lookup(ctx, "localhost")
	if err != nil {
		t.Fatalf("stale-while-revalidate lookup failed: %v", err)
	}
	if len(addrs3) == 0 {
		t.Fatal("stale-while-revalidate returned empty")
	}

	// Wait for background refresh to complete
	time.Sleep(50 * time.Millisecond)

	// Verify entry was refreshed (new expiry)
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

	// Test dialing localhost
	conn, err := dialFunc(context.Background(), "tcp", "localhost:80")
	if err != nil {
		// Expected on most systems where port 80 isn't listening
		t.Logf("dial failed (expected if port 80 not open): %v", err)
	} else {
		conn.Close()
	}
}

func TestDNSCacheHardExpiry(t *testing.T) {
	cache := New(50 * time.Millisecond)
	ctx := context.Background()

	_, err := cache.Lookup(ctx, "localhost")
	if err != nil {
		t.Fatalf("initial lookup failed: %v", err)
	}

	// Wait past hard expiry
	time.Sleep(60 * time.Millisecond)

	// Should perform fresh synchronous lookup
	addrs, err := cache.Lookup(ctx, "localhost")
	if err != nil {
		t.Fatalf("post-expiry lookup failed: %v", err)
	}
	if len(addrs) == 0 {
		t.Fatal("post-expiry lookup returned empty")
	}
}
