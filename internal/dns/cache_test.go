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
		t