package response_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestPackageDependencies guards the architecture boundary this whole
// milestone depends on: backend/response may depend on aiprompt, llm, and
// messaging (contracts) plus the standard library — never a channel provider,
// PostgreSQL, an HTTP framework, or any other backend/internal package.
// Model/provider switching must stay configuration-only, which only holds if
// this package never imports a concrete provider or channel adapter.
func TestPackageDependencies(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "./...").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}

	allowedInternal := map[string]bool{
		"github.com/yerassyldanay/xchats/backend/response":  true,
		"github.com/yerassyldanay/xchats/backend/aiprompt":  true,
		"github.com/yerassyldanay/xchats/backend/llm":       true,
		"github.com/yerassyldanay/xchats/backend/messaging": true,
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "github.com/yerassyldanay/xchats/") {
			continue // standard library / third-party: not what this boundary is about
		}
		if !allowedInternal[line] {
			t.Errorf("backend/response has a forbidden dependency on %s", line)
		}
	}
}
