package main

import (
	"math"
	"sort"
	"sync"
	"time"
)

// histogramBoundsMS are the bounded latency histogram's fixed bucket
// upper-bounds (milliseconds) — chosen to give reasonable p50/p95/p99
// resolution across everything from a sub-millisecond in-memory mock
// response to a multi-second outlier, without storing a raw sample per
// request (the spec's own "bounded histograms" requirement: memory stays
// flat regardless of run length or request volume).
var histogramBoundsMS = []float64{
	1, 2, 5, 10, 20, 30, 50, 75, 100, 150, 200, 300, 500, 750,
	1000, 1500, 2000, 3000, 5000, 10000, 20000,
}

// histogram is a thread-safe, fixed-bucket latency histogram in
// milliseconds. The last (implicit) bucket catches everything above the
// largest bound.
type histogram struct {
	mu     sync.Mutex
	counts []int64
	total  int64
	sumMS  float64
	minMS  float64
	maxMS  float64
}

func newHistogram() *histogram {
	return &histogram{counts: make([]int64, len(histogramBoundsMS)+1)}
}

func (h *histogram) Observe(d time.Duration) {
	ms := float64(d) / float64(time.Millisecond)
	idx := sort.SearchFloat64s(histogramBoundsMS, ms)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.counts[idx]++
	h.total++
	h.sumMS += ms
	if h.total == 1 || ms < h.minMS {
		h.minMS = ms
	}
	if ms > h.maxMS {
		h.maxMS = ms
	}
}

// Percentile returns an approximate p-th percentile (0..100) in
// milliseconds — accurate to the bucket boundary it falls in, which is
// exactly the tradeoff a bounded histogram makes for O(1) memory.
func (h *histogram) Percentile(p float64) float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.total == 0 {
		return 0
	}
	target := int64(math.Ceil(p / 100 * float64(h.total)))
	if target < 1 {
		target = 1
	}
	var cum int64
	for i, c := range h.counts {
		cum += c
		if cum >= target {
			if i < len(histogramBoundsMS) {
				return histogramBoundsMS[i]
			}
			return h.maxMS
		}
	}
	return h.maxMS
}

func (h *histogram) Snapshot() (count int64, meanMS, minMS, maxMS, p50, p95, p99 float64) {
	h.mu.Lock()
	total, sum, mn, mx := h.total, h.sumMS, h.minMS, h.maxMS
	h.mu.Unlock()
	mean := 0.0
	if total > 0 {
		mean = sum / float64(total)
	}
	return total, mean, mn, mx, h.Percentile(50), h.Percentile(95), h.Percentile(99)
}

// routeStats accumulates one route's outcomes: latency histogram, status
// code counts, and a bounded sample of recent error strings (never every
// error ever seen — that would grow without bound on a persistently
// misbehaving route).
type routeStats struct {
	hist *histogram

	mu       sync.Mutex
	statuses map[int]int64
	errors   int64
	// recentErrors is a small ring of the most recent distinct-ish error
	// strings, for the human-readable report — capped, not a log.
	recentErrors []string
}

const maxRecentErrors = 10

func newRouteStats() *routeStats {
	return &routeStats{hist: newHistogram(), statuses: map[int]int64{}}
}

func (r *routeStats) Record(d time.Duration, status int, err error) {
	r.hist.Observe(d)
	r.mu.Lock()
	defer r.mu.Unlock()
	if status != 0 {
		r.statuses[status]++
	}
	if err != nil {
		r.errors++
		if len(r.recentErrors) < maxRecentErrors {
			r.recentErrors = append(r.recentErrors, err.Error())
		}
	}
}

// statsCollector is the run's whole result set: one routeStats per route
// name, keyed by the same name used in the request-mix configuration, plus
// an aggregate across every route.
type statsCollector struct {
	mu      sync.Mutex
	routes  map[string]*routeStats
	overall *routeStats
}

func newStatsCollector() *statsCollector {
	return &statsCollector{routes: map[string]*routeStats{}, overall: newRouteStats()}
}

// Overall returns the aggregate across every route.
func (s *statsCollector) Overall() *routeStats { return s.overall }

func (s *statsCollector) routeFor(name string) *routeStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	rs, ok := s.routes[name]
	if !ok {
		rs = newRouteStats()
		s.routes[name] = rs
	}
	return rs
}

func (s *statsCollector) Record(route string, d time.Duration, status int, err error) {
	s.routeFor(route).Record(d, status, err)
	s.overall.Record(d, status, err)
}

// RouteNames returns every route with at least one recorded observation,
// sorted for deterministic report ordering.
func (s *statsCollector) RouteNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.routes))
	for n := range s.routes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (s *statsCollector) Route(name string) *routeStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.routes[name]
}

// TotalErrors sums every route's error count — the run's own nonzero-exit
// signal ("return nonzero for any transport, HTTP, or response-contract
// error").
func (s *statsCollector) TotalErrors() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var total int64
	for _, rs := range s.routes {
		rs.mu.Lock()
		total += rs.errors
		rs.mu.Unlock()
	}
	return total
}
