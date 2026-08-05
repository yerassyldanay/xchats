package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
)

// --- test doubles -----------------------------------------------------

// fakeTunnel wraps a real loopback TCP listener so http.Serve/Accept/Close
// behave exactly as they do in production (including Accept returning a
// net.ErrClosed-wrapped error once Close is called) without needing the
// real ngrok SDK.
type fakeTunnel struct {
	net.Listener
	url string
}

func (f *fakeTunnel) URL() string { return f.url }

func newFakeTunnel(t *testing.T) *fakeTunnel {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return &fakeTunnel{Listener: ln, url: "https://fake.ngrok.app"}
}

// erroringTunnel's Accept always fails with a fixed, non-close error —
// simulating http.Serve's loop exiting for a reason OTHER than a
// deliberate Stop (e.g. the ngrok session itself failing).
type erroringTunnel struct {
	err error
}

func (e *erroringTunnel) Accept() (net.Conn, error) { return nil, e.err }
func (e *erroringTunnel) Close() error              { return nil }
func (e *erroringTunnel) Addr() net.Addr            { return fakeAddr{} }
func (e *erroringTunnel) URL() string               { return "https://fake.ngrok.app" }

type fakeAddr struct{}

func (fakeAddr) Network() string { return "tcp" }
func (fakeAddr) String() string  { return "127.0.0.1:0" }

// recordingConnect is a connectFn double that records every call and
// returns a scripted tunnel/error.
type recordingConnect struct {
	mu     sync.Mutex
	calls  []connectOptions
	tunnel ngrokTunnel
	err    error
}

func (r *recordingConnect) fn(_ context.Context, opts connectOptions) (ngrokTunnel, error) {
	r.mu.Lock()
	r.calls = append(r.calls, opts)
	r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	return r.tunnel, nil
}

func (r *recordingConnect) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

type fakeTokenReader struct {
	token string
	err   error
}

func (f fakeTokenReader) Get(context.Context, credentials.Key) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}

type fakeSettingsReader struct {
	s   settings.Settings
	err error
}

func (f fakeSettingsReader) Load() (settings.Settings, error) { return f.s, f.err }

// fakeNgrokError implements the exported ngrok.Error interface (error,
// Msg() string, ErrorCode() string) — how tests simulate the ngrok SERVICE
// rejecting a connect attempt (an invalid/expired/revoked authtoken, most
// commonly) without needing the real SDK's unexported error types.
type fakeNgrokError struct {
	msg  string
	code string
}

func (e fakeNgrokError) Error() string     { return e.msg + "\n\n" + e.code }
func (e fakeNgrokError) Msg() string       { return e.msg }
func (e fakeNgrokError) ErrorCode() string { return e.code }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestManager(t *testing.T, creds tokenReader, sr settingsReader, connect connectFn) *Manager {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	m := NewManager(Deps{
		Creds: creds, Settings: sr, Handler: handler, Log: discardLogger(),
		connect: connect,
	})
	t.Cleanup(func() {
		_ = m.Stop(context.Background())
	})
	return m
}

// --- tests --------------------------------------------------------------

func TestStartSuccessSetsRunningStatus(t *testing.T) {
	tun := newFakeTunnel(t)
	rc := &recordingConnect{tunnel: tun}
	m := newTestManager(t, fakeTokenReader{token: "tok"}, nil, rc.fn)

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	st := m.Status()
	if !st.Running {
		t.Error("Running = false, want true")
	}
	if st.PublicURL != tun.url {
		t.Errorf("PublicURL = %q, want %q", st.PublicURL, tun.url)
	}
	if st.StartedAt == nil {
		t.Error("StartedAt is nil, want non-nil")
	}
	if st.LastError != "" {
		t.Errorf("LastError = %q, want empty", st.LastError)
	}
}

func TestStartServesHandlerOverTheTunnel(t *testing.T) {
	tun := newFakeTunnel(t)
	addr := tun.Addr().String()
	rc := &recordingConnect{tunnel: tun}
	m := newTestManager(t, fakeTokenReader{token: "tok"}, nil, rc.fn)

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("GET through the tunnel: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Errorf("response = (%d, %q), want (200, %q)", resp.StatusCode, body, "ok")
	}
}

func TestStartIsIdempotentWhileRunning(t *testing.T) {
	tun := newFakeTunnel(t)
	rc := &recordingConnect{tunnel: tun}
	m := newTestManager(t, fakeTokenReader{token: "tok"}, nil, rc.fn)

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start (1st): %v", err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start (2nd): %v", err)
	}
	if n := rc.callCount(); n != 1 {
		t.Errorf("connect called %d times, want 1 (Start while running must be a no-op)", n)
	}
}

func TestStartCredentialLookupFailureNeverCallsConnect(t *testing.T) {
	rc := &recordingConnect{tunnel: newFakeTunnel(t)}
	m := newTestManager(t, fakeTokenReader{err: credentials.ErrNotFound}, nil, rc.fn)

	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("Start with no stored token: want an error, got nil")
	}
	if n := rc.callCount(); n != 0 {
		t.Errorf("connect called %d times, want 0", n)
	}
	st := m.Status()
	if st.Running {
		t.Error("Running = true after a failed Start, want false")
	}
	if st.LastError == "" {
		t.Error("LastError is empty after a failed Start")
	}
}

func TestStartConnectFailureSanitizesTokenFromStatus(t *testing.T) {
	const token = "super-secret-authtoken-value"
	rc := &recordingConnect{err: fakeNgrokError{
		msg:  fmt.Sprintf("authentication failed for token %s", token),
		code: "ERR_NGROK_105",
	}}
	m := newTestManager(t, fakeTokenReader{token: token}, nil, rc.fn)

	if err := m.Start(context.Background()); err == nil {
		t.Fatal("Start with a rejected token: want an error, got nil")
	}
	st := m.Status()
	if st.Running {
		t.Error("Running = true after a failed Start, want false")
	}
	if strings.Contains(st.LastError, token) {
		t.Errorf("LastError = %q, contains the raw token — must be redacted", st.LastError)
	}
	if !strings.Contains(st.LastError, "[REDACTED]") {
		t.Errorf("LastError = %q, want the token replaced with a redaction marker", st.LastError)
	}
}

func TestClassifyConnectError(t *testing.T) {
	t.Run("ngrok.Error is classified as auth", func(t *testing.T) {
		reason, msg := classifyConnectError(fakeNgrokError{msg: "bad token", code: "ERR_NGROK_105"})
		if reason != "auth" {
			t.Errorf("reason = %q, want %q", reason, "auth")
		}
		if msg != "bad token" {
			t.Errorf("message = %q, want the ngrok.Error's own Msg(), %q", msg, "bad token")
		}
	})

	t.Run("a plain error is classified as unreachable", func(t *testing.T) {
		reason, msg := classifyConnectError(errors.New("dial tcp: connection refused"))
		if reason != "unreachable" {
			t.Errorf("reason = %q, want %q", reason, "unreachable")
		}
		if msg != "dial tcp: connection refused" {
			t.Errorf("message = %q, want the plain error text", msg)
		}
	})

	t.Run("a wrapped ngrok.Error is still found", func(t *testing.T) {
		wrapped := fmt.Errorf("connect: %w", fakeNgrokError{msg: "bad token", code: "ERR_NGROK_105"})
		reason, _ := classifyConnectError(wrapped)
		if reason != "auth" {
			t.Errorf("reason = %q, want %q (errors.As must unwrap)", reason, "auth")
		}
	})
}

func TestSanitizeMessage(t *testing.T) {
	if got := sanitizeMessage("token abc123 was rejected", "abc123"); got != "token [REDACTED] was rejected" {
		t.Errorf("sanitizeMessage = %q, want the token redacted", got)
	}
	if got := sanitizeMessage("no token here", ""); got != "no token here" {
		t.Errorf("sanitizeMessage with an empty token = %q, want the message unchanged", got)
	}
}

func TestStopClosesListenerAndResetsStatus(t *testing.T) {
	tun := newFakeTunnel(t)
	addr := tun.Addr().String()
	rc := &recordingConnect{tunnel: tun}
	m := newTestManager(t, fakeTokenReader{token: "tok"}, nil, rc.fn)

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	st := m.Status()
	if st.Running || st.PublicURL != "" || st.StartedAt != nil {
		t.Errorf("Status after Stop = %+v, want the zero value", st)
	}

	// The underlying listener is genuinely closed — a new connection attempt
	// must fail rather than reach the (now-defunct) handler.
	if _, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
		t.Error("dial succeeded against a stopped tunnel's address, want a connection failure")
	}
}

func TestStopIsIdempotentWhenNotRunning(t *testing.T) {
	m := newTestManager(t, fakeTokenReader{token: "tok"}, nil, (&recordingConnect{tunnel: newFakeTunnel(t)}).fn)
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("Stop with nothing running: %v", err)
	}
}

func TestStartAfterStopReconnectsWithAFreshToken(t *testing.T) {
	tokens := &fakeTokenReaderCounter{token: "tok"}
	m := newTestManager(t, tokens, nil, func(ctx context.Context, opts connectOptions) (ngrokTunnel, error) {
		// A fresh listener each call — the first one is closed by Stop and
		// cannot be reused.
		return &fakeTunnel{Listener: mustListen(t), url: "https://fake.ngrok.app"}, nil
	})

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start (1st): %v", err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start (2nd): %v", err)
	}
	if !m.Status().Running {
		t.Error("Running = false after restarting, want true")
	}
	if tokens.calls() < 2 {
		t.Errorf("token fetched %d times, want at least 2 (once per Start)", tokens.calls())
	}
}

type fakeTokenReaderCounter struct {
	token string
	mu    sync.Mutex
	n     int
}

func (f *fakeTokenReaderCounter) Get(context.Context, credentials.Key) (string, error) {
	f.mu.Lock()
	f.n++
	f.mu.Unlock()
	return f.token, nil
}

func (f *fakeTokenReaderCounter) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.n
}

func mustListen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return ln
}

func TestTunnelOptionsPassRegionAndDomainFromSettings(t *testing.T) {
	rc := &recordingConnect{tunnel: newFakeTunnel(t)}
	sr := fakeSettingsReader{s: settings.Settings{Ngrok: settings.NgrokSettings{Region: "eu", Domain: "custom.ngrok.app"}}}
	m := newTestManager(t, fakeTokenReader{token: "tok"}, sr, rc.fn)

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(rc.calls) != 1 {
		t.Fatalf("connect called %d times, want 1", len(rc.calls))
	}
	got := rc.calls[0]
	if got.Region != "eu" || got.Domain != "custom.ngrok.app" {
		t.Errorf("connectOptions = %+v, want Region=eu Domain=custom.ngrok.app", got)
	}
}

func TestTunnelOptionsNilSettingsIsFine(t *testing.T) {
	rc := &recordingConnect{tunnel: newFakeTunnel(t)}
	m := newTestManager(t, fakeTokenReader{token: "tok"}, nil, rc.fn)

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	got := rc.calls[0]
	if got.Region != "" || got.Domain != "" {
		t.Errorf("connectOptions = %+v, want both empty with no Settings configured", got)
	}
}

func TestNewManagerDefaultsLogAndConnectWhenUnset(t *testing.T) {
	m := NewManager(Deps{})
	if m.log == nil {
		t.Error("log should default to slog.Default() when Deps.Log is unset")
	}
	if m.deps.connect == nil {
		t.Error("connect should default to realConnect when Deps.connect is unset")
	}
}

func TestServeLoopFailureIsRecordedAsLastError(t *testing.T) {
	tun := &erroringTunnel{err: errors.New("session terminated by ngrok")}
	rc := &recordingConnect{tunnel: tun}
	m := newTestManager(t, fakeTokenReader{token: "tok"}, nil, rc.fn)

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// The serve goroutine's Accept() fails immediately; give it a moment to
	// reconcile Status (no exported "wait for the goroutine" hook — this is
	// a genuinely asynchronous transition, mirrored on the poll interval a
	// real ngrok session failure would need too).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !m.Status().Running {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	st := m.Status()
	if st.Running {
		t.Error("Running = true after the serve loop exited with an error, want false")
	}
	if st.LastError == "" {
		t.Error("LastError is empty after an unexpected serve-loop failure, want the error recorded")
	}
}
