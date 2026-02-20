// Package proxy provides a heap-based proxy pool for O(1) selection.
// B-13: Replaced O(n log n) sort.Slice per Pick() with O(1) heap access.
package proxy

import (
	"container/heap"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// Proxy represents a single proxy endpoint with performance tracking.
type Proxy struct {
	Address   string
	heapIndex int // position in the heap — needed for heap.Fix

	mu        sync.Mutex
	successes int64
	failures  int64
	totalMs   float64
	ewmaScore float64
	lastUsed  time.Time
}

// NewProxy creates a new proxy with default score.
func NewProxy(address string) *Proxy {
	return &Proxy{
		Address:   address,
		ewmaScore: 1.0, // start optimistic
	}
}

// Score returns the current proxy score (higher is better).
// Score = success_rate * (1 / avg_latency_ms) — proxies with high success
// and low latency score highest.
func (p *Proxy) Score() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ewmaScore
}

// RecordResult updates the proxy's score based on a fetch result.
// Uses EWMA (Exponential Weighted Moving Average) with alpha=0.3
// to smooth score updates and prevent oscillation.
func (p *Proxy) RecordResult(success bool, latencyMs float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if success {
		p.successes++
	} else {
		p.failures++
	}

	total := p.successes + p.failures
	if total == 0 {
		return // no data yet, can't compute score
	}

	successRate := float64(p.successes) / float64(total)

	p.totalMs += latencyMs
	avgLatency := p.totalMs / float64(total)

	// EWMA: newScore = 0.3 * observed + 0.7 * previous
	var observed float64
	if avgLatency > 0 {
		observed = successRate * (1000.0 / avgLatency) // normalize
	}
	p.ewmaScore = 0.3*observed + 0.7*p.ewmaScore
	p.lastUsed = time.Now()
}

// proxyHeap implements heap.Interface as a max-heap (highest score first).
type proxyHeap []*Proxy

func (h proxyHeap) Len() int { return len(h) }

// Less: max-heap — best proxy (highest score) is at index 0
func (h proxyHeap) Less(i, j int) bool { return h[i].Score() > h[j].Score() }

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
	old[n-1] = nil // avoid memory leak
	x.heapIndex = -1
	*h = old[:n-1]
	return x
}

// Pool manages a collection of proxies with heap-based O(1) selection.
//
// B-13 FIX: The original Pool called sort.Slice() on every Pick() invocation,
// which is O(n log n) per request. With 100k pages/min and 500 proxies, this
// accumulates 450M comparison operations per minute while holding a lock.
// The heap provides O(1) Pick() and O(log n) score updates.
type Pool struct {
	mu        sync.Mutex
	heap      proxyHeap
	pickCount atomic.Int64
}

// NewPool creates a proxy pool from a list of proxy addresses.
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

// Pick returns the highest-scoring proxy. O(1) operation.
func (pool *Pool) Pick() *Proxy {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	if len(pool.heap) == 0 {
		return nil
	}

	pool.pickCount.Add(1)
	return pool.heap[0] // O(1) — top of max-heap
}

// RecordResult updates a proxy's score and re-sorts the heap.
// O(log n) heap.Fix after score update.
func (pool *Pool) RecordResult(p *Proxy, success bool, latencyMs float64) {
	p.RecordResult(success, latencyMs)

	pool.mu.Lock()
	if p.heapIndex >= 0 && p.heapIndex < len(pool.heap) {
		heap.Fix(&pool.heap, p.heapIndex) // O(log n) re-sort
	}
	pool.mu.Unlock()
}

// AddProxy adds a new proxy to the pool.
func (pool *Pool) AddProxy(address string) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	p := NewProxy(address)
	heap.Push(&pool.heap, p)
}

// RemoveProxy removes a proxy from the pool.
func (pool *Pool) RemoveProxy(p *Proxy) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	if p.heapIndex >= 0 && p.heapIndex < len(pool.heap) {
		heap.Remove(&pool.heap, p.heapIndex)
	}
}

// Len returns the number of proxies in the pool.
func (pool *Pool) Len() int {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return len(pool.heap)
}

// Score computes a proxy score from success rate and inverse latency.
func computeScore(successRate, avgLatencyMs float64) float64 {
	if avgLatencyMs <= 0 {
		return successRate
	}
	return successRate * math.Log1p(1000.0/avgLatencyMs)
}
