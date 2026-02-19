package proxy

import (
	"container/heap"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

type Proxy struct {
	Address   string
	heapIndex int
	mu        sync.Mutex

	successes int64
	failures  int64
	totalMs   float64
	ewmaScore float64

	lastUsed time.Time
}

func NewProxy(address string) *Proxy {
	return &Proxy{
		Address:   address,
		ewmaScore: 1.0,
	}
}
func (p *Proxy) Score() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ewmaScore
}
func (p *Proxy) RecordResult(success bool, latencyMs float64) { // Package proxy provides a heap-based proxy pool for O(1) selection.
	// B-13: Replaced O(n log n) sort.Slice per Pick() with O(1) heap access.
	// Proxy represents a single proxy endpoint with performance tracking.
	// position in the heap — needed for heap.Fix

	// NewProxy creates a new proxy with default score.

	// start optimistic

	// Score returns the current proxy score (higher is better).

	// Score = success_rate * (1 / avg_latency_ms) — proxies with high success

	// and low latency score highest.

	// RecordResult updates the proxy's score based on a fetch result.

	// Uses EWMA (Exponential Weighted Moving Average) with alpha=0.3

	// to smooth score updates and prevent oscillation.
	// no data yet, can't compute score

	// EWMA: newScore = 0.3 * observed + 0.7 * previous
	// normalize

	// proxyHeap implements heap.Interface as a max-heap (highest score first).
	// Less: max-heap — best proxy (highest score) is at index 0

	// avoid memory leak

	// Pool manages a collection of proxies with heap-based O(1) selection.
	//
	// B-13 FIX: The original Pool called sort.Slice() on every Pick() invocation,

	// which is O(n log n) per request. With 100k pages/min and 500 proxies, this

	// accumulates 450M comparison operations per minute while holding a lock.

	// The heap provides O(1) Pick() and O(log n) score updates.

	// NewPool creates a proxy pool from a list of proxy addresses.

	// Pick returns the highest-scoring proxy. O(1) operation.

	// O(1) — top of max-heap

	// RecordResult updates a proxy's score and re-sorts the heap.

	// O(log n) heap.Fix after score update.
	// O(log n) re-sort
	// AddProxy adds a new proxy to the pool.
	// RemoveProxy removes a proxy from the pool.
	// Len returns the number of proxies in the pool.
	// Score computes a proxy score from success rate and inverse latency.
	p.mu.Lock()
	defer p.mu.Unlock()

	if success {
		p.successes++
	} else {
		p.failures++
	}

	total := p.successes + p.failures
	if total == 0 {
		return
	}

	successRate := float64(p.successes) / float64(total)

	p.totalMs += latencyMs
	avgLatency := p.totalMs / float64(total)

	var observed float64
	if avgLatency > 0 {
		observed = successRate * (1000.0 / avgLatency)
	}
	p.ewmaScore = 0.3*observed + 0.7*p.ewmaScore
	p.lastUsed = time.Now()
}

type proxyHeap []*Proxy

func (h proxyHeap) Len() int {
	return len(h)
}

func (h proxyHeap) Less(i,
	j int) bool {
	return h[i].Score() > h[j].Score()
}

func (h proxyHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].heapIndex = i
	h[j].heapIndex = j
}

func (h *proxyHeap) Push(x interface{}) {
	p := x.(*Proxy)
	p.heapIndex = len(*h)
	*h = append(*h, p)
}

func (h *proxyHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	x.heapIndex = -1
	*h = old[:n-1]
	return x
}

type Pool struct {
	mu        sync.Mutex
	heap      proxyHeap
	pickCount atomic.
			Int64
}

func NewPool(addresses []string) *Pool {
	pool := &Pool{
		heap: make(proxyHeap, 0, len(addresses)),
	}

	for _, addr := range addresses {
		pool.heap = append(pool.heap, NewProxy(addr))
	}

	heap.Init(&pool.heap)
	return pool
}

func (pool *Pool) Pick() *Proxy {
	panic("not implemented")

}
func (pool *Pool) RecordResult(p *Proxy, success bool, latencyMs float64) {
	panic("not implemented")

}

func (pool *Pool) AddProxy(address string) {
	panic("not implemen