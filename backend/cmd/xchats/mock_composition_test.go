package main

// mock_composition_test.go proves the OTHER half of the hermetic-profiling
// harness's central claim: not just that mock-externals mode works (see
// mock_e2e_test.go), but that it never once falls through to a real
// outbound transport while doing so. It swaps the package-level
// http.DefaultTransport — the fallback every *http.Client{} without an
// explicit custom Transport silently uses, e.g. the exact
// &http.Client{Timeout: validateTimeout} shape credentials.validateHTTPClient
// had before buildServer's mock-externals branch replaces it — for a
// recording, panicking one, then drives the same full request cycle
// mock_e2e_test.go does. A single real call anywhere in that cycle is
// caught, whichever boundary it slipped through.

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordingTransport is installed as http.DefaultTransport for the
// duration of TestMockModeCompositionZeroExternalCalls. Every call is both
// recorded (so the test can report exactly what leaked, even if the panic
// below was recovered somewhere upstream — a queue/worker pool commonly
// wraps job handlers in a recover() to keep one bad job from taking down
// the whole pool) and panicked on (so a call NOT recovered anywhere fails
// the test binary immediately and loudly, with a stack trace pointing at
// the exact call site).
type recordingTransport struct {
	mu    sync.Mutex
	calls []string
}

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	call := req.Method + " " + req.URL.String()
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
	panic("mock-externals: unexpected real HTTP call via http.DefaultTransport: " + call)
}

func (r *recordingTransport) Calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
}

// TestMockModeCompositionZeroExternalCalls builds the real composition root
// (buildServer) in system.mock_externals mode with http.DefaultTransport
// swapped for one that fails the test on any use, then drives it through
// the exact same request cycle TestMockModeEndToEnd does — authentication,
// inbound ingestion, the async AI-draft worker, chat/message/KB listing,
// synchronous response generation, an outbound send per channel, a
// credential probe, a KB import, and the tunnel/update-check surfaces.
// Passing proves mock-externals mode is hermetic in fact, not merely by
// code inspection: nothing anywhere in that cycle constructed an
// *http.Client{} (or any other http.RoundTripper user) without an explicit,
// already-mocked Transport of its own.
func TestMockModeCompositionZeroExternalCalls(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := mockE2EConfig(t)
	mockE2ESeed(t, cfg, log)

	transport := &recordingTransport{}
	original := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = original })

	ctx, cancel := context.WithCancel(context.Background())
	bundle, err := buildServer(ctx, cfg, log, "")
	if err != nil {
		t.Fatalf("buildServer: %v", err)
	}
	t.Cleanup(func() {
		// See TestMockModeEndToEnd's identical cleanup for why cancel must
		// run before Teardown: automation.Scheduler (and every other
		// background loop buildServer started) only exits once ctx is Done.
		cancel()
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()
		bundle.Teardown(shutdownCtx)
	})

	srv := httptest.NewServer(bundle.Router)
	t.Cleanup(srv.Close)

	runMockModeRequestCycle(t, srv.URL)

	if calls := transport.Calls(); len(calls) > 0 {
		t.Errorf("mock-externals mode made %d real HTTP call(s) via http.DefaultTransport instead of a mock:\n%s",
			len(calls), strings.Join(calls, "\n"))
	}
}
