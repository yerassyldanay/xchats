package main

import (
	"math"
	"testing"
)

// TestWilsonInterval_ReferenceValues pins wilsonInterval against reference values
// independently hand-computed from the standard Wilson score interval formula (z=1.96),
// not derived from the implementation itself — including (48,75), the actual pooled
// scale a 15-intent x 5-repetition finalist run produces.
func TestWilsonInterval_ReferenceValues(t *testing.T) {
	const tol = 0.001
	tests := []struct {
		name           string
		successes      int
		total          int
		wantLo, wantHi float64
	}{
		{"8/10", 8, 10, 0.4902, 0.9433},
		{"0/10 (lower bound floors at 0, not a small negative)", 0, 10, 0.0, 0.2775},
		{"10/10 (upper bound caps at 1)", 10, 10, 0.7225, 1.0},
		{"48/75 (the actual 15 intents x 5 repetitions pooled scale)", 48, 75, 0.5270, 0.7394},
		{"1/1 edge case", 1, 1, 0.2065, 1.0},
		{"0/1 edge case", 0, 1, 0.0, 0.7935},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lo, hi := wilsonInterval(tt.successes, tt.total)
			if math.Abs(lo-tt.wantLo) > tol {
				t.Errorf("wilsonInterval(%d,%d) lo = %.4f, want %.4f (±%.3f)", tt.successes, tt.total, lo, tt.wantLo, tol)
			}
			if math.Abs(hi-tt.wantHi) > tol {
				t.Errorf("wilsonInterval(%d,%d) hi = %.4f, want %.4f (±%.3f)", tt.successes, tt.total, hi, tt.wantHi, tol)
			}
		})
	}
}

func TestWilsonInterval_ZeroTotalReturnsZeroZero(t *testing.T) {
	lo, hi := wilsonInterval(0, 0)
	if lo != 0 || hi != 0 {
		t.Errorf("wilsonInterval(0,0) = (%v,%v), want (0,0) — no misleadingly confident interval for zero trials", lo, hi)
	}
}

// TestWilsonInterval_AlwaysWithinUnitInterval is a property test across a spread of
// (successes, total) pairs — the clamping logic must never let floating-point rounding
// push a bound outside [0,1], regardless of how close successes is to 0 or total.
func TestWilsonInterval_AlwaysWithinUnitInterval(t *testing.T) {
	for total := 1; total <= 100; total++ {
		for successes := 0; successes <= total; successes++ {
			lo, hi := wilsonInterval(successes, total)
			if lo < 0 || lo > 1 {
				t.Fatalf("wilsonInterval(%d,%d) lo = %v, out of [0,1]", successes, total, lo)
			}
			if hi < 0 || hi > 1 {
				t.Fatalf("wilsonInterval(%d,%d) hi = %v, out of [0,1]", successes, total, hi)
			}
			if lo > hi {
				t.Fatalf("wilsonInterval(%d,%d): lo (%v) > hi (%v)", successes, total, lo, hi)
			}
		}
	}
}
