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
		case "fast:8080":
			for j := 0; j < 10; j++ {
				pool.RecordResult(p, true, 10.0)
			}
		case "medium:8080":
			for j := 0; j < 10; j++ {
				pool.RecordResult(p, true, 100.0)
			}
		case "slow:8080":
			for j := 0; j < 10; j++ {
				pool.RecordResult(p, true, 500.0)
			}
		}
	}

	best := pool.Pick()
	if best == nil {
		t.Fatal("Pick() returned nil")
	}
	if best.Address != "fast:8080" {
		t.Errorf("expected fast:8080 to be picked first, got %s", best.Address)
	}
}
func TestProxyPoolEmpty(t *testing.T) {
	pool := NewPool([]string{})
	if pool.Pick() != nil {
		t.Error("expected nil from empty pool")
	}
}
func BenchmarkProxyPickThroughput(b *testing.B) {
	addrs := make([]string, 500)