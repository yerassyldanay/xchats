package telegrammock

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/telegram"
)

// TestGetUpdates_WaitsForRequestedTimeoutRatherThanSpinning is this
// package's whole reason to exist: telegram.Fake's own default returns
// instantly, which would spin tgpoller's poll loop at 100% CPU.
func TestGetUpdates_WaitsForRequestedTimeoutRatherThanSpinning(t *testing.T) {
	f := New(1, "mockbot")
	start := time.Now()
	updates, err := f.GetUpdates(context.Background(), "token", telegram.GetUpdatesRequest{TimeoutSeconds: 1})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if len(updates) != 0 {
		t.Fatalf("expected no updates, got %d", len(updates))
	}
	if elapsed < 900*time.Millisecond {
		t.Fatalf("GetUpdates returned after %v, want it to actually wait out the ~1s requested timeout (not spin)", elapsed)
	}
}

// TestGetUpdates_ReturnsAsSoonAsContextIsCanceled proves cancellation is
// still fast even with a long requested timeout — a poller shutdown must
// not have to wait out the full ~50s default.
func TestGetUpdates_ReturnsAsSoonAsContextIsCanceled(t *testing.T) {
	f := New(1, "mockbot")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err := f.GetUpdates(ctx, "token", telegram.GetUpdatesRequest{TimeoutSeconds: 50})
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetUpdates error = %v, want context.Canceled", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("GetUpdates took %v to notice cancellation, want it to return almost immediately", elapsed)
	}
}

// TestGetUpdates_ZeroTimeoutStillWaits guards against a busy loop when a
// caller passes TimeoutSeconds<=0 (a malformed request should still never
// spin; New() falls back to a 1s wait in that case).
func TestGetUpdates_ZeroTimeoutStillWaits(t *testing.T) {
	f := New(1, "mockbot")
	start := time.Now()
	if _, err := f.GetUpdates(context.Background(), "token", telegram.GetUpdatesRequest{}); err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Fatal("a zero requested timeout must still wait, not spin")
	}
}

func TestNewSatisfiesTelegramClient(t *testing.T) {
	var _ telegram.Client = New(1, "mockbot")
}
