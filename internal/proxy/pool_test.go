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

	p1 := pool.Pick()
	pool.RecordResult(p1, true, 500.0)

	p2 := pool.Pick()
	if p2 == nil {
		t.Fatal("Pick() returned nil after RecordResult")
	}
}

func TestProxyHeapOrdering(t *testing.T) {
	pool := NewPool([]string{
		"slow:8080",
		"fast:8080",
		"medium:8080",
	})

	for i := 0; i < len(pool.heap); i++ {
		p := pool.heap[i]
		switch p.Address {
		case "fast:8080"