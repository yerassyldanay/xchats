// Package lintgatesmoketest exists only to verify that backend-test's Lint
// step actually catches a new violation. Deliberate errcheck violation
// below — this package and PR are throwaway, never meant to merge.
package lintgatesmoketest

import "os"

func LeakyOpen(path string) {
	f, _ := os.Open(path)
	f.Close()
}
