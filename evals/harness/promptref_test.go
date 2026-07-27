package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"xchats-evals-harness/internal/provenance"
)

// extractPromptV1SHA256 pins evals/prompts/extract/v1.txt to the exact bytes of the
// Go const it replaced (extractSystemPrompt, formerly in extract_types.go) — computed
// once, before the const was deleted, via a throwaway test that dumped the const to a
// file and hashed it. If this test ever fails, the prompt file's content silently
// changed (e.g. a stray editor newline) — the eval would then be testing a different
// prompt than the one every prior run graded against, without anyone deciding that on
// purpose. Bump this hash deliberately (i.e. cut prompts/extract/v2.txt instead) rather
// than editing v1.txt in place.
const extractPromptV1SHA256 = "dcad0020679babe7928f721af04a451b570bbafb6fe0179d5967c593cb21aa44"

func TestExtractPromptV1_MatchesOriginalConstHash(t *testing.T) {
	got, err := provenance.SHA256File(filepath.Join("..", "prompts", "extract", "v1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got != extractPromptV1SHA256 {
		t.Fatalf("prompts/extract/v1.txt sha256 = %s, want %s (pinned to the original extractSystemPrompt const)", got, extractPromptV1SHA256)
	}
}

func TestLoadPrompt_RealExtractV1(t *testing.T) {
	content, ref, err := provenance.LoadPrompt(filepath.Join("..", "prompts"), "extract", 1)
	if err != nil {
		t.Fatal(err)
	}
	if content == "" {
		t.Fatal("want non-empty prompt content")
	}
	if ref.Name != "extract" || ref.Version != 1 {
		t.Fatalf("unexpected ref: %+v", ref)
	}
	if ref.SHA256 != extractPromptV1SHA256 {
		t.Fatalf("ref.SHA256 = %s, want %s", ref.SHA256, extractPromptV1SHA256)
	}
}

func TestPreprocess_UnknownKindIsHardError(t *testing.T) {
	_, _, _, err := preprocess("pdf", "whatever.pdf")
	if err == nil {
		t.Fatal("want error for unimplemented kind, got nil (fail-closed requirement)")
	}
}

func TestPreprocess_EmptyKindDefaultsToImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 50, 40))); err != nil {
		t.Fatal(err)
	}
	f.Close()

	data, mimetype, preprocessorName, err := preprocess("", path)
	if err != nil {
		t.Fatal(err)
	}
	if mimetype != "image/jpeg" {
		t.Errorf("want image/jpeg, got %s", mimetype)
	}
	if preprocessorName != preprocessorImageJPEG85 {
		t.Errorf("want preprocessor %s, got %s", preprocessorImageJPEG85, preprocessorName)
	}
	if len(data) == 0 {
		t.Error("want non-empty processed data")
	}
}
