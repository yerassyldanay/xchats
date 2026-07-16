package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"xchats-evals-harness/internal/provenance"
)

// cmdLaunch implements `harness launch` — the umbrella over `run` and `extract` that
// groups both families under one launch id, written to runs/launches/<id>.json BEFORE
// any billed call (see provenance.LaunchManifest's doc comment for why that ordering
// matters: an interrupted or partially-failed launch still leaves an honest record of
// what was planned). v1 scope: -all only (every scenario + every extraction case),
// matching each underlying command's own -all flag — a partial-scope launch (specific
// scenarios/cases) isn't supported yet; run `run`/`extract` directly for that, with
// their own -launch <id> flag if you want it grouped under a launch you minted by hand.
//
// Both members are attempted regardless of whether the first failed — a chat-eval bug
// must not block getting extraction data, and vice versa — so a launch's final status
// can legitimately be "partial," not just "complete" or "failed."
func cmdLaunch(args []string) error {
	fs := flag.NewFlagSet("launch", flag.ExitOnError)
	all := fs.Bool("all", false, "run every scenario AND every extraction case under this one launch (required in v1)")
	noCache := fs.Bool("no-cache", false, "pass -no-cache to the scenario member run")
	modelsPath := fs.String("models-file", "models.yaml", "path to models.yaml")
	expectCalls := fs.Int("expect-calls", 0, "if >0, hard-fail before spending anything unless the COMBINED (scenario + extract) resolved call count matches exactly")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*all {
		return fmt.Errorf("launch: -all is required in this version (a partial-scope launch isn't supported yet — run `run`/`extract` directly for that)")
	}

	// Pre-flight: resolve BOTH families' call counts before minting the launch
	// manifest or spending anything — mirrors cmdRun's own pass-1 (render every
	// scenario, count tests) plus the analogous count for extraction.
	scenarioCalls, err := preflightScenarioCallCount(*modelsPath)
	if err != nil {
		return fmt.Errorf("launch: scenario pre-flight: %w", err)
	}
	extractCalls, err := preflightExtractCallCount(*modelsPath)
	if err != nil {
		return fmt.Errorf("launch: extract pre-flight: %w", err)
	}
	total := scenarioCalls + extractCalls
	fmt.Printf("launch: scenario=%d + extract=%d => %d combined billed calls\n", scenarioCalls, extractCalls, total)
	if *expectCalls > 0 && total != *expectCalls {
		return fmt.Errorf("launch: resolved %d combined calls, -expect-calls wanted %d — refusing to spend anything; adjust -models-file/-expect-calls if this is intentional", total, *expectCalls)
	}

	lm, err := provenance.NewLaunchManifest("runs", []string{"scenario", "extract"},
		provenance.LaunchCallCount{Scenario: scenarioCalls, Extract: extractCalls, Total: total})
	if err != nil {
		return err
	}
	fmt.Printf("launch: %s (runs/launches/%s.json)\n", lm.LaunchID, lm.LaunchID)

	fmt.Println("--- scenario member ---")
	runArgs := []string{"-all", "-models-file", *modelsPath, "-launch", lm.LaunchID}
	if *noCache {
		runArgs = append(runArgs, "-no-cache")
	}
	lm.SetMember("scenario", provenance.LaunchMemberRunning, "", "")
	_ = provenance.WriteLaunchManifest("runs", lm)
	scenarioRunID, runErr := cmdRun(runArgs)
	if runErr != nil {
		lm.SetMember("scenario", provenance.LaunchMemberFailed, scenarioRunID, runErr.Error())
		fmt.Fprintf(os.Stderr, "launch: scenario member failed: %v\n", runErr)
	} else {
		lm.SetMember("scenario", provenance.LaunchMemberComplete, scenarioRunID, "")
	}
	_ = provenance.WriteLaunchManifest("runs", lm)

	fmt.Println("--- extract member ---")
	extractArgs := []string{"-all", "-models-file", *modelsPath, "-launch", lm.LaunchID}
	lm.SetMember("extract", provenance.LaunchMemberRunning, "", "")
	_ = provenance.WriteLaunchManifest("runs", lm)
	extractRunID, extractErr := cmdExtract(extractArgs)
	if extractErr != nil {
		lm.SetMember("extract", provenance.LaunchMemberFailed, extractRunID, extractErr.Error())
	} else {
		lm.SetMember("extract", provenance.LaunchMemberComplete, extractRunID, "")
	}

	lm.Finish()
	if err := provenance.WriteLaunchManifest("runs", lm); err != nil {
		return err
	}
	fmt.Printf("launch %s: %s\n", lm.LaunchID, lm.Status)

	switch {
	case runErr != nil && extractErr != nil:
		return fmt.Errorf("launch: both members failed: scenario: %v; extract: %v", runErr, extractErr)
	case runErr != nil:
		return fmt.Errorf("launch: scenario member failed (extract completed): %w", runErr)
	case extractErr != nil:
		return fmt.Errorf("launch: extract member failed (scenario completed): %w", extractErr)
	}
	return nil
}

// resolveExpectedExtractCalls is extraction's analogue to run.go's
// resolveExpectedCalls — factored out as a pure function for the same reason
// (independently testable, not just exercised inline inside a pre-flight helper).
func resolveExpectedExtractCalls(numCases, numModels, numPrompts int) int {
	return numCases * numModels * numPrompts
}

// preflightScenarioCallCount mirrors cmdRun's own pass-1 (render every -all scenario,
// count resolved tests) against every model in modelsPath, at repeats=1 — launch's v1
// scope has no -repeats of its own. Renders as a side effect (same as cmdRun's own
// pass-1); idempotent, so calling it before minting the launch manifest is safe.
func preflightScenarioCallCount(modelsPath string) (int, error) {
	matches, err := filepath.Glob(filepath.Join("scenarios", "*", "scenario.yaml"))
	if err != nil {
		return 0, err
	}
	models, err := loadModels(modelsPath)
	if err != nil {
		return 0, fmt.Errorf("load %s: %w", modelsPath, err)
	}
	totalTests := 0
	for _, m := range matches {
		sd := filepath.Dir(m)
		if err := renderScenario(sd, modelsPath, ""); err != nil {
			return 0, fmt.Errorf("render %s: %w", sd, err)
		}
		var resolved ResolvedTests
		if err := readJSON(filepath.Join(sd, "generated", "resolved_tests.json"), &resolved); err != nil {
			return 0, fmt.Errorf("read resolved tests for %s: %w", sd, err)
		}
		totalTests += len(resolved.Tests)
	}
	return resolveExpectedCalls(totalTests, len(models.Providers), 1), nil
}

// preflightExtractCallCount mirrors cmdExtract's own case/model/prompt resolution
// (every case in cases.yaml, resolveVisionModels' default model set, the default
// -prompt extract@v1) to compute the extraction half of a launch's combined
// pre-flight count — launch's v1 scope has no -prompt/-cases override of its own.
func preflightExtractCallCount(modelsPath string) (int, error) {
	cf, err := loadExtractCases("extract/cases.yaml")
	if err != nil {
		return 0, err
	}
	models, err := resolveVisionModels("", modelsPath)
	if err != nil {
		return 0, err
	}
	prompts, err := provenance.LoadPromptSpecs("prompts", []string{"extract@v1"})
	if err != nil {
		return 0, err
	}
	return resolveExpectedExtractCalls(len(cf.Cases), len(models), len(prompts)), nil
}
