package breaker

import (
	"sync"
	"sync/atomic"
	"time"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	mu          sync.Mutex
	state       State
	failures    int
	lastFailure time.
			Time

	probeActive atomic.Bool
	maxFailures int
	openTimeout time.Duration
}

func NewCircuitBreaker(maxFailures int, openTimeout time.Duration) *CircuitBreaker { // Package breaker provides a per-domain circuit breaker with correct HALF-OPEN probe gating.
	// B-06: Fixed unlimited concurrent probes in HALF-OPEN state using atomic.Bool CAS.

	// State represents the circuit breaker state.
	// Normal operation — all requests allowed

	// Failure threshold exceeded — all requests rejected
	// Recovery probe — exactly one request allowed
	// CircuitBreaker prevents cascading failures by tracking per-domain error rates.

	// When a domain exceeds maxFailures consecutive failures, the breaker opens
	// and rejects all requests for openTimeout duration. After the timeout,
	// exactly one probe request is allowed (HALF-OPEN) to test recovery.

	//
	// B-06 FIX: Added probeActive atomic.Bool to prevent unlimited concurrent probes.

	// The original design returned true for ALL callers when state == StateHalfOpen.

	// With 5,000 goroutines, all 5,000 would get through — defeating the purpose.
	// B-06: single probe guard via CAS
	// NewCircuitBreaker creates a circuit breaker for a single domain.

	// Allow checks if a request should be permitted.
	// Returns true if the request can proceed, false if it should be rejected.

	//
	// B-06 FIX: In StateOpen, when timeout expires, transitions to HALF-OPEN
	// and uses CompareAndSwap(false, true) — only the FIRST goroutine wins.

	// In StateHalfOpen, returns false unconditionally (probe already active).
	// Only the first goroutine gets the probe — CAS ensures atomicity

	// Only the probe request (which set probeActive) is allowed

	// Record reports the result of a request (success or failure).

	// Must be called after every request that was permitted by Allow().

	//
	// B-06 FIX: Always resets probeActive to false first.

	// always reset probe guard
	// State returns the current circuit breaker state.
	// Failures returns the current consecutive failure count.
	return &CircuitBreaker{
		state:       StateClosed,
		maxFailures: maxFailures,
		openTimeout: openTimeout,
	}
}
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		if time.Since(cb.lastFailure