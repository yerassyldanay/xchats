package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"xchats-evals-harness/internal/provenance"
)

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	scenarioList := fs.String("scenario", "", "comma-separated scenario dirs, e.g. scenarios/shop-current,scenarios/shop-decisions-v1")
	all := fs.Bool("all", false, "run every scenario under scenarios/")
	noCache := fs.Bool("no-cache", false, "pass --no-cache to promptfoo (force fresh calls)")
	modelsPath := fs.String("models", "models.yaml", "path to models.yaml")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if os.Getenv("OPENROUTER_API_KEY") == "" {
		return fmt.Errorf("run: OPENROUTER_API_KEY is not set (this makes real, billed model calls)")
	}

	var scenarioDirs []string
	if *all {
		matches, err := filepath.Glob(filepath.Join("scenarios", "*", "scenario.yaml"))
		if err != nil {
			return err
		}
		for _, m := range matches {
			scenarioDirs = append(scenarioDirs, filepath.Dir(m))
		}
	} else if *scenarioList != "" {
		for _, s := range strings.Split(*scenarioList, ",") {
			scenarioDirs = append(scenarioDirs, strings.TrimSpace(s))
		}
	}
	if len(scenarioDirs) == 0 {
		return fmt.Errorf("run: pass -scenario <dir>[,<dir>...] or -all")
	}

	runID, runDir, err := provenance.NewRunDir("runs")
	if err != nil {
		return err
	}
	absRunDir, err := filepath.Abs(runDir)
	if err != nil {
		return err
	}

	manifest := provenance.NewManifest(runDir, runID, "scenario", "run", args)
	manifest.PromptfooVersion = provenance.PromptfooVersion
	manifest.ModelsPath = *modelsPath
	if sha, err := provenance.SnapshotFile(*modelsPath, filepath.Join(runDir, "snapshots", "models.yaml")); err != nil {
		return fmt.Errorf("snapshot %s: %w", *modelsPath, err)
	} else {
		manifest.ModelsSHA256 = sha
	}
	if err := provenance.WriteManifest(runDir, manifest); err != nil {
		return err
	}

	for _, sd := range scenarioDirs {
		fmt.Printf("--- %s ---\n", sd)
		if err := renderScenario(sd, *modelsPath); err != nil {
			return fmt.Errorf("render %s: %w", sd, err)
		}
		scenario, err := loadScenario(sd)
		if err != nil {
			return err
		}

		ref, err := provenance.SnapshotScenario(sd, runDir, scenario.Name)
		if err != nil {
			return fmt.Errorf("snapshot %s: %w", scenario.Name, err)
		}

		if err := runPromptfoo(sd, scenario.Name, absRunDir, *noCache); err != nil {
			return fmt.Errorf("promptfoo eval for %s: %w", scenario.Name, err)
		}
		if sha, err := provenance.SHA256File(filepath.Join(runDir, scenario.Name+".results.json")); err == nil {
			ref.ResultsSHA256 = sha
		}
		manifest.Scenarios = append(manifest.Scenarios, ref)
		if err := provenance.WriteManifest(runDir, manifest); err != nil {
			return err
		}

		if err := judgeScenario(sd, absRunDir, *modelsPath); err != nil {
			return fmt.Errorf("judge %s: %w", scenario.Name, err)
		}
	}

	manifest.Finish()
	if err := provenance.WriteManifest(runDir, manifest); err != nil {
		return err
	}

	return reportRun(runDir, *modelsPath)
}

func runPromptfoo(scenarioDir, scenarioName, absRunDir string, noCache bool) error {
	genDir := filepath.Join(scenarioDir, "generated")
	outPath := filepath.Join(absRunDir, scenarioName+".results.json")
	args := []string{"--yes", "promptfoo@" + provenance.PromptfooVersion, "eval", "-c", "promptfooconfig.yaml", "-o", outPath}
	if noCache {
		args = append(args, "--no-cache")
	}
	cmd := exec.Command("npx", args...)
	cmd.Dir = genDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
