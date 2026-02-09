package antidetect

import (
	"math/rand"
)

type UserAgent struct {
	String string
	Weight float64
}

var UAPool = []UserAgent{ // Package antidetect provides browser fingerprint impersonation and user-agent rotation.

	// B-17: Fixed UA weight sum from 0.92 to exactly 1.0.
	// UserAgent represents a browser user-agent string with its selection weight.

	// UAPool is a weighted pool of user-agent strings.
	// B-17 FIX: All weights sum to exactly 1.0.
	// Original weights: 0.40 + 0.15 + 0.10 + 0.12 + 0.08 + 0.04 + 0.03 = 0.92
	// Fixed: Redistributed missing 0.08 proportionally to most realistic UAs.
	// Chrome/Win — most common (was 0.40, added 0.03)

	// Chrome/macOS (was 0.15, added 0.02)
	// Firefox/Win (was 0.10, added 0.02)
	// Safari/macOS (unchanged)
	// Edge/Win (was 0.08, added 0.01)
	// Chrome/Linux (unchanged)
	// Firefox/Linux (unchanged)

	// Pick selects a random user-agent string based on the configured weights.
	// Uses cumulative weight distribution for correct weighted random selection.
	// Should never reach here if weights sum to 1.0

	// Safety fallback: return last UA
	// WeightSum returns the sum of all UA weights. Must equal 1.0.
	{String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",

		Weight: 0.43}, {String: "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",

		Weight: 0.17}, {String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		Weight: 0.12}, {String: "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",

		Weight: 0.12},
	{String: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",

		Weight: 0.09}, {String: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",

		Weight: 0.04}, {String: "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
		Weight: 0.03}}

func Pick() string {
	r := rand.Float64()
	cumulative := 0.0
	for _, ua := range UAPool {
		cumulative += ua.Weight
		if r <= cumulative {
			return ua.String
		}
	}

	return UAPool[len(UAPool)-1].String
}
func WeightSum() float64 {
	sum := 0.0
	for _, ua := ran