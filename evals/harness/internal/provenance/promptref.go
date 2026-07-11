package provenance

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// PromptRef identifies exactly one versioned prompt file: prompts/<name>/v<version>.txt.
// Recorded on every extractRunResult so a result can never be read without knowing
// which prompt produced it — the gap that made the old Go-const prompt unversionable.
type PromptRef struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
	SHA256  string `json:"sha256"`
}

func (r PromptRef) String() string {
	return fmt.Sprintf("%s@v%d", r.Name, r.Version)
}

var promptSpecRE = regexp.MustCompile(`^([a-zA-Z0-9_-]+)@v?([0-9]+)$`)

// ParsePromptSpec parses the `-prompt` flag's "<name>@v<N>" syntax (e.g. "extract@v1";
// "extract@1" also accepted). Rejects anything else outright — a malformed spec must
// fail loudly, not silently resolve to some default version.
func ParsePromptSpec(spec string) (name string, version int, err error) {
	m := promptSpecRE.FindStringSubmatch(spec)
	if m == nil {
		return "", 0, fmt.Errorf("invalid prompt spec %q, want <name>@v<N> (e.g. extract@v1)", spec)
	}
	version, err = strconv.Atoi(m[2])
	if err != nil {
		return "", 0, fmt.Errorf("invalid prompt spec %q: %w", spec, err)
	}
	return m[1], version, nil
}

// LoadPrompt reads promptsDir/<name>/v<version>.txt and returns its content plus the
// PromptRef recording exactly what was loaded (name, version, content hash). A missing
// file is a hard error — fail-closed: an eval must never silently run with no prompt,
// or worse, an old cached one from a partial refactor.
func LoadPrompt(promptsDir, name string, version int) (content string, ref PromptRef, err error) {
	path := filepath.Join(promptsDir, name, fmt.Sprintf("v%d.txt", version))
	b, err := os.ReadFile(path)
	if err != nil {
		return "", PromptRef{}, fmt.Errorf("load prompt %s@v%d: %w", name, version, err)
	}
	return string(b), PromptRef{Name: name, Version: version, SHA256: SHA256Bytes(b)}, nil
}

// LoadedPrompt pairs a prompt's content with the ref that identifies it.
type LoadedPrompt struct {
	Ref     PromptRef
	Content string
}

// LoadPromptSpecs parses and loads every "<name>@v<N>" spec in specs, in order. A
// missing or malformed spec fails the whole call — fail-closed, matching LoadPrompt.
func LoadPromptSpecs(promptsDir string, specs []string) ([]LoadedPrompt, error) {
	out := make([]LoadedPrompt, 0, len(specs))
	for _, spec := range specs {
		name, version, err := ParsePromptSpec(spec)
		if err != nil {
			return nil, err
		}
		content, ref, err := LoadPrompt(promptsDir, name, version)
		if err != nil {
			return nil, err
		}
		out = append(out, LoadedPrompt{Ref: ref, Content: content})
	}
	return out, nil
}
