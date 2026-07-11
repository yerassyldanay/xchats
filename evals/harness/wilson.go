package main

import "math"

// wilsonZ95 is the z-score for a 95% Wilson score interval — the one confidence level
// this harness reports, everywhere, so a number is always comparable across scenarios
// without also having to check which confidence level produced it.
const wilsonZ95 = 1.96

// wilsonInterval computes the 95% Wilson score interval for `successes` out of `total`
// binomial trials. Used for the POOLED pass rate of one (prompt, model) pair's full
// repetition set (e.g. 15 intents x 5 repetitions = 75 outputs) — never per-intent: 5
// attempts per intent is enough to notice a systematically-broken intent, not enough for
// a per-intent confidence interval (see report.go's wiring for where this is applied and
// how it's labeled). Chosen over a plain normal approximation because it stays inside
// [0,1] and remains meaningful at small n and near p=0 or p=1 — exactly where a
// production-safety-adjacent screening score often lands (see the total=10 boundary
// cases in wilson_test.go).
//
// total=0 returns (0, 0) — there is no interval to report for zero trials, not a
// misleadingly confident (0,1) or (0,0) that looks like a real computed result.
func wilsonInterval(successes, total int) (lo, hi float64) {
	if total <= 0 {
		return 0, 0
	}
	n := float64(total)
	phat := float64(successes) / n
	z2 := wilsonZ95 * wilsonZ95

	denominator := 1 + z2/n
	center := phat + z2/(2*n)
	adjustment := wilsonZ95 * math.Sqrt(phat*(1-phat)/n+z2/(4*n*n))

	lo = (center - adjustment) / denominator
	hi = (center + adjustment) / denominator

	// Clamp: floating-point rounding can push the raw computation a hair outside [0,1]
	// at the p=0/p=1 boundary (e.g. a raw lower bound of -0.0000...1 for successes=0) —
	// clamping reports the true mathematical bound, not a rounding artifact.
	if lo < 0 {
		lo = 0
	}
	if hi > 1 {
		hi = 1
	}
	return lo, hi
}
