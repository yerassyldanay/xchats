// Command genkbfixture (re)writes the committed shop-kb-v1 Russian fixture
// from internal/kbfixture's deterministic generator. Run from evals/harness:
//
//	go run ./cmd/genkbfixture
//
// Pass an explicit path as the first argument to write somewhere else (used
// by generate_test.go's drift check to render into a temp file for
// comparison instead of overwriting the committed one).
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"xchats-evals-harness/internal/kbfixture"
)

const defaultOut = "../scenarios/shop-kb-v1/data-ru.yaml"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "genkbfixture:", err)
		os.Exit(1)
	}
}

func run() error {
	dest := defaultOut
	if len(os.Args) > 1 {
		dest = os.Args[1]
	}
	out, err := kbfixture.GenerateYAML()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
	}
	if err := os.WriteFile(dest, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	fmt.Println("wrote", dest)
	return nil
}
