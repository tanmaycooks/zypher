// Package breaker provides a per-domain circuit breaker with correct HALF-OPEN probe gating.
// B-06: Fixed unlimited concurrent probes in HALF-OPEN state using atomic.Bool CAS.
package breaker

import (
	"sync"
	"sync/atomic"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed   State = iota // Normal operation — all requests allowed
	StateOpen                  // Failure threshold exceeded — all requests rejected
	StateHalfOpen              // Recovery probe — exactly one request allowed
)

// CircuitBreaker prevents cascading failures by tracking per-domain error rates.
// When a domain exceeds maxFailures consecutive failures, the breaker opens
// and rejects all requests for openTimeout duration. After the timeout,
// exactly one probe request is allowed (HALF-OPEN) to test recovery.
//
// B-06 FIX: Added probeActive atomic.Bool to prevent unlimited concurrent probes.
// The original design returned true for ALL callers when state == StateHalfOpen.
// With 5,000 goroutines, all 5,000 would get through — defeating the purpose.
type CircuitBreaker struct {
	mu          sync.Mutex
	state       State
	failures    int
	lastFailure time.Time
	probeActive atomic.Bool // B-06: single probe guard via CAS
	maxFailures int
	openTimeout time.Duration
}

// NewCircuitBreaker creates a circuit breaker for a single domain.
func NewCircuitBreaker(maxFailures int, openTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:       StateClosed,
		maxFailures: maxFailures,
		openTimeout: openTimeout,
	}
}

// Allow checks if a request should be permitted.
// Returns true if the request can proceed, false if it should be rejected.
//
// B-06 FIX: In StateOpen, when timeout expires, transitions to HALF-OPEN
// and uses CompareAndSwap(false, true) — only the FIRST goroutine wins.
// In StateHalfOpen, returns false unconditionally (probe already active).
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		if time.Since(cb.lastFailure) >= cb.openTimeout {
			cb.state = StateHalfOpen
			// Only the first goroutine gets the probe — CAS ensures atomicity
			return cb.probeActive.CompareAndSwap(false, true)
		}
		return false

	case StateHalfOpen:
		// Only the probe request (which set probeActive) is allowed
		return false
	}

	return false
}

// Record reports the result of a request (success or failure).
// Must be called after every request that was permitted by Allow().
//
// B-06 FIX: Always resets probeActive to false first.
func (cb *CircuitBreaker) Record(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.probeActive.Store(false) // always reset probe guard

	if success {
		cb.failures = 0
		cb.state = StateClosed
	} else {
		cb.failures++
		cb.lastFailure = time.Now()
		if cb.failures >= cb.maxFailures {
			cb.state = StateOpen
		}
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) GetState() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Failures returns the current consecutive failure count.
func (cb *CircuitBreaker) Failures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failures
}
