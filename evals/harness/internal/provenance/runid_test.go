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
