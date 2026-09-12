package main

import (
	"errors"
	"testing"
	"time"
)

func TestHistogram_PercentilesOnUniformSamples(t *testing.T) {
	h := newHistogram()
	// 100 samples evenly spread from 1ms to 100ms.
	for i := 1; i <= 100; i++ {
		h.Observe(time.Duration(i) * time.Millisecond)
	}
	count, mean, min, max, p50, p95, p99 := h.Snapshot()
	if count != 100 {
		t.Fatalf("count = %d, want 100", count)
	}
	if min <= 0 || max < min {
		t.Fatalf("min=%v max=%v look wrong", min, max)
	}
	if mean < min || mean > max {
		t.Fatalf("mean=%v outside [min,max]=[%v,%v]", mean, min, max)
	}
	// Bucketed approximation: percentiles must be non-decreasing and within range.
	if !(p50 <= p95 && p95 <= p99) {
		t.Fatalf("percentiles not monotonic: p50=%v p95=%v p99=%v", p50, p95, p99)
	}
	if p99 > max+1 { // +1ms slack for bucket-boundary rounding
		t.Fatalf("p99=%v exceeds observed max=%v", p99, max)
	}
}

func TestHistogram_EmptyIsZeroEverywhere(t *testing.T) {
	h := newHistogram()
	count, mean, min, max, p50, p95, p99 := h.Snapshot()
	if count != 0 || mean != 0 || min != 0 || max != 0 || p50 != 0 || p95 != 0 || p99 != 0 {
		t.Fatalf("empty histogram snapshot should be all zero, got count=%d mean=%v min=%v max=%v p50=%v p95=%v p99=%v",
			count, mean, min, max, p50, p95, p99)
	}
}

func TestHistogram_CapturesOutliersAboveLargestBound(t *testing.T) {
	h := newHistogram()
	h.Observe(1 * time.Millisecond)
	h.Observe(1 * time.Hour) // far beyond the largest bucket bound
	count, _, _, max, _, _, p99 := h.Snapshot()
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	if max < float64(time.Hour/time.Millisecond) {
		t.Fatalf("max = %v, want it to reflect the 1h outlier", max)
	}
	if p99 <= 1 {
		t.Fatalf("p99 = %v, want it pulled toward the outlier", p99)
	}
}

func TestRouteStats_TracksStatusesAndBoundedRecentErrors(t *testing.T) {
	rs := newRouteStats()
	for i := 0; i < 5; i++ {
		rs.Record(time.Millisecond, 200, nil)
	}
	for i := 0; i < maxRecentErrors+5; i++ {
		rs.Record(time.Millisecond, 500, errors.New("boom"))
	}
	rs.mu.Lock()
	gotStatuses := map[int]int64{200: rs.statuses[200], 500: rs.statuses[500]}
	gotErrors := rs.errors
	gotRecent := len(rs.recentErrors)
	rs.mu.Unlock()

	if gotStatuses[200] != 5 || gotStatuses[500] != int64(maxRecentErrors+5) {
		t.Fatalf("status counts = %v, want 200:5 500:%d", gotStatuses, maxRecentErrors+5)
	}
	if gotErrors != int64(maxRecentErrors+5) {
		t.Fatalf("errors = %d, want %d", gotErrors, maxRecentErrors+5)
	}
	if gotRecent != maxRecentErrors {
		t.Fatalf("recentErrors len = %d, want bounded to %d", gotRecent, maxRecentErrors)
	}
}

func TestStatsCollector_OverallAggregatesEveryRoute(t *testing.T) {
	s := newStatsCollector()
	s.Record("routeA", time.Millisecond, 200, nil)
	s.Record("routeB", 2*time.Millisecond, 200, nil)
	s.Record("routeB", 3*time.Millisecond, 500, errors.New("fail"))

	overallCount, _, _, _, _, _, _ := s.Overall().hist.Snapshot()
	if overallCount != 3 {
		t.Fatalf("overall count = %d, want 3", overallCount)
	}
	if got := s.TotalErrors(); got != 1 {
		t.Fatalf("TotalErrors() = %d, want 1", got)
	}
	names := s.RouteNames()
	if len(names) != 2 || names[0] != "routeA" || names[1] != "routeB" {
		t.Fatalf("RouteNames() = %v, want sorted [routeA routeB]", names)
	}
}
