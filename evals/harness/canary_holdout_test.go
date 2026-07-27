package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestCanaryHoldout_ParsesAndIsWellFormed confirms canary-holdout/tests.yaml is valid,
// loadable YAML with the expected shape. Deliberately does NOT assert anything about
// what detectLang, judgeOne, or any prompt variant does with its content — writing that
// assertion would itself be "opening" the held-out set for calibration purposes, the
// exact thing sealing it is meant to prevent. See tests.yaml's own header comment.
func TestCanaryHoldout_ParsesAndIsWellFormed(t *testing.T) {
	path := filepath.Join("..", "canary-holdout", "tests.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var tf TestsFile
	if err := yaml.Unmarshal(b, &tf); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if len(tf.Include) != 0 {
		t.Errorf("want no `include:` in the held-out set (must stay self-contained, not pull in content from a bank already used for calibration), got %v", tf.Include)
	}
}

// TestCanaryHoldout_NeverInsideScenariosDir is the structural half of the holdout
// guarantee: cmdRun's -all flag globs scenarios/*/scenario.yaml, so a held-out set that
// lived there could be swept into a billed run by a single future `-all` invocation,
// protected only by nobody having added a scenario.yaml to it yet. Confirms the
// directory is genuinely outside scenarios/, and — belt and suspenders — that no
// scenario.yaml exists inside it even if it were ever moved there by mistake.
func TestCanaryHoldout_NeverInsideScenariosDir(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "scenarios", "canary-holdout")); err == nil {
		t.Fatal("canary-holdout must not also exist inside scenarios/ — that would let -all reach it")
	}
	if _, err := os.Stat(filepath.Join("..", "canary-holdout", "scenario.yaml")); err == nil {
		t.Fatal("canary-holdout/ must never gain a scenario.yaml — that is what would let -all reach it")
	}
	matches, err := filepath.Glob(filepath.Join("..", "scenarios", "*", "scenario.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range matches {
		if filepath.Base(filepath.Dir(m)) == "canary-holdout" {
			t.Fatalf("found a canary-holdout scenario.yaml reachable by -all's glob: %s", m)
		}
	}
}

// TestCanaryHoldout_ZeroMessageOverlapWithAnyOtherBank enforces that the held-out set's
// messages (once real content is added — this passes vacuously today since the set is
// empty) never duplicate a message already used for calibration elsewhere in the repo.
// Scans every common/*.yaml bank and every scenarios/*/tests.yaml file directly (exact
// string match only — no fuzzy/semantic matching, that's out of scope for this guard).
func TestCanaryHoldout_ZeroMessageOverlapWithAnyOtherBank(t *testing.T) {
	holdout := loadRawTestMessages(t, filepath.Join("..", "canary-holdout", "tests.yaml"))

	usedElsewhere := map[string]string{} // message -> which file it came from
	for _, pattern := range []string{
		filepath.Join("..", "common", "*.yaml"),
		filepath.Join("..", "scenarios", "*", "tests.yaml"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range matches {
			for _, msg := range loadRawTestMessages(t, path) {
				usedElsewhere[msg] = path
			}
		}
	}

	for _, msg := range holdout {
		if src, dup := usedElsewhere[msg]; dup {
			t.Errorf("held-out message %q also appears in %s — the canary subset must never share content with anything used during calibration", msg, src)
		}
	}
}

// loadRawTestMessages reads a tests.yaml-shaped file's OWN `tests:` list (not following
// `include:`) and returns every message — used purely for the overlap scan above.
func loadRawTestMessages(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var tf TestsFile
	if err := yaml.Unmarshal(b, &tf); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out []string
	for _, tc := range tf.Tests {
		if tc.Message != "" {
			out = append(out, tc.Message)
		}
	}
	return out
}
