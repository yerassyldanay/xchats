package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

// routeReport is one route's (or the overall aggregate's) JSON-serializable
// result summary.
type routeReport struct {
	Route        string        `json:"route"`
	Count        int64         `json:"count"`
	Errors       int64         `json:"errors"`
	RPS          float64       `json:"rps"`
	MeanMS       float64       `json:"mean_ms"`
	MinMS        float64       `json:"min_ms"`
	MaxMS        float64       `json:"max_ms"`
	P50MS        float64       `json:"p50_ms"`
	P95MS        float64       `json:"p95_ms"`
	P99MS        float64       `json:"p99_ms"`
	Statuses     map[int]int64 `json:"statuses,omitempty"`
	RecentErrors []string      `json:"recent_errors,omitempty"`
}

func buildRouteReport(name string, rs *routeStats, measuredSeconds float64) routeReport {
	count, mean, min, max, p50, p95, p99 := rs.hist.Snapshot()
	rs.mu.Lock()
	statuses := make(map[int]int64, len(rs.statuses))
	for k, v := range rs.statuses {
		statuses[k] = v
	}
	errs := rs.errors
	recent := append([]string(nil), rs.recentErrors...)
	rs.mu.Unlock()
	rps := 0.0
	if measuredSeconds > 0 {
		rps = float64(count) / measuredSeconds
	}
	return routeReport{
		Route: name, Count: count, Errors: errs, RPS: rps,
		MeanMS: mean, MinMS: min, MaxMS: max, P50MS: p50, P95MS: p95, P99MS: p99,
		Statuses: statuses, RecentErrors: recent,
	}
}

// runReport is the complete JSON output document.
type runReport struct {
	GeneratedAt     time.Time     `json:"generated_at"`
	BaseURL         string        `json:"base_url"`
	Concurrency     int           `json:"concurrency"`
	WarmupSeconds   float64       `json:"warmup_seconds"`
	DurationSeconds float64       `json:"duration_seconds"`
	Seed            int64         `json:"seed"`
	ChannelsSeen    []string      `json:"channels_seen"`
	Overall         routeReport   `json:"overall"`
	Routes          []routeReport `json:"routes"`
}

func buildRunReport(cfg runConfig, baseURL string, stats *statsCollector, channels []string) runReport {
	measuredSeconds := cfg.Duration.Seconds()
	names := stats.RouteNames()
	routes := make([]routeReport, 0, len(names))
	for _, n := range names {
		routes = append(routes, buildRouteReport(n, stats.Route(n), measuredSeconds))
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i] < channels[j] })
	return runReport{
		GeneratedAt:     time.Now().UTC(),
		BaseURL:         baseURL,
		Concurrency:     cfg.Concurrency,
		WarmupSeconds:   cfg.Warmup.Seconds(),
		DurationSeconds: cfg.Duration.Seconds(),
		Seed:            cfg.Seed,
		ChannelsSeen:    channels,
		Overall:         buildRouteReport("overall", stats.Overall(), measuredSeconds),
		Routes:          routes,
	}
}

// WriteText renders the human-readable stdout report.
func (r runReport) WriteText(w io.Writer) {
	fmt.Fprintf(w, "xchats load — %s\n", r.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "  base_url=%s concurrency=%d warmup=%.0fs duration=%.0fs seed=%d\n",
		r.BaseURL, r.Concurrency, r.WarmupSeconds, r.DurationSeconds, r.Seed)
	fmt.Fprintf(w, "  channels seen: %v\n\n", r.ChannelsSeen)

	printRow := func(rr routeReport) {
		fmt.Fprintf(w, "  %-20s count=%-8d errors=%-6d rps=%-8.1f p50=%-7.1fms p95=%-7.1fms p99=%-7.1fms mean=%-7.1fms max=%-8.1fms\n",
			rr.Route, rr.Count, rr.Errors, rr.RPS, rr.P50MS, rr.P95MS, rr.P99MS, rr.MeanMS, rr.MaxMS)
		if len(rr.Statuses) > 0 {
			fmt.Fprintf(w, "    statuses: %v\n", rr.Statuses)
		}
		for _, e := range rr.RecentErrors {
			fmt.Fprintf(w, "    error: %s\n", e)
		}
	}
	fmt.Fprintln(w, "OVERALL")
	printRow(r.Overall)
	fmt.Fprintln(w, "\nPER ROUTE")
	for _, rr := range r.Routes {
		printRow(rr)
	}
}

// WriteJSON writes the report as indented JSON to path (truncating/creating
// it) — the Makefile's profile-load target reads this back for its own
// generated report.
func (r runReport) WriteJSON(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
