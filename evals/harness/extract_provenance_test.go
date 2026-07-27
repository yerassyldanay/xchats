package main

import (
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xchats-evals-harness/internal/provenance"
)

// TestCmdExtract_WritesManifestSnapshotsAndInputs is an end-to-end run of `extract`
// against a fake OpenRouter server (no real API key, no spend) that proves step 2's
// provenance wiring actually fires from the real command path, not just from calling
// its helpers directly: manifest.json, snapshots/extract_cases.yaml, snapshots/
// models.yaml, and inputs/<case>.jpg must all exist and be internally consistent after
// one `extract` invocation.
func TestCmdExtract_WritesManifestSnapshotsAndInputs(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	extractDir := filepath.Join(root, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}

	promptDir := filepath.Join(root, "prompts", "extract")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(promptDir, "v1.txt"), []byte("test extraction prompt"), 0o644); err != nil {
		t.Fatal(err)
	}

	imgPath := filepath.Join(extractDir, "synthetic.png")
	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 100, 80))); err != nil {
		t.Fatal(err)
	}
	f.Close()

	casesYAML := "cases:\n  - id: synth\n    file: synthetic.png\n    allowed_numbers: []\n"
	if err := os.WriteFile(filepath.Join(extractDir, "cases.yaml"), []byte(casesYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	modelsYAML := "pricing_source: test\npricing_checked_at: \"2026-01-01\"\nproviders:\n  - id: test/model\n    temperature: 0.3\n    max_tokens: 700\n"
	if err := os.WriteFile(filepath.Join(root, "models.yaml"), []byte(modelsYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := orResponse{Usage: &orUsage{PromptTokens: 10, CompletionTokens: 5}}
		resp.Choices = []orChoice{{}}
		resp.Choices[0].Message.Content = `{"content_kind":"other","summary":"x","extracted_text":"","language":"none","visibility_suggestion":"visible","media_role_hint":"none","relates_to_hint":""}`
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	t.Setenv("OPENROUTER_API_KEY", "test-key")

	_, err = cmdExtract([]string{
		"-cases", filepath.Join("extract", "cases.yaml"),
		"-models", "test/model",
		"-models-file", "models.yaml",
		"-base-url", srv.URL,
		"-env", filepath.Join(root, "does-not-exist.env"),
	})
	if err != nil {
		t.Fatalf("cmdExtract: %v", err)
	}

	allRuns, err := filepath.Glob(filepath.Join(root, "runs", "*"))
	if err != nil {
		t.Fatal(err)
	}
	var runDirs []string
	for _, d := range allRuns {
		if filepath.Base(d) == provenance.IncompleteRunsDirName {
			continue
		}
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			runDirs = append(runDirs, d)
		}
	}
	if len(runDirs) != 1 {
		t.Fatalf("want exactly 1 run dir, got %d: %v", len(runDirs), runDirs)
	}
	runDir := runDirs[0]

	var m provenance.Manifest
	if err := readJSON(filepath.Join(runDir, "manifest.json"), &m); err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	if m.Family != "extract" {
		t.Errorf("want family=extract, got %s", m.Family)
	}
	if m.FinishedAt == "" {
		t.Error("want FinishedAt set on a completed run")
	}
	if m.ExtractCasesSHA256 == "" {
		t.Error("want ExtractCasesSHA256 set")
	}
	if wantSHA, _ := provenance.SHA256File(filepath.Join(extractDir, "cases.yaml")); wantSHA != m.ExtractCasesSHA256 {
		t.Errorf("ExtractCasesSHA256 mismatch: manifest=%s actual=%s", m.ExtractCasesSHA256, wantSHA)
	}
	if wantSHA, _ := provenance.SHA256File(filepath.Join(root, "models.yaml")); wantSHA != m.ModelsSHA256 {
		t.Errorf("ModelsSHA256 mismatch: manifest=%s actual=%s", m.ModelsSHA256, wantSHA)
	}
	if len(m.Inputs) != 1 {
		t.Fatalf("want 1 input ref, got %d", len(m.Inputs))
	}
	in := m.Inputs[0]
	if in.CaseID != "synth" {
		t.Errorf("want case_id=synth, got %s", in.CaseID)
	}
	if wantSHA, _ := provenance.SHA256File(imgPath); wantSHA != in.OriginalSHA256 {
		t.Errorf("OriginalSHA256 mismatch: manifest=%s actual=%s", in.OriginalSHA256, wantSHA)
	}

	// snapshots/ exist and match what was hashed.
	snapCases := filepath.Join(runDir, "snapshots", "extract_cases.yaml")
	if got, err := os.ReadFile(snapCases); err != nil || string(got) != casesYAML {
		t.Errorf("snapshots/extract_cases.yaml mismatch or missing: err=%v got=%q", err, got)
	}
	snapModels := filepath.Join(runDir, "snapshots", "models.yaml")
	if got, err := os.ReadFile(snapModels); err != nil || string(got) != modelsYAML {
		t.Errorf("snapshots/models.yaml mismatch or missing: err=%v got=%q", err, got)
	}

	// inputs/<case>.jpg exists, is a re-encoded JPEG (not a byte-copy of the PNG), and
	// its hash matches the manifest's ProcessedSHA256.
	inputPath := filepath.Join(runDir, "inputs", "synth.jpg")
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read %s: %v", inputPath, err)
	}
	if provenance.SHA256Bytes(inputBytes) != in.ProcessedSHA256 {
		t.Errorf("ProcessedSHA256 mismatch: manifest=%s actual=%s", in.ProcessedSHA256, provenance.SHA256Bytes(inputBytes))
	}
	if in.OriginalSHA256 == in.ProcessedSHA256 {
		t.Error("original and processed hashes should differ (PNG source, JPEG re-encode)")
	}

	// The output filename now carries the prompt version segment (step 3), and the
	// result JSON itself records which prompt + preprocessor produced it.
	outPath := filepath.Join(runDir, "extract_outputs", "synth__test_model__extract-v1.json")
	outBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("expected extract_outputs file %s: %v", outPath, err)
	}
	var res extractRunResult
	if err := json.Unmarshal(outBytes, &res); err != nil {
		t.Fatal(err)
	}
	if res.Prompt.Name != "extract" || res.Prompt.Version != 1 {
		t.Errorf("want prompt extract@v1, got %+v", res.Prompt)
	}
	wantPromptSHA, _ := provenance.SHA256File(filepath.Join(promptDir, "v1.txt"))
	if res.Prompt.SHA256 != wantPromptSHA {
		t.Errorf("prompt sha256 mismatch: got %s want %s", res.Prompt.SHA256, wantPromptSHA)
	}
	if res.Preprocessor != preprocessorImageJPEG85 {
		t.Errorf("want preprocessor %s, got %s", preprocessorImageJPEG85, res.Preprocessor)
	}
	if _, err := os.Stat(filepath.Join(runDir, "EXTRACT.md")); err != nil {
		t.Errorf("expected EXTRACT.md: %v", err)
	}

	// The HTML viewer (step 5) is auto-invoked at the end of a real `extract` run —
	// prove it actually fired via this real command path, not just when called directly.
	htmlBytes, err := os.ReadFile(filepath.Join(runDir, "index.html"))
	if err != nil {
		t.Fatalf("expected auto-generated index.html: %v", err)
	}
	if !strings.Contains(string(htmlBytes), "synth") {
		t.Error("expected index.html to mention the case id")
	}
}
