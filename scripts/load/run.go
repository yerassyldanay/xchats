package main

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// runConfig is the closed-loop load run's own tunables.
type runConfig struct {
	Concurrency int
	Warmup      time.Duration
	Duration    time.Duration
	Seed        int64
}

// runLoad drives cfg.Concurrency workers in a closed loop (each worker
// issues one request, waits for the response, then immediately issues the
// next — no think time, no fixed arrival rate) for Warmup+Duration wall
// time. Results from the first Warmup are discarded; only Duration's worth
// counts toward stats. Every worker's own RNG is seeded deterministically
// from cfg.Seed + its own index, so a fixed concurrency reproduces the same
// sequence of route/parameter choices run to run.
func runLoad(ctx context.Context, cfg runConfig, rnr *runner, weights *weightedRoutes, stats *statsCollector) {
	start := time.Now()
	deadline := start.Add(cfg.Warmup + cfg.Duration)

	var wg sync.WaitGroup
	for workerID := 0; workerID < cfg.Concurrency; workerID++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(cfg.Seed + int64(workerID)))
			for {
				if ctx.Err() != nil || time.Now().After(deadline) {
					return
				}
				route := weights.Pick(rng)

				reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
				begin := time.Now()
				status, err := rnr.execute(reqCtx, route, rng, workerID)
				elapsed := time.Since(begin)
				cancel()

				if time.Since(start) >= cfg.Warmup {
					stats.Record(route, elapsed, status, err)
				}
			}
		}(workerID)
	}
	wg.Wait()
}
