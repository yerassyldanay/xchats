package blob

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisk_PutGetRoundTrip(t *testing.T) {
	d, err := NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("NewDisk: %v", err)
	}
	if _, err := d.Put("some-id", []byte("hello"), Meta{Mimetype: "text/plain", FileName: "a.txt"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	data, meta, err := d.Get("some-id")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(data) != "hello" || meta.Mimetype != "text/plain" || meta.FileName != "a.txt" {
		t.Fatalf("Get returned %q %+v, want the stored bytes/meta", data, meta)
	}
}

// TestDisk_TraversalIDsStayInsideDir proves traversal components are reduced
// to ordinary direct-child names before they reach the root-scoped file API.
func TestDisk_TraversalIDsStayInsideDir(t *testing.T) {
	d, err := NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("NewDisk: %v", err)
	}
	for _, id := range []string{"..", ".", ""} {
		name := blobName(id)
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
			t.Fatalf("blobName(%q) = %q, want a safe direct-child name", id, name)
		}
		if _, _, err := d.Get(id); err == nil {
			t.Fatalf("Get(%q) unexpectedly succeeded — want not-found, since nothing was ever Put at that id", id)
		}
		if _, ok := d.Meta(id); ok {
			t.Fatalf("Meta(%q) unexpectedly found a sidecar", id)
		}
	}
}

// TestDisk_SanitizesSeparatorsInID proves separators embedded in an id cannot
// address a subdirectory or an already-cleaned traversal.
func TestDisk_SanitizesSeparatorsInID(t *testing.T) {
	for _, id := range []string{"../../../etc/passwd", `..\..\secret`} {
		if name := blobName(id); strings.ContainsAny(name, `/\`) {
			t.Fatalf("blobName(%q) = %q, still contains a path separator", id, name)
		}
	}
}

func TestDisk_RootRejectsSymlinksOutsideDir(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "blobs")
	d, err := NewDisk(dir)
	if err != nil {
		t.Fatalf("NewDisk: %v", err)
	}

	outside := filepath.Join(parent, "outside")
	if err := os.WriteFile(outside, []byte("do not touch"), 0o600); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	inside := filepath.Join(dir, blobName("escape"))
	if err := os.Symlink(outside, inside); err != nil {
		t.Skipf("cannot create symlink fixture: %v", err)
	}

	if _, _, err := d.Get("escape"); err == nil {
		t.Fatal("Get followed a symlink outside the blob root")
	}
	if _, err := d.Put("escape", []byte("overwritten"), Meta{}); err == nil {
		t.Fatal("Put followed a symlink outside the blob root")
	}
	got, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("read outside fixture: %v", err)
	}
	if string(got) != "do not touch" {
		t.Fatalf("outside file was modified: %q", got)
	}
}
