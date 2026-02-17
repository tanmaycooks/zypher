// Package limiter provides an AIMD (Additive Increase Multiplicative Decrease)
// adaptive concurrency limiter for the worker pool.
// B-04: Fixed semaphore/counter divergence race by replacing buffered channel
// with atomic integers. Uses CAS-based Acquire() with Gosched() hint.
package limiter

import (
	"runtime"
	"sync/atomic"
)

// AdaptiveLimiter controls concurrent fetch goroutines using the AIMD algorithm
// (same as TCP congestion control). It is self-tuning: increases concurrency
// on success (+1) and halves it on failure (/2).
//
// B-04 FIX: Uses atomic ceiling + inFlight counters instead of a buffered channel
// semaphore. The original design had a critical race: OnFailure()'s default branch
// set al.concurrency = newConcur WITHOUT draining the channel, leaving the channel
// with more tokens than the limit implied. Goroutines would drain those old tokens
// freely, bypassing the intended concurrency ceiling.
type AdaptiveLimiter struct {
	ceiling   atomic.Int64 // current allowed maximum
	inFlight  atomic.Int64 // goroutines currently holding a slot
	minConcur int64        // minimum concurrency floor
	maxConcur int64        // maximum concurrency ceiling
}

// NewAdaptiveLimiter creates a new AIMD limiter with the given bounds.
// Initial ceiling is set to minConcur for conservative startup.
func NewAdaptiveLimiter(minConcur, maxConcur int64) *AdaptiveLimiter {
	al := &AdaptiveLimiter{
		minConcur: minConcur,
		maxConcur: maxConcur,
	}
	al.ceiling.Store(minConcur)
	return al
}

// Acquire blocks until a concurrency slot is available.
// Uses a CAS loop with runtime.Gosched() to yield the processor
// when at capacity, preventing busy-wait spinlock behavior.
func (al *AdaptiveLimiter) Acquire() {
	for {
		current := al.inFlight.Load()
		if current >= al.ceiling.Load() {
			runtime.Gosched() // yield; don't busy-wait in tight spin
			continue
		}
		if al.inFlight.CompareAndSwap(current, current+1) {
			return // atomic slot claim — no channel needed
		}
	}
}

// Release returns a concurrency slot. Must be called when a fetch completes.
// Typical usage: defer al.Release()
func (al *AdaptiveLimiter) Release() {
	al.inFlight.Add(-1)
}

// OnSuccess performs additive increase: ceiling += 1 (up to maxConcur).
// Called when a fetch succeeds, gradually increasing throughput.
func (al *AdaptiveLimiter) OnSuccess() {
	for {
		current := al.ceiling.Load()
		newCeil := current + 1
		if newCeil > al.maxConcur {
			newCeil = al.maxConcur
		}
		if al.ceiling.CompareAndSwap(current, newCeil) {
			return
		}
	}
}

// OnFailure performs multiplicative decrease: ceiling /= 2 (down to minConcur).
// Called when a fetch fails, rapidly reducing load on struggling targets.
func (al *AdaptiveLimiter) OnFailure() {
	for {
		current := al.ceiling.Load()
		newCeil := current / 2
		if newCeil < al.minConcur {
			newCeil = al.minConcur
		}
		if al.ceiling.CompareAndSwap(current, newCeil) {
			return
		}
	}
}

// Ceiling returns the current concurrency ceiling.
func (al *AdaptiveLimiter) Ceiling() int64 {
	return al.ceiling.Load()
}

// InFlight returns the current number of active goroutines.
func (al *AdaptiveLimiter) InFlight() int64 {
	return al.inFlight.Load()
}
