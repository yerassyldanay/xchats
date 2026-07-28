package aiprompt

import (
	"fmt"
	"regexp"
)

// storageLocatorPattern is copied exactly from the eval harness's
// storageLocatorPattern (evals/harness/render_schema_kb.go:110): any
// "scheme://..."-shaped span has no legitimate reason to appear in a rendered
// prompt at all — fact values are token-hidden, so a real KB should never put a
// URL or storage locator in front of the model. No http/https carve-out.
var storageLocatorPattern = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://\S+`)

// ValidateNoStorageLocatorLeak is the harness's validateNoStorageLocatorLeak
// (evals/harness/render_schema_kb.go:112-117), copied as-is: a hard failure on
// any URL- or storage-key-shaped substring anywhere in a rendered prompt.
func ValidateNoStorageLocatorLeak(prompt string) error {
	if m := storageLocatorPattern.FindString(prompt); m != "" {
		return fmt.Errorf("aiprompt: prompt leaks a storage-key/URL-shaped string: %s", m)
	}
	return nil
}
