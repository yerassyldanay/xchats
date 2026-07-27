package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"xchats-evals-harness/internal/provenance"
)

// cmdRun returns the fresh run's id on success (empty on any error) so a caller like
// cmdLaunch can record it as that family's member without re-deriving it from
// directory-listing side effects.
func cmdRun(args []string) (runID string, err error) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	scenarioList := fs.String("scenario", "", "comma-separated scenario dirs, e.g. scenarios/shop-current,scenarios/shop-decisions-v1")
	all := fs.Bool("all", false, "run every scenario under scenarios/")
	noCache := fs.Bool("no-cache", false, "pass --no-cache to promptfoo (force fresh calls)")
	modelsPath := fs.String("models-file", "models.yaml", "path to models.yaml")
	modelsFilter := fs.String("models", "", "comma-separated model ids to run (default: providers marked default:true in models.yaml, or every provider if none are marked; pass \"all\" for every provider explicitly)")
	expectCalls := fs.Int("expect-calls", 0, "if >0, hard-fail before spending anything unless the resolved (tests x models x repeats) call count matches exactly — a deliberate confirmation gate for billed runs")
	repeats := fs.Int("repeats", 1, "run every (test, model) pair this many times (formalized sample sizes: 3 uncached repetitions for screening, 5 for a finalist's 15-intent bank) — requires -no-cache when >1")
	launchID := fs.String("launch", "", "group this run under an existing launch id (see `harness launch`) — leave empty for a standalone run, which is then its own singleton launch")
	retryMedia := fs.Bool("retry-media", false, "opt-in (default off): after each scenario's billed pass, retry IN PLACE (within this same run, before judging/publishing) any row whose media_files_to_send named a nonexistent or no-longer-resolvable media reference — labeled retry_reason=\"media_not_found\" in judged.json. See `harness retry` for the post-hoc, derivative-run alternative against an already-published run.")
	baseURL := fs.String("base-url", "", "override the OpenRouter base URL used by -retry-media's retry calls (default: $EVAL_BASE_URL, else https://openrouter.ai/api/v1) — same convention as `harness retry`")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if err := validateRepeats(*repeats, *noCache); err != nil {
		return "", err
	}
	if os.Getenv("OPENROUTER_API_KEY") == "" {
		return "", fmt.Errorf("run: OPENROUTER_API_KEY is not set (this makes real, billed model calls)")
	}

	var scenarioDirs []string
	if *all {
		matches, err := filepath.Glob(filepath.Join("scenarios", "*", "scenario.yaml"))
		if err != nil {
			return "", err
		}
		for _, m := range matches {
			sd := filepath.Dir(m)
			sc, err := loadScenario(sd)
			if err != nil {
				return "", fmt.Errorf("load %s: %w", sd, err)
			}
			// Archived scenarios are silently skipped (logged, not an error) during -all
			// glob expansion — -all must keep working forever with archived dirs present,
			// exactly the zero-new-spend guarantee archival exists for. See
			// ScenarioConfig.Archived's doc comment.
			if sc.Archived {
				fmt.Printf("run: skipping archived scenario %s (%s)\n", sd, sc.ArchivedReason)
				continue
			}
			scenarioDirs = append(scenarioDirs, sd)
		}
	} else if *scenarioList != "" {
		for _, s := range strings.Split(*scenarioList, ",") {
			sd := strings.TrimSpace(s)
			sc, err := loadScenario(sd)
			if err != nil {
				return "", fmt.Errorf("load %s: %w", sd, err)
			}
			// Unlike -all's silent skip, an EXPLICIT -scenario request naming an archived
			// scenario is a mistake worth failing loudly before any spend, not a request to
			// quietly honor.
			if sc.Archived {
				return "", fmt.Errorf("run: scenario %s is archived (%s) — refusing to run it; remove it from -scenario if this is intentional", sd, sc.ArchivedReason)
			}
			scenarioDirs = append(scenarioDirs, sd)
		}
	}
	if len(scenarioDirs) == 0 {
		return "", fmt.Errorf("run: pass -scenario <dir>[,<dir>...] or -all")
	}

	// Pass 1 (free): render every scenario and resolve exactly how many (test x model)
	// calls this invocation is about to bill, BEFORE any promptfoo call runs. Render is
	// idempotent, so re-rendering in the pass-2 loop below is intentional, not wasted
	// duplicate work worth avoiding — it keeps this counting pass a pure, side-effect-only
	// read of what pass 2 will do.
	models, err := loadModels(*modelsPath)
	if err != nil {
		return "", fmt.Errorf("load %s: %w", *modelsPath, err)
	}
	filteredModels, err := filterProviders(models, *modelsFilter)
	if err != nil {
		return "", err
	}
	if len(filteredModels) == 0 {
		return "", fmt.Errorf("run: no models selected (empty models.yaml or -models matched nothing)")
	}

	totalTests := 0
	for _, sd := range scenarioDirs {
		if err := renderScenario(sd, *modelsPath, *modelsFilter); err != nil {
			return "", fmt.Errorf("render %s: %w", sd, err)
		}
		var resolved ResolvedTests
		if err := readJSON(filepath.Join(sd, "generated", "resolved_tests.json"), &resolved); err != nil {
			return "", fmt.Errorf("read resolved tests for %s: %w", sd, err)
		}
		totalTests += len(resolved.Tests)
	}
	totalCalls := resolveExpectedCalls(totalTests, len(filteredModels), *repeats)
	if err := validateResolvedRunSize(totalTests, totalCalls); err != nil {
		return "", err
	}
	if *repeats > 1 {
		fmt.Printf("run: %d scenario(s), %d test(s) total, %d model(s), %d repeat(s) => %d billed calls (uncached — every repeat is a fresh call)\n",
			len(scenarioDirs), totalTests, len(filteredModels), *repeats, totalCalls)
	} else {
		fmt.Printf("run: %d scenario(s), %d test(s) total, %d model(s) => %d billed calls (cache hits may reduce this)\n",
			len(scenarioDirs), totalTests, len(filteredModels), totalCalls)
	}
	if *expectCalls > 0 && totalCalls != *expectCalls {
		return "", fmt.Errorf("run: resolved %d calls, -expect-calls wanted %d — refusing to spend anything; adjust -scenario/-models/-repeats/-expect-calls if this is intentional", totalCalls, *expectCalls)
	}

	var runDir string
	runID, runDir, err = provenance.NewStagedRunDir("runs")
	if err != nil {
		return "", err
	}
	absRunDir, err := filepath.Abs(runDir)
	if err != nil {
		return runID, err
	}

	manifest := provenance.NewManifest(runDir, runID, "scenario", "run", args)
	manifest.PromptfooVersion = provenance.PromptfooVersion
	manifest.ModelsPath = *modelsPath
	manifest.LaunchID = *launchID
	if sha, err := provenance.SnapshotFile(*modelsPath, filepath.Join(runDir, "snapshots", "models.yaml")); err != nil {
		return runID, fmt.Errorf("snapshot %s: %w", *modelsPath, err)
	} else {
		manifest.ModelsSHA256 = sha
	}
	if err := provenance.WriteManifest(runDir, manifest); err != nil {
		return runID, err
	}

	// Pass 2: the actual (billed) work.
	for _, sd := range scenarioDirs {
		fmt.Printf("--- %s ---\n", sd)
		if err := renderScenario(sd, *modelsPath, *modelsFilter); err != nil {
			return runID, fmt.Errorf("render %s: %w", sd, err)
		}
		scenario, err := loadScenario(sd)
		if err != nil {
			return runID, err
		}

		fixturePath := ""
		if scenario.Pipeline == "schema_kb_v1" {
			fixturePath = filepath.Join(sd, scenario.Data)
		}
		ref, err := provenance.SnapshotScenario(sd, runDir, scenario.Name, fixturePath)
		if err != nil {
			return runID, fmt.Errorf("snapshot %s: %w", scenario.Name, err)
		}

		if err := runPromptfoo(sd, scenario.Name, absRunDir, *noCache, *repeats); err != nil {
			return runID, fmt.Errorf("promptfoo eval for %s: %w", scenario.Name, err)
		}
		if *retryMedia {
			base := *baseURL
			if base == "" {
				base = envOrDefault("EVAL_BASE_URL", "https://openrouter.ai/api/v1")
			}
			retried, err := retryMediaNotFoundInPlace(sd, absRunDir, scenario, models, manifest.ModelsSHA256, base, os.Getenv("OPENROUTER_API_KEY"))
			if err != nil {
				return runID, fmt.Errorf("retry-media %s: %w", scenario.Name, err)
			}
			if retried > 0 {
				fmt.Printf("retry-media: retried %d media-not-found row(s) in %s\n", retried, scenario.Name)
			}
		}
		if sha, err := provenance.SHA256File(filepath.Join(runDir, scenario.Name+".results.json")); err == nil {
			ref.ResultsSHA256 = sha
		}
		manifest.Scenarios = append(manifest.Scenarios, ref)
		if err := provenance.WriteManifest(runDir, manifest); err != nil {
			return runID, err
		}

		if err := judgeScenario(sd, absRunDir, *modelsPath); err != nil {
			return runID, fmt.Errorf("judge %s: %w", scenario.Name, err)
		}
	}

	manifest.Finish()
	if err := provenance.WriteManifest(runDir, manifest); err != nil {
		return runID, err
	}

	publishedRunDir, err := provenance.PublishStagedRun("runs", runID, runDir)
	if err != nil {
		return runID, err
	}
	return runID, reportRun(publishedRunDir, *modelsPath)
}

func runPromptfoo(scenarioDir, scenarioName, absRunDir string, noCache bool, repeats int) error {
	genDir := filepath.Join(scenarioDir, "generated")
	outPath := filepath.Join(absRunDir, scenarioName+".results.json")
	args := []string{"--yes", "promptfoo@" + provenance.PromptfooVersion, "eval", "-c", "promptfooconfig.yaml", "-o", outPath}
	if noCache {
		args = append(args, "--no-cache")
	}
	if repeats > 1 {
		args = append(args, "--repeat", strconv.Itoa(repeats))
	}
	cmd := exec.Command("npx", args...)
	cmd.Dir = genDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// resolveExpectedCalls is the ONE place the pre-flight billed-call count is computed —
// factored out to a pure function so -expect-calls's safety arithmetic is independently
// testable (run_test.go) rather than only exercised inline inside cmdRun.
func resolveExpectedCalls(totalTests, numModels, repeats int) int {
	return totalTests * numModels * repeats
}

// validateResolvedRunSize rejects an empty scenario selection before a run directory
// is minted. An empty test set is a configuration error, not a meaningful 0/0 result.
func validateResolvedRunSize(totalTests, totalCalls int) error {
	if totalTests < 1 {
		return fmt.Errorf("run: resolved zero tests — refusing to create an empty 0/0 run")
	}
	if totalCalls < 1 {
		return fmt.Errorf("run: resolved zero model calls — refusing to create an empty 0/0 run")
	}
	return nil
}

// validateRepeats hard-requires -no-cache whenever -repeats requests more than one call
// per (test, model) pair. promptfoo's --repeat/caching interaction has a documented
// history of surprising behavior (a repeat silently replaying one cached response
// instead of making an independent fresh call) and isn't independently re-verified
// against this repo's pinned promptfoo version (see provenance.PromptfooVersion) without
// a live call — disabling caching entirely sidesteps that ambiguity rather than trusting
// unverified internals, and it's what "uncached repetitions" (the formalized sample-size
// requirement this flag exists for) already asks for regardless. Also rejects repeats<1
// outright — a repeat count only makes sense as a positive integer.
func validateRepeats(repeats int, noCache bool) error {
	if repeats < 1 {
		return fmt.Errorf("run: -repeats must be >= 1, got %d", repeats)
	}
	if repeats > 1 && !noCache {
		return fmt.Errorf("run: -repeats %d requires -no-cache — otherwise repeats beyond the first may replay a cached response instead of an independent sample, silently deflating the variance a Wilson interval depends on", repeats)
	}
	return nil
}
