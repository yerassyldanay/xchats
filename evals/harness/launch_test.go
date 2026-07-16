package main

import "testing"

func TestResolveExpectedExtractCalls(t *testing.T) {
	tests := []struct {
		name                              string
		numCases, numModels, numPrompts   int
		want                              int
	}{
		{"one case, one model, default prompt", 1, 1, 1, 1},
		{"5 cases x 3 models x 2 prompts (v1/v2 comparison)", 5, 3, 2, 30},
		{"zero cases", 0, 3, 2, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveExpectedExtractCalls(tt.numCases, tt.numModels, tt.numPrompts); got != tt.want {
				t.Errorf("resolveExpectedExtractCalls(%d,%d,%d) = %d, want %d", tt.numCases, tt.numModels, tt.numPrompts, got, tt.want)
			}
		})
	}
}

// TestCmdLaunch_RequiresAllInV1 guards the documented v1 scope (review amendment 4's
// launch manifest doesn't help if the command silently ran a partial, undeclared
// scope) — must fail BEFORE any pre-flight counting or manifest write, not partway
// through.
func TestCmdLaunch_RequiresAllInV1(t *testing.T) {
	if err := cmdLaunch(nil); err == nil {
		t.Fatal("want cmdLaunch to reject running with neither -all nor any scope flag")
	}
}
