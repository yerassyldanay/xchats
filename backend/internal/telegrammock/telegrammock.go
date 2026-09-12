// Package telegrammock adapts backend/internal/telegram.Fake for
// cmd/xchats' mock-externals mode.
package telegrammock

import (
	"context"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/telegram"
)

// New returns a telegram.Fake configured so GetUpdates never spins
// tgpoller.Manager's long-poll loop. telegram.Fake's own default (see its
// doc comment) is a test convenience: once PendingUpdates is drained,
// GetUpdates returns an empty, non-error batch IMMEDIATELY. tgpoller's poll
// loop (internal/tgpoller/tgpoller.go's run) only backs off on a GetUpdates
// ERROR — a real Telegram server throttles the very same loop for free by
// blocking server-side for the requested long-poll timeout (~50s by
// default) before answering empty, so an instant-return fake would instead
// busy-loop at 100% CPU forever. GetUpdatesFn (a first-class test seam on
// Fake) reproduces that same blocking behavior here: it waits for either
// the poller's own requested timeout or ctx cancellation, then answers with
// no updates — satisfying "waits for cancellation or its requested timeout
// rather than spinning" with no pending updates ever queued, which is the
// steady state this hermetic profiling harness runs in (inbound Telegram
// traffic is out of scope for the load mix; WhatsApp's debug/wa-event route
// covers inbound-message profiling instead).
func New(botID int64, username string) *telegram.Fake {
	f := telegram.NewFake(botID, username)
	f.GetUpdatesFn = func(ctx context.Context, token string, req telegram.GetUpdatesRequest) ([]telegram.RawUpdate, error) {
		timeout := time.Duration(req.TimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = time.Second
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(timeout):
			return nil, nil
		}
	}
	return f
}
