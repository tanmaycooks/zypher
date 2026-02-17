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
	al := NewAdaptiveLimiter(2, 100)

	for i := 0; i < 10; i++ {
		al.OnSuccess()
	}
	if al.Ceiling() != 12 {
		t.Errorf("ceiling after 10 successes = %d, want 12", al.Ceiling())
	}

	al.OnFailure()
	if al.Ceiling() != 6 {
		t.Errorf("ceiling after failure = %d, want 6", al.Ceiling())
	}

	al.OnFailure()
	if al.Ceiling() != 3 {
		t.Errorf("ceiling after second failure = %d, want 3", al.Ceiling())
	}

	al.OnFailure()
	if al.Ceiling() != 2 {
		t.Errorf("ceiling should not go below min = %d, want 2", al.Ceiling())
	}
}
func TestAdaptiveLimiterMaxConcur(t *testing.T) {
	al := NewAdaptiveLimiter(1, 5)

	for i := 0; i < 100; i++ {
		al.OnSuccess()
	}
	if al.Ceiling() != 5 {
		t.Errorf("ceiling should not exceed max = %d, want 5", al.Ceiling())
	}
}
func BenchmarkAIMDLimiterConcurrent(b *testing.B) {
	al := NewAdaptiveLimiter(10, 50)
	var maxSeen atomic.Int64

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			al.Acquire()
			current := al.InFlight()
			for {
				old := maxSeen.Load()
				if current <= old {
					break
				}
				if maxSeen.CompareAndSwap(old, current) {
					break
				}
			}

			if current > al.Ceiling()+1 {
				b.Errorf("inFlight %d exceeded ceiling %d", current, al.Ceiling())
			}
			al.Rel