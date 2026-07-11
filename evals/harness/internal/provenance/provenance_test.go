package provenance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readManifestJSON(t *testing.T, path string) Manifest {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSnapshotFile_CopiesAndHashesContent(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	src := filepath.Join(srcDir, "a.txt")
	if err := os.WriteFile(src, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dstDir, "nested", "a.txt")

	sha, err := SnapshotFile(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	want, err := SHA256File(src)
	if err != nil {
		t.Fatal(err)
	}
	if sha != want {
		t.Fatalf("snapshotFile hash %s != sha256File hash %s", sha, want)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Fatalf("copied content mismatch: %q", got)
	}
}

func TestSnapshotScenario_CopiesAllFiveFilesWithMatchingHashes(t *testing.T) {
	scenarioDir := t.TempDir()
	genDir := filepath.Join(scenarioDir, "generated")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(scenarioDir, "scenario.yaml"):   "name: x\n",
		filepath.Join(genDir, "prompt.txt"):           "prompt body",
		filepath.Join(genDir, "catalog.json"):         `{"tokens":[]}`,
		filepath.Join(genDir, "resolved_tests.json"):  `{"tests":[]}`,
		filepath.Join(genDir, "promptfooconfig.yaml"): "providers: []\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	runDir := t.TempDir()
	ref, err := SnapshotScenario(scenarioDir, runDir, "x")
	if err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		name string
		got  string
		src  string
	}{
		{"scenario.yaml", ref.ScenarioYAMLSHA256, filepath.Join(scenarioDir, "scenario.yaml")},
		{"prompt.txt", ref.PromptSHA256, filepath.Join(genDir, "prompt.txt")},
		{"catalog.json", ref.CatalogSHA256, filepath.Join(genDir, "catalog.json")},
		{"resolved_tests.json", ref.ResolvedTestsSHA256, filepath.Join(genDir, "resolved_tests.json")},
		{"promptfooconfig.yaml", ref.PromptfooConfigSHA256, filepath.Join(genDir, "promptfooconfig.yaml")},
	}
	for _, c := range checks {
		want, err := SHA256File(c.src)
		if err != nil {
			t.Fatal(err)
		}
		if c.got != want {
			t.Errorf("%s: hash mismatch, got %s want %s", c.name, c.got, want)
		}
		dst := filepath.Join(runDir, "snapshots", "x", c.name)
		if _, err := os.Stat(dst); err != nil {
			t.Errorf("%s: not copied to %s: %v", c.name, dst, err)
		}
	}

	dir, ok := SnapshotDirFor(runDir, "x")
	if !ok {
		t.Fatal("snapshotDirFor: expected snapshot to be found")
	}
	if dir != filepath.Join(runDir, "snapshots", "x") {
		t.Fatalf("unexpected snapshot dir: %s", dir)
	}
	if _, ok := SnapshotDirFor(runDir, "does-not-exist"); ok {
		t.Fatal("snapshotDirFor: expected no snapshot for an unknown scenario")
	}
}

func TestSnapshotModelsPath_FallsBackWhenNoSnapshot(t *testing.T) {
	runDir := t.TempDir()
	got := SnapshotModelsPath(runDir, "live-models.yaml")
	if got != "live-models.yaml" {
		t.Fatalf("want fallback to live path, got %s", got)
	}

	snapPath := filepath.Join(runDir, "snapshots", "models.yaml")
	if err := os.MkdirAll(filepath.Dir(snapPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapPath, []byte("providers: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = SnapshotModelsPath(runDir, "live-models.yaml")
	if got != snapPath {
		t.Fatalf("want snapshot path %s, got %s", snapPath, got)
	}
}

func TestManifest_WriteAndFinishRoundTrip(t *testing.T) {
	runDir := t.TempDir()
	m := NewManifest(runDir, "2026-01-01_00-00-00-abcd", "extract", "extract", []string{"-case", "foo"})
	if m.SchemaVersion != manifestSchemaVersion {
		t.Fatalf("want schema version %d, got %d", manifestSchemaVersion, m.SchemaVersion)
	}
	if m.StartedAt == "" {
		t.Fatal("want StartedAt to be set")
	}
	if m.FinishedAt != "" {
		t.Fatal("want FinishedAt unset before finish()")
	}
	if err := WriteManifest(runDir, m); err != nil {
		t.Fatal(err)
	}

	loaded := readManifestJSON(t, filepath.Join(runDir, "manifest.json"))
	if loaded.RunID != m.RunID || loaded.Family != "extract" {
		t.Fatalf("round-trip mismatch: %+v", loaded)
	}

	m.Finish()
	if m.FinishedAt == "" {
		t.Fatal("want FinishedAt set after finish()")
	}
	if err := WriteManifest(runDir, m); err != nil {
		t.Fatal(err)
	}
	loaded = readManifestJSON(t, filepath.Join(runDir, "manifest.json"))
	if loaded.FinishedAt == "" {
		t.Fatal("want FinishedAt persisted after rewrite")
	}
}

// TestGitCaptureRaw_NeverPanicsOutsideRepo exercises the best-effort contract: git
// provenance capture must degrade to empty values, never fail the run, even where git
// itself might error (a bogus subcommand here stands in for "git missing"/"not a repo"
// without actually needing to fake either).
func TestGitCaptureRaw_NeverPanicsOutsideRepo(t *testing.T) {
	out := gitCaptureRaw("not-a-real-git-subcommand")
	if out != nil {
		t.Fatalf("want nil on git error, got %q", out)
	}
	if got := gitCapture("not-a-real-git-subcommand"); got != "" {
		t.Fatalf("want empty string on git error, got %q", got)
	}
}
