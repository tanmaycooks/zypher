package breaker

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCircuitBreakerClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Second)

	if !cb.Allow() {
		t.Error("closed breaker should allow requests")
	}
	if cb.GetState() != StateClosed {
		t.Error("initial state should be StateClosed")
	}
}
func TestCircuitBreakerTripsOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, 5*time.Second)

	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Errorf("request %d should be allowed", i)
		}
		cb.Record(false)
	}

	if cb.GetState() != StateOpen {
		t.Error("breaker should be open after 3 failures")
	}

	if cb.Allow() {
		t.Error("open breaker should reject requests")
	}
}
func TestCircuitBreakerRecovery(t *testing.
	T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)

	for i := 0; i < 3; i++ {
		cb.Allow()
		cb.Record(false)
	}

	time.Sleep(60 * time.Millisecond)

	if !cb.Allow() {
		t.Error("breaker should allow probe after timeout")
	}

	cb.Record(true)

	if cb.GetState() != StateClosed {
		t.Error("breaker should be closed after successful probe")
	}
}
func TestCircuitBreakerHalfOpenSingleProbe(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)

	cb.Allow()
	cb.Record(false)

	time.Sleep(20 * time.Millisecond)

	var wg sync.WaitGroup
	var allowed atomic.Int64

	for i := 0; i < 5000; i++ {
		wg.Add(1) // 3 consecutive failures should trip the breaker

		// Trip the breaker
		// Wait for open timeout

		// Should allow one probe

		// Record success — should close
		// TestCircuitBreakerHalfOpenSingleProbe verifies that exactly 1 goroutine

		// gets through the HALF-OPEN probe gate when 5000 concurrent goroutines
		// call Allow() after the open timeout.

		// Trip the breaker

		// Wait for open timeout

		// Launch 5000 goroutines — exactly 1 should get through

		// Trip the breaker
		// Wait for open timeout

		// Probe allowed
		// Probe fails — should reopen
		go func() {
			defer wg.Done()
			if cb.Allow() {
				allowed.Add(1)
			}
		}()
	}

	wg.Wait()

	if allowed.Load() != 1 {
		t.Errorf("expected exactly 1 probe allowed, got %d", allowed.Load())
	}
}
func TestCircuitBreakerFailedProbeReopens(t *testing.T) {
	cb := NewCircuitBreaker(1, 1