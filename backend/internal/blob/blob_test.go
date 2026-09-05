package blob

import (
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

// TestDisk_TraversalIDsStayInsideDir proves an id of exactly "." or ".."
// (the only sanitized values idSanitize's charset lets through unchanged
// that carry special meaning to filepath.Join) can never resolve outside
// the blob directory — see bytesPath's own doc comment for the mechanism.
func TestDisk_TraversalIDsStayInsideDir(t *testing.T) {
	dir := t.TempDir()
	d, err := NewDisk(dir)
	if err != nil {
		t.Fatalf("NewDisk: %v", err)
	}
	for _, id := range []string{"..", ".", ""} {
		bp := d.bytesPath(id)
		rel, err := filepath.Rel(dir, bp)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("bytesPath(%q) = %q, escapes blob dir %q (rel=%q, err=%v)", id, bp, dir, rel, err)
		}
		if _, _, err := d.Get(id); err == nil {
			t.Fatalf("Get(%q) unexpectedly succeeded — want not-found, since nothing was ever Put at that id", id)
		}
		if _, ok := d.Meta(id); ok {
			t.Fatalf("Meta(%q) unexpectedly found a sidecar", id)
		}
	}
}

// TestDisk_SanitizesSeparatorsInID proves a slash embedded in an id can
// never be used to address a subdirectory or an already-cleaned traversal
// (e.g. "../etc/passwd", whose slashes idSanitize replaces before the ".."
// component check ever sees them).
func TestDisk_SanitizesSeparatorsInID(t *testing.T) {
	dir := t.TempDir()
	d, err := NewDisk(dir)
	if err != nil {
		t.Fatalf("NewDisk: %v", err)
	}
	bp := d.bytesPath("../../../etc/passwd")
	rel, err := filepath.Rel(dir, bp)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("bytesPath with embedded traversal = %q, escapes blob dir %q (rel=%q, err=%v)", bp, dir, rel, err)
	}
	if filepath.Dir(bp) != dir {
		t.Fatalf("bytesPath with embedded traversal = %q, want a direct child of %q", bp, dir)
	}
}
