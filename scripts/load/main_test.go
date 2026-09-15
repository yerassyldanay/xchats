package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseWeights_EmptyKeepsDefaults(t *testing.T) {
	got, err := parseWeights("")
	if err != nil {
		t.Fatalf("parseWeights: %v", err)
	}
	if len(got) != len(defaultWeights) {
		t.Fatalf("got %d weights, want %d (the defaults)", len(got), len(defaultWeights))
	}
	for k, v := range defaultWeights {
		if got[k] != v {
			t.Errorf("weight[%q] = %v, want default %v", k, got[k], v)
		}
	}
}

func TestParseWeights_OverridesNamedRoutesOnly(t *testing.T) {
	got, err := parseWeights("kb=50, list_chats=10")
	if err != nil {
		t.Fatalf("parseWeights: %v", err)
	}
	if got[routeKB] != 50 {
		t.Errorf("kb weight = %v, want 50", got[routeKB])
	}
	if got[routeListChats] != 10 {
		t.Errorf("list_chats weight = %v, want 10", got[routeListChats])
	}
	// Untouched routes keep their default.
	if got[routeSendMessage] != defaultWeights[routeSendMessage] {
		t.Errorf("send_message weight = %v, want untouched default %v", got[routeSendMessage], defaultWeights[routeSendMessage])
	}
}

func TestParseWeights_RejectsUnknownRoute(t *testing.T) {
	if _, err := parseWeights("not_a_real_route=10"); err == nil {
		t.Fatal("expected an error for an unknown route name")
	}
}

func TestParseWeights_RejectsMalformedEntry(t *testing.T) {
	if _, err := parseWeights("kb"); err == nil {
		t.Fatal("expected an error for an entry with no '='")
	}
	if _, err := parseWeights("kb=not-a-number"); err == nil {
		t.Fatal("expected an error for a non-numeric weight")
	}
}

func TestWaitForReadyFile_ReturnsOnceFileAppears(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ready")
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(path, []byte("ok"), 0o644)
	}()
	start := time.Now()
	if err := waitForReadyFile(context.Background(), path, 5*time.Second); err != nil {
		t.Fatalf("waitForReadyFile: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("waitForReadyFile took %v, want it to return shortly after the file appeared", elapsed)
	}
}

func TestWaitForReadyFile_TimesOut(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "never-created")
	err := waitForReadyFile(context.Background(), path, 150*time.Millisecond)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
}

func TestWaitForReadyFile_RespectsContextCancellation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "never-created")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := waitForReadyFile(ctx, path, 30*time.Second)
	if err == nil {
		t.Fatal("expected an error from context cancellation")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("waitForReadyFile took %v to notice cancellation", elapsed)
	}
}
