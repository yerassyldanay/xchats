package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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

	runID := time.Now().Format("2006-01-02_15-04-05")
	runDir := filepath.Join("runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return err
	}
	absRunDir, err := filepath.Abs(runDir)
	if err != nil {
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
		if err := runPromptfoo(sd, scenario.Name, absRunDir, *noCache); err != nil {
			return fmt.Errorf("promptfoo eval for %s: %w", scenario.Name, err)
		}
		if err := judgeScenario(sd, absRunDir, *modelsPath); err != nil {
			return fmt.Errorf("judge %s: %w", scenario.Name, err)
		}
	}

	return reportRun(runDir, *modelsPath)
}

func runPromptfoo(scenarioDir, scenarioName, absRunDir string, noCache bool) error {
	genDir := filepath.Join(scenarioDir, "generated")
	outPath := filepath.Join(absRunDir, scenarioName+".results.json")
	args := []string{"--yes", "promptfoo@latest", "eval", "-c", "promptfooconfig.yaml", "-o", outPath}
	if noCache {
		args = append(args, "--no-cache")
	}
	cmd := exec.Command("npx", args...)
	cmd.Dir = genDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
