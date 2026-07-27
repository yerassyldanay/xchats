package provenance

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNewRunDir_UniqueAndCreated(t *testing.T) {
	root := t.TempDir()
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		id, dir, err := NewRunDir(root)
		if err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
		if seen[id] {
			t.Fatalf("duplicate run id: %s", id)
		}
		seen[id] = true
		if filepath.Join(root, id) != dir {
			t.Fatalf("dir mismatch: %s vs %s", dir, filepath.Join(root, id))
		}
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("dir not created: %s", dir)
		}
	}
}

// TestNewRunDir_ConcurrentCallsNeverCollide fires many goroutines at the same runs
// root simultaneously — the real scenario this exists for: two harness processes (or
// two goroutines in a future worker pool) starting in the same second must never be
// handed the same run directory.
func TestNewRunDir_ConcurrentCallsNeverCollide(t *testing.T) {
	root := t.TempDir()
	const n = 50
	ids := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			id, _, err := NewRunDir(root)
			ids[i] = id
			errs[i] = err
		}(i)
	}
	wg.Wait()

	seen := map[string]bool{}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
		if seen[ids[i]] {
			t.Fatalf("goroutine %d: duplicate run id %s", i, ids[i])
		}
		seen[ids[i]] = true
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != n {
		t.Fatalf("expected %d run dirs, found %d", n, len(entries))
	}
}

func TestStagedRun_IsHiddenUntilPublished(t *testing.T) {
	root := t.TempDir()

	id, stagedDir, err := NewStagedRunDir(root)
	if err != nil {
		t.Fatal(err)
	}
	wantStaged := filepath.Join(root, IncompleteRunsDirName, id)
	if stagedDir != wantStaged {
		t.Fatalf("staged dir = %s, want %s", stagedDir, wantStaged)
	}
	if _, err := os.Stat(filepath.Join(root, id)); !os.IsNotExist(err) {
		t.Fatalf("top-level run must not exist before publication, stat err = %v", err)
	}

	finalDir, err := PublishStagedRun(root, id, stagedDir)
	if err != nil {
		t.Fatal(err)
	}
	if finalDir != filepath.Join(root, id) {
		t.Fatalf("final dir = %s, want %s", finalDir, filepath.Join(root, id))
	}
	if info, err := os.Stat(finalDir); err != nil || !info.IsDir() {
		t.Fatalf("published run not found at %s: %v", finalDir, err)
	}
	if _, err := os.Stat(stagedDir); !os.IsNotExist(err) {
		t.Fatalf("staged dir must be gone after publication, stat err = %v", err)
	}
}

func TestPublishStagedRun_RejectsUnrelatedDirectory(t *testing.T) {
	root := t.TempDir()
	unrelated := filepath.Join(root, "do-not-move")
	if err := os.Mkdir(unrelated, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := PublishStagedRun(root, "run-id", unrelated); err == nil {
		t.Fatal("want unrelated source path rejected")
	}
	if info, err := os.Stat(unrelated); err != nil || !info.IsDir() {
		t.Fatalf("unrelated directory was changed: %v", err)
	}
}

func TestAtomicWriteFile_WritesCompleteFileNoLeftoverTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	want := []byte(`{"hello":"world"}`)
	if err := AtomicWriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 entry (no leftover temp file), got %d: %v", len(entries), entries)
	}
}

func TestAtomicWriteFile_Overwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := AtomicWriteFile(path, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteFile(path, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "second" {
		t.Fatalf("got %q, want %q", got, "second")
	}
}
