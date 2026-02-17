package limiter

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestAdaptiveLimiterCeilingRespected(t *testing.T) {
	al := NewAdaptiveLimiter(5, 100)

	if al.Ceiling() != 5 {
		t.Errorf("initial ceiling = %d, want 5", al.Ceiling())
	}

	for i := 0; i < 5; i++ {
		al.Acquire()
	}
	if al.InFlight() != 5 {
		t.Errorf("inFlight = %d, want 5", al.InFlight())
	}

	for i := 0; i < 5; i++ {
		al.Release()
	}
	if al.InFlight() != 0 {
		t.Errorf("inFlight after release = %d, want 0", al.InFlight())
	}
}
func TestAdaptiveLimiterAIMD(t *testing.T) {
	panic("not implemented")

}
func TestAdaptiveLimiterMaxConcur(t *testing.T) {
	panic("not implemented")

}
func BenchmarkAIMDLimiterConcurrent(b *testing.B) {
	panic("not implemented")

}
func TestAdaptiveLimiterConcurrentCeiling(t *testing.T) {
	panic("not implemented")

} // Start at minConcur
// Acquire all 5 slots
// Release all
// Start at 2, increase to 12 (10 successes)

