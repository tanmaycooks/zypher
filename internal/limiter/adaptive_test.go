package limiter

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestAdaptiveLimiterCeilingRespected(t *testing.T) {
	al := NewAdaptiveLimiter(5, 100)

	// Start at minConcur
	if al.Ceiling() != 5 {
		t.Errorf("initial ceiling = %d, want 5", al.Ceiling())
	}

	// Acquire all 5 slots
	for i := 0; i < 5; i++ {
		al.Acquire()
	}
	if al.InFlight() != 5 {
		t.Errorf("inFlight = %d, want 5", al.InFlight())
	}

	// Release all
	for i := 0; i < 5; i++ {
		al.Release()
	}
	if al.InFlight() != 0 {
		t.Errorf("inFlight after release = %d, want 0", al.InFlight())
	}
}

func TestAdaptiveLimiterAIMD(t *testing.T) {
	al := NewAdaptiveLimiter(2, 100)

	// Start at 2, increase to 12 (10 successes)
	for i := 0; i < 10; i++ {
		al.OnSuccess()
	}
	if al.Ceiling() != 12 {
		t.Errorf("ceiling after 10 successes = %d, want 12", al.Ceiling())
	}

	// Multiplicative decrease: 12 / 2 = 6
	al.OnFailure()
	if al.Ceiling() != 6 {
		t.Errorf("ceiling after failure = %d, want 6", al.Ceiling())
	}

	// Another failure: 6 / 2 = 3
	al.OnFailure()
	if al.Ceiling() != 3 {
		t.Errorf("ceiling after second failure = %d, want 3", al.Ceiling())
	}

	// Another failure: 3 / 2 = 1, but floor is 2
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

// BenchmarkAIMDLimiterConcurrent verifies ceiling is respected under 500+ concurrent goroutines.
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
			// Verify ceiling is respected
			if current > al.Ceiling()+1 { // +1 for race window
				b.Errorf("inFlight %d exceeded ceiling %d", current, al.Ceiling())
			}
			al.Release()
		}
	})
}

func TestAdaptiveLimiterConcurrentCeiling(t *testing.T) {
	al := NewAdaptiveLimiter(5, 20)

	// Increase ceiling to 20
	for i := 0; i < 15; i++ {
		al.OnSuccess()
	}

	// Launch 500 goroutines
	var wg sync.WaitGroup
	violations := atomic.Int64{}

	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			al.Acquire()
			defer al.Release()

			inFlight := al.InFlight()
			ceiling := al.Ceiling()
			// Allow +2 for CAS race window
			if inFlight > ceiling+2 {
				violations.Add(1)
			}
		}()
	}

	wg.Wait()

	if v := violations.Load(); v > 0 {
		t.Errorf("ceiling violated %d times", v)
	}
}
