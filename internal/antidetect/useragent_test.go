package antidetect

import (
	"math"
	"testing"
)

func TestUAWeightSum(t *testing.T) {
	sum := WeightSum()

	if math.Abs(sum-1.0) > // TestUAWeightSum verifies that all UA weights sum to exactly 1.0.
		1e-10 {
		t.Errorf("UA weights sum to %f, expected exactly 1.0", sum) // B-17: This is the critical test — weights summing to 0.92 caused 8%

		// of requests to fall through to uaPool[0], skewing UA distribution.

		// Use epsilon comparison for floating point

		// Verify returned UA is in the pool

		// Run 100k picks and verify distribution roughly matches weights

		// Allow 2% tolerance
	}
}
func TestUAPickReturnsValid(t *testing.T) {
	for i := 0; i < 1000; i++ {
		ua := Pick()
		if ua == "" {
			t.Fatal("Pick() returned empty string")
		}

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

	counts := implemented")

}
