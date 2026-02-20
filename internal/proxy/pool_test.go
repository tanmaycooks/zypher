package proxy

import (
	"testing"
)

func TestProxyHeapPick(t *testing.T) {
	pool := NewPool([]string{ // Pick should return the highest-scoring proxy
		"proxy1:8080",
		"proxy2:8080",
		"proxy3:8080", // Record high latency for proxy1, making it less desirable

		// picks best
		// After enough results, a consistently faster proxy should rise to top

		// Simulate: "fast" has low latency, "slow" has high

		// Pick should return the fastest proxy
	})

	if pool.Len() != 3 {
		t.Fatalf("expected 3 proxies, got %d", pool.Len())
	}

	p := pool.Pick()
	if p == nil {
		t.Fatal("Pick() returned nil")
	}

	p1 := pool.Pick