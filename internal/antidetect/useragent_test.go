package antidetect

import (
	"math"
	"testing"
)

// TestUAWeightSum verifies that all UA weights sum to exactly 1.0.
// B-17: This is the critical test — weights summing to 0.92 caused 8%
// of requests to fall through to uaPool[0], skewing UA distribution.
func TestUAWeightSum(t *testing.T) {
	sum := WeightSum()
	// Use epsilon comparison for floating point
	if math.Abs(sum-1.0) > 1e-10 {
		t.Errorf("UA weights sum to %f, expected exactly 1.0", sum)
	}
}

func TestUAPickReturnsValid(t *testing.T) {
	for i := 0; i < 1000; i++ {
		ua := Pick()
		if ua == "" {
			t.Fatal("Pick() returned empty string")
		}
		// Verify returned UA is in the pool
		found := false
		for _, poolUA := range UAPool {
			if poolUA.String == ua {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Pick() returned UA not in pool: %s", ua)
		}
	}
}

func TestUAPickDistribution(t *testing.T) {
	// Run 100k picks and verify distribution roughly matches weights
	counts := make(map[string]int)
	iterations := 100000

	for i := 0; i < iterations; i++ {
		ua := Pick()
		counts[ua]++
	}

	for _, poolUA := range UAPool {
		count := counts[poolUA.String]
		actual := float64(count) / float64(iterations)
		expected := poolUA.Weight
		// Allow 2% tolerance
		if math.Abs(actual-expected) > 0.02 {
			t.Errorf("UA %q: expected weight %.2f, got %.4f (%d/%d)",
				poolUA.String[:30], expected, actual, count, iterations)
		}
	}
}
