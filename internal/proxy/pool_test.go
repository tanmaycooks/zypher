package proxy

import (
	"testing"
)

func TestProxyHeapPick(t *testing.T) {
	pool := NewPool([]string{
		"proxy1:8080",
		"proxy2:8080",
		"proxy3:8080",
	})

	if pool.Len() != 3 {
		t.Fatalf("expected 3 proxies, got %d", pool.Len())
	}

	// Pick should return the highest-scoring proxy
	p := pool.Pick()
	if p == nil {
		t.Fatal("Pick() returned nil")
	}

	// Record high latency for proxy1, making it less desirable
	p1 := pool.Pick() // picks best
	pool.RecordResult(p1, true, 500.0)

	// After enough results, a consistently faster proxy should rise to top
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

	// Simulate: "fast" has low latency, "slow" has high
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

	// Pick should return the fastest proxy
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
	for i := range addrs {
		addrs[i] = "proxy" + string(rune(i)) + ":8080"
	}
	pool := NewPool(addrs)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p := pool.Pick()
			if p != nil {
				pool.RecordResult(p, true, 50.0)
			}
		}
	})
}
