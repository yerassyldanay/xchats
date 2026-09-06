// Package blob is the media byte store. Build 0 ships exactly one adapter:
// local disk (bytes + a JSON sidecar of metadata), behind the Store port.
package blob

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

// Meta is the descriptor stored alongside the bytes.
type Meta struct {
	MediaType string `json:"media_type"`
	Mimetype  string `json:"mimetype"`
	FileName  string `json:"file_name"`
	FileSize  int64  `json:"file_size"`
	// OrgID scopes access for stores fetched by raw blob key rather than
	// through an organization-scoped DB row (see httpapi.handleServeMedia's
	// pending-upload fallback) — empty for callers that don't set it, which
	// a raw-key fetch must treat as "no organization ever authorized this."
	OrgID string `json:"org_id,omitempty"`
}

// Store is the blob port.
type Store interface {
	Put(id string, data []byte, meta Meta) (string, error)
	Get(id string) ([]byte, Meta, error)
	Meta(id string) (Meta, bool)
	Exists(id string) bool
}

var idSanitize = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// Disk is the local-disk implementation.
type Disk struct {
	dir string
}

// NewDisk creates the blob directory if needed and returns a Disk store.
func NewDisk(dir string) (*Disk, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Disk{dir: dir}, nil
}

// blobName turns an external blob id into one direct-child name. Root-scoped
// file operations below provide the actual traversal and symlink boundary;
// sanitizing here also preserves the store's existing deterministic filenames.
func blobName(id string) string {
	safe := idSanitize.ReplaceAllString(id, "_")
	if safe == "" || safe == "." || safe == ".." {
		safe = "_"
	}
	return safe
}

func metaName(id string) string { return blobName(id) + ".meta.json" }

// Put writes bytes + sidecar meta under a deterministic, id-derived path; a
// re-delivery of the same id overwrites the same bytes (idempotent).
func (d *Disk) Put(id string, data []byte, meta Meta) (string, error) {
	if meta.FileSize == 0 {
		meta.FileSize = int64(len(data))
	}
	root, err := os.OpenRoot(d.dir)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()
	if err := root.WriteFile(blobName(id), data, 0o644); err != nil {
		return "", err
	}
	mb, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}
	if err := root.WriteFile(metaName(id), mb, 0o644); err != nil {
		return "", err
	}
	return id, nil
}

// Get returns bytes + meta for an id.
func (d *Disk) Get(id string) ([]byte, Meta, error) {
	root, err := os.OpenRoot(d.dir)
	if err != nil {
		return nil, Meta{}, err
	}
	defer func() { _ = root.Close() }()
	data, err := root.ReadFile(blobName(id))
	if err != nil {
		return nil, Meta{}, fmt.Errorf("blob bytes %s: %w", id, err)
	}
	meta, _ := readMeta(root, id)
	return data, meta, nil
}

// Meta returns just the descriptor.
func (d *Disk) Meta(id string) (Meta, bool) {
	root, err := os.OpenRoot(d.dir)
	if err != nil {
		return Meta{}, false
	}
	defer func() { _ = root.Close() }()
	return readMeta(root, id)
}

func readMeta(root *os.Root, id string) (Meta, bool) {
	mb, err := root.ReadFile(metaName(id))
	if err != nil {
		return Meta{}, false
	}
	var m Meta
	if err := json.Unmarshal(mb, &m); err != nil {
		return Meta{}, false
	}
	return m, true
}

// Exists reports whether bytes are present for an id.
func (d *Disk) Exists(id string) bool {
	root, err := os.OpenRoot(d.dir)
	if err != nil {
		return false
	}
	defer func() { _ = root.Close() }()
	_, err = root.Stat(blobName(id))
	return err == nil
}
