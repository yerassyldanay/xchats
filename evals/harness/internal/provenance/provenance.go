package provenance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PromptfooVersion pins the exact promptfoo release the harness invokes via npx
// (evals/harness/run.go's runPromptfoo), replacing the earlier "promptfoo@latest" —
// an unpinned tool version means the same scenario+model matrix can silently behave
// differently between two runs for a reason that has nothing to do with the prompt or
// model under test. Bump deliberately, not implicitly.
const PromptfooVersion = "0.121.18"

const manifestSchemaVersion = 1

// ScenarioSnapshotRef records what a run captured for one scenario: everything judge
// needs to re-grade this run's results without depending on the live, mutable
// scenarios/*/generated/ directory (which the next `render` overwrites).
type ScenarioSnapshotRef struct {
	Scenario              string `json:"scenario"`
	ScenarioYAMLSHA256    string `json:"scenario_yaml_sha256"`
	FixtureSHA256         string `json:"fixture_sha256,omitempty"`
	PromptSHA256          string `json:"prompt_sha256"`
	CatalogSHA256         string `json:"catalog_sha256"`
	ResolvedTestsSHA256   string `json:"resolved_tests_sha256"`
	PromptfooConfigSHA256 string `json:"promptfooconfig_sha256"`
	// ResultsSHA256 is filled in once promptfoo has written <scenario>.results.json —
	// that file itself is gitignored (large, reproducible), but its hash lets a reader
	// notice if a committed judged.json no longer matches the raw answers it came from.
	ResultsSHA256 string `json:"results_sha256,omitempty"`
}

// InputSnapshotRef records provenance for one extraction case's image: the ORIGINAL
// file's hash (so "did the source asset change" is answerable) and the PROCESSED
// (scaled/re-encoded) bytes' hash (what the model actually saw) — these differ by
// design (imgscale.go always re-encodes to JPEG), and conflating them would make a
// content change in either step invisible.
type InputSnapshotRef struct {
	CaseID          string `json:"case_id"`
	OriginalSHA256  string `json:"original_sha256"`
	ProcessedSHA256 string `json:"processed_sha256"`
}

// Manifest is runs/<id>/manifest.json — the immutable record of what a run was, so a
// human or agent can look at an old run and know exactly what produced it, without
// trusting that scenarios/*/generated/, extract/cases.yaml, or models.yaml still look
// the same today as they did at run time.
type Manifest struct {
	SchemaVersion int      `json:"schema_version"`
	RunID         string   `json:"run_id"`
	Family        string   `json:"family"` // "scenario" | "extract"
	Command       string   `json:"command"`
	Args          []string `json:"args"`
	// LaunchID groups this run with sibling runs from one `harness launch` invocation
	// (see launch.go) — empty for a run started directly via `run`/`extract` (no
	// -launch flag passed) or from before this field existed. Readers must treat an
	// empty LaunchID as "this run is its own singleton launch," never as an error —
	// see index.go's buildRunsIndexRow for where that fallback is applied.
	LaunchID string `json:"launch_id,omitempty"`

	// Git provenance is best-effort: a missing git binary or a non-repo checkout must
	// never fail an eval run, so every field here is the empty value on any git error.
	GitSHA             string `json:"git_sha,omitempty"`
	GitDirty           bool   `json:"git_dirty"`
	GitStatusPorcelain string `json:"git_status_porcelain,omitempty"`
	// WorktreeDiffPath is relative to the run dir ("worktree.diff"); set only when the
	// worktree was dirty, so a clean run's manifest doesn't imply a diff file exists.
	WorktreeDiffPath string `json:"worktree_diff_path,omitempty"`

	ModelsPath   string `json:"models_path"`
	ModelsSHA256 string `json:"models_sha256"`

	PromptfooVersion string `json:"promptfoo_version,omitempty"`

	Scenarios []ScenarioSnapshotRef `json:"scenarios,omitempty"`

	ExtractCasesPath   string             `json:"extract_cases_path,omitempty"`
	ExtractCasesSHA256 string             `json:"extract_cases_sha256,omitempty"`
	Inputs             []InputSnapshotRef `json:"inputs,omitempty"`

	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at,omitempty"`

	// Retry-repair provenance — set ONLY on a derivative run created by `harness retry`
	// (retry.go, package main). A derivative mixes rows produced under TWO different
	// model configs (the parent run's original config, and whatever models.yaml was
	// active when the retry ran), so a single ModelsSHA256/models.yaml snapshot cannot
	// honestly describe it — these fields plus the dual snapshots/models.parent.yaml +
	// snapshots/models.retry.yaml are what make that auditable. RepairKey is derived
	// from (parent run id, retry models.yaml sha, harness git sha) — the mechanism
	// `harness retry` uses to detect it already repaired this exact parent under this
	// exact config, so re-running the command doesn't silently bill retries twice.
	RepairedFrom         string `json:"repaired_from,omitempty"`
	RepairKey            string `json:"repair_key,omitempty"`
	ParentManifestSHA256 string `json:"parent_manifest_sha256,omitempty"`
	ParentResultsSHA256  string `json:"parent_results_sha256,omitempty"`
	ParentModelsSHA256   string `json:"parent_models_sha256,omitempty"`
	RetryModelsSHA256    string `json:"retry_models_sha256,omitempty"`
}

// NewManifest builds a Manifest for a fresh run, capturing best-effort git provenance
// (and, if the worktree is dirty, writing runDir/worktree.diff) right away — this must
// happen before any model call, so an interrupted run still leaves an accurate record
// of what code produced whatever partial results exist.
func NewManifest(runDir, runID, family, command string, args []string) *Manifest {
	m := &Manifest{
		SchemaVersion: manifestSchemaVersion,
		RunID:         runID,
		Family:        family,
		Command:       command,
		Args:          append([]string(nil), args...),
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	m.GitSHA = gitCapture("rev-parse", "HEAD")
	status := gitCaptureRaw("status", "--porcelain")
	m.GitStatusPorcelain = string(status)
	m.GitDirty = strings.TrimSpace(m.GitStatusPorcelain) != ""
	if m.GitDirty {
		if diff := gitCaptureRaw("diff", "HEAD", "--binary"); len(diff) > 0 {
			if err := AtomicWriteFile(filepath.Join(runDir, "worktree.diff"), diff, 0o644); err == nil {
				m.WorktreeDiffPath = "worktree.diff"
			}
		}
	}
	return m
}

// gitTimeout bounds every git subprocess the manifest shells out to — provenance
// capture is best-effort and must never let a hung or slow git call stall an eval run.
const gitTimeout = 5 * time.Second

func gitCapture(args ...string) string {
	return strings.TrimSpace(string(gitCaptureRaw(args...)))
}

// gitCaptureRaw runs `git <args>` and returns stdout verbatim (untrimmed — callers
// that need patch bytes, like `diff --binary`, must not lose leading/trailing bytes).
// Any failure (git missing, not a repo, timeout) returns nil: git provenance is
// best-effort and never fails the eval it's describing.
func gitCaptureRaw(args ...string) []byte {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return nil
	}
	return out
}

// WriteManifest atomically (re)writes runDir/manifest.json — safe to call multiple
// times for the same run (e.g. once at start, once with FinishedAt patched in at end).
func WriteManifest(runDir string, m *Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteFile(filepath.Join(runDir, "manifest.json"), b, 0o644)
}

// Finish stamps FinishedAt — call once the run completes, then WriteManifest again.
func (m *Manifest) Finish() {
	m.FinishedAt = time.Now().UTC().Format(time.RFC3339)
}

func SHA256Bytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SnapshotFile copies src's bytes to dst (creating dst's parent directory as needed,
// writing atomically) and returns their sha256 — the one primitive every provenance
// snapshot (scenario files, models.yaml, extract cases) is built from, so "what we
// captured" and "the hash we recorded" can never drift apart from each other.
func SnapshotFile(src, dst string) (sha256hex string, err error) {
	b, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	if err := AtomicWriteFile(dst, b, 0o644); err != nil {
		return "", err
	}
	return SHA256Bytes(b), nil
}

// SnapshotScenario copies the historically-relevant files for one scenario run — its
// scenario.yaml, an optional private schema fixture, and everything render wrote to generated/ — into
// runs/<id>/snapshots/<scenario>/, so judge and report can later read the EXACT
// requirements this run graded against, not whatever scenarios/*/generated/ contains
// by the time someone re-judges it.
func SnapshotScenario(scenarioDir, runDir, scenarioName, fixturePath string) (ScenarioSnapshotRef, error) {
	genDir := filepath.Join(scenarioDir, "generated")
	dstDir := filepath.Join(runDir, "snapshots", scenarioName)
	ref := ScenarioSnapshotRef{Scenario: scenarioName}

	pairs := []struct {
		src string
		dst string
		out *string
	}{
		{filepath.Join(scenarioDir, "scenario.yaml"), filepath.Join(dstDir, "scenario.yaml"), &ref.ScenarioYAMLSHA256},
		{filepath.Join(genDir, "prompt.txt"), filepath.Join(dstDir, "prompt.txt"), &ref.PromptSHA256},
		{filepath.Join(genDir, "catalog.json"), filepath.Join(dstDir, "catalog.json"), &ref.CatalogSHA256},
		{filepath.Join(genDir, "resolved_tests.json"), filepath.Join(dstDir, "resolved_tests.json"), &ref.ResolvedTestsSHA256},
		{filepath.Join(genDir, "promptfooconfig.yaml"), filepath.Join(dstDir, "promptfooconfig.yaml"), &ref.PromptfooConfigSHA256},
	}
	for _, p := range pairs {
		sha, err := SnapshotFile(p.src, p.dst)
		if err != nil {
			return ref, fmt.Errorf("snapshot %s: %w", p.src, err)
		}
		*p.out = sha
	}
	if fixturePath != "" {
		sha, err := SnapshotFile(fixturePath, filepath.Join(dstDir, "kb-fixture.yaml"))
		if err != nil {
			return ref, fmt.Errorf("snapshot %s: %w", fixturePath, err)
		}
		ref.FixtureSHA256 = sha
	}
	return ref, nil
}

// SnapshotDirFor returns this run's own snapshot directory for scenarioName if step 2
// provenance captured one. Legacy runs (created before this feature existed) and a
// standalone `harness judge` invoked against a fresh render have no snapshot — callers
// fall back to the live scenarioDir/generated in that case.
func SnapshotDirFor(runDir, scenarioName string) (dir string, ok bool) {
	d := filepath.Join(runDir, "snapshots", scenarioName)
	info, err := os.Stat(d)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return d, true
}

// SnapshotModelsPath returns this run's own snapshotted models.yaml if present, else
// falls back to modelsPath (today's live file) — same legacy/standalone-invocation
// fallback as SnapshotDirFor, kept separate because models.yaml is one file per run,
// not one per scenario.
func SnapshotModelsPath(runDir, modelsPath string) string {
	p := filepath.Join(runDir, "snapshots", "models.yaml")
	if info, err := os.Stat(p); err == nil && !info.IsDir() {
		return p
	}
	return modelsPath
}
