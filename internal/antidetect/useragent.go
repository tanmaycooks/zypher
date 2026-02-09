// Package antidetect provides browser fingerprint impersonation and user-agent rotation.
// B-17: Fixed UA weight sum from 0.92 to exactly 1.0.
package antidetect

import (
	"math/rand"
)

// UserAgent represents a browser user-agent string with its selection weight.
type UserAgent struct {
	String string
	Weight float64
}

// UAPool is a weighted pool of user-agent strings.
// B-17 FIX: All weights sum to exactly 1.0.
// Original weights: 0.40 + 0.15 + 0.10 + 0.12 + 0.08 + 0.04 + 0.03 = 0.92
// Fixed: Redistributed missing 0.08 proportionally to most realistic UAs.
var UAPool = []UserAgent{
	{
		String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Weight: 0.43, // Chrome/Win — most common (was 0.40, added 0.03)
	},
	{
		String: "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Weight: 0.17, // Chrome/macOS (was 0.15, added 0.02)
	},
	{
		String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		Weight: 0.12, // Firefox/Win (was 0.10, added 0.02)
	},
	{
		String: "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
		Weight: 0.12, // Safari/macOS (unchanged)
	},
	{
		String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		Weight: 0.09, // Edge/Win (was 0.08, added 0.01)
	},
	{
		String: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Weight: 0.04, // Chrome/Linux (unchanged)
	},
	{
		String: "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
		Weight: 0.03, // Firefox/Linux (unchanged)
	},
}

// Pick selects a random user-agent string based on the configured weights.
// Uses cumulative weight distribution for correct weighted random selection.
func Pick() string {
	r := rand.Float64()
	cumulative := 0.0
	for _, ua := range UAPool {
		cumulative += ua.Weight
		if r <= cumulative {
			return ua.String
		}
	}
	// Should never reach here if weights sum to 1.0
	// Safety fallback: return last UA
	return UAPool[len(UAPool)-1].String
}

// WeightSum returns the sum of all UA weights. Must equal 1.0.
func WeightSum() float64 {
	sum := 0.0
	for _, ua := range UAPool {
		sum += ua.Weight
	}
	return sum
}
