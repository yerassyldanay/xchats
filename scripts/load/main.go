// Command xchats-load is a standard-library-only closed-loop HTTP load
// generator for the xchats backend's hermetic profiling harness (see
// backend's system.mock_externals). It authenticates once (handling
// fresh-install password rotation), validates every route it will exercise,
// then drives a fixed number of workers in a closed loop for a warm-up
// period (discarded) followed by a measured duration, reporting aggregate
// and per-route latency percentiles, throughput, and errors.
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "xchats-load: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	baseURL := flag.String("base-url", "http://127.0.0.1:8080", "xchats backend base URL (no trailing slash)")
	email := flag.String("email", "admin@xchat.kz", "login email")
	password := flag.String("password", "xchat-admin-change-me", "login password (the fresh-install bootstrap default, unless overridden)")
	newPassword := flag.String("new-password", "xchats-load-harness-rotated-1", "password to set if the account still requires a fresh-install rotation")
	concurrency := flag.Int("concurrency", 32, "fixed number of closed-loop workers")
	warmup := flag.Duration("warmup", 5*time.Second, "warm-up duration (results discarded)")
	duration := flag.Duration("duration", 35*time.Second, "measured duration (results reported)")
	seed := flag.Int64("seed", 42, "PRNG seed — reproducible across runs for a fixed concurrency")
	weightsFlag := flag.String("weights", "", "override the default request mix, e.g. \"kb=20,list_chats=15\" (unlisted routes keep their default weight)")
	contactPoolSize := flag.Int("contact-pool-size", 200, "bounded number of distinct synthetic contacts/conversations reused across requests")
	readyFile := flag.String("ready-file", "", "if set, wait for this file to exist before starting (signals the target server has finished booting/seeding)")
	readyTimeout := flag.Duration("ready-timeout", 60*time.Second, "how long to wait for --ready-file")
	jsonOut := flag.String("json-out", "", "if set, also write the full report as JSON to this path")
	flag.Parse()

	weights, err := parseWeights(*weightsFlag)
	if err != nil {
		return fmt.Errorf("--weights: %w", err)
	}
	wr, err := newWeightedRoutes(weights)
	if err != nil {
		return fmt.Errorf("request mix: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if *readyFile != "" {
		if err := waitForReadyFile(ctx, *readyFile, *readyTimeout); err != nil {
			return err
		}
	}

	client, err := newAPIClient(strings.TrimRight(*baseURL, "/"))
	if err != nil {
		return err
	}
	authCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	err = client.EnsureAuthenticated(authCtx, *email, *password, *newPassword)
	cancel()
	if err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}

	rnr := &runner{
		client:    client,
		chats:     newReservoir(*seed, reservoirCap),
		byChannel: newChannelReservoirs(*seed),
		contacts:  newContactPool(*contactPoolSize),
	}
	preflightCtx, cancel := context.WithTimeout(ctx, requestTimeout*5)
	// Preflight's own RNG is seeded distinctly from any worker's own (XOR
	// with an arbitrary constant) so its one-shot parameter choices never
	// collide with worker index 0's, while staying just as deterministic.
	err = rnr.Preflight(preflightCtx, rand.New(rand.NewSource(*seed^0x5eed)))
	cancel()
	if err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	fmt.Fprintf(os.Stderr, "xchats-load: preflight ok; starting %d workers for %s warm-up + %s measured\n",
		*concurrency, *warmup, *duration)

	stats := newStatsCollector()
	runLoad(ctx, runConfig{
		Concurrency: *concurrency, Warmup: *warmup, Duration: *duration, Seed: *seed,
	}, rnr, wr, stats)

	report := buildRunReport(runConfig{Concurrency: *concurrency, Warmup: *warmup, Duration: *duration, Seed: *seed},
		*baseURL, stats, rnr.byChannel.Channels())
	report.WriteText(os.Stdout)
	if *jsonOut != "" {
		if err := report.WriteJSON(*jsonOut); err != nil {
			return fmt.Errorf("write json report: %w", err)
		}
		fmt.Fprintf(os.Stderr, "xchats-load: wrote %s\n", *jsonOut)
	}

	if n := stats.TotalErrors(); n > 0 {
		return fmt.Errorf("%d request(s) failed (transport, HTTP, or response-contract error) — see report above", n)
	}
	return nil
}

// parseWeights parses "route=weight,route=weight" into a full weight map,
// starting from defaultWeights so an override only needs to name the
// routes it actually changes.
func parseWeights(spec string) (map[string]float64, error) {
	out := make(map[string]float64, len(defaultWeights))
	for k, v := range defaultWeights {
		out[k] = v
	}
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return out, nil
	}
	for _, pair := range strings.Split(spec, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("malformed entry %q (want route=weight)", pair)
		}
		name := strings.TrimSpace(kv[0])
		if _, known := defaultWeights[name]; !known {
			return nil, fmt.Errorf("unknown route %q", name)
		}
		weight, err := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("weight for %q: %w", name, err)
		}
		out[name] = weight
	}
	return out, nil
}

// waitForReadyFile polls for path's existence, so this tool can be started
// concurrently with the server it targets rather than racing its boot —
// the Makefile's profile-load target relies on this instead of a fixed
// sleep.
func waitForReadyFile(ctx context.Context, path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("--ready-file %s did not appear within %s", path, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
