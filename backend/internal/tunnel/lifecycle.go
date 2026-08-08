package tunnel

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/credentials"
)

// Start resolves the ngrok authtoken (fresh from the credential store —
// never cached) and region/reserved-domain (from Settings, if any), opens
// an ngrok session and HTTP tunnel, and begins serving Deps.Handler over
// it. Idempotent: calling Start while already running is a no-op that
// returns nil without reconnecting.
//
// An authtoken lookup failure or a connect failure is recorded in Status
// (see classifyConnectError/sanitizeMessage) and returned to the caller —
// neither ever mutates the stored credential itself ("doesn't condemn the
// token" — a transient network failure must not read as "your token is
// bad," and this package draws no such conclusion either way; that
// judgment belongs to whoever reads Status and decides what to tell the
// operator).
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.tun != nil {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	token, err := m.deps.Creds.Get(ctx, credentials.KeyNgrokAuthtoken)
	if err != nil {
		m.setStatus(Status{LastError: sanitizeMessage(err.Error(), "")})
		return err
	}

	region, domain := m.tunnelOptions()
	// The ngrok SDK keeps the context passed to Connect for the entire
	// session lifetime. Start is called from an HTTP handler, whose request
	// context is cancelled as soon as the response is written, so handing
	// that context directly to ngrok tears the new listener down
	// immediately. Preserve request-scoped values while giving the session
	// its own cancellation lifetime; the small watcher still lets a client
	// cancellation abort the connection attempt, then exits before Start
	// returns so it cannot leak into the running session.
	sessionCtx, cancelSession := context.WithCancel(context.WithoutCancel(ctx))
	setupDone := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			cancelSession()
		case <-setupDone:
		}
	}()

	tun, err := m.deps.connect(sessionCtx, connectOptions{Authtoken: token, Region: region, Domain: domain})
	close(setupDone)
	<-watcherDone
	if err == nil {
		err = sessionCtx.Err()
		if err != nil {
			_ = tun.Close()
		}
	}
	if err != nil {
		cancelSession()
		reason, msg := classifyConnectError(err)
		m.log.Warn("ngrok tunnel failed to start", "reason", reason)
		m.setStatus(Status{LastError: sanitizeMessage(msg, token)})
		return err
	}

	done := make(chan struct{})
	now := time.Now().UTC()

	m.mu.Lock()
	if m.tun != nil {
		// Lost a race against a concurrent Start that got here first —
		// close our own redundant tunnel rather than leaking it or
		// clobbering the winner's state.
		m.mu.Unlock()
		_ = tun.Close()
		cancelSession()
		return nil
	}
	m.tun = tun
	m.done = done
	m.stopping = false
	m.status = Status{Running: true, PublicURL: tun.URL(), StartedAt: &now}
	status := m.status
	m.mu.Unlock()
	m.notify(status)

	go m.serve(tun, done, token, cancelSession)

	return nil
}

// serve runs the tunnel's HTTP server until tun is closed (by Stop, or by
// the ngrok session itself failing) and then reconciles Status.
func (m *Manager) serve(tun ngrokTunnel, done chan struct{}, token string, cancelSession context.CancelFunc) {
	defer func() {
		cancelSession()
		close(done)
	}()
	err := http.Serve(tun, m.deps.Handler)

	m.mu.Lock()
	if m.tun != tun {
		// Stop() already replaced/cleared this tunnel; its own call owns
		// the resulting status, not this now-stale goroutine.
		m.mu.Unlock()
		return
	}
	if m.stopping {
		// Stop deliberately closed the listener and owns the final zero
		// status. The ngrok SDK does not wrap its "Listener closed" Accept
		// error with net.ErrClosed, so checking the error alone would report
		// every normal stop as an unexpected session failure.
		m.mu.Unlock()
		return
	}
	m.tun = nil
	if err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, http.ErrServerClosed) {
		m.status = Status{LastError: sanitizeMessage(err.Error(), token)}
		m.log.Warn("ngrok tunnel serve loop exited unexpectedly", "err", err)
		status := m.status
		m.mu.Unlock()
		m.notify(status)
		return
	}
	m.status = Status{}
	status := m.status
	m.mu.Unlock()
	m.notify(status)
}

// Stop closes the tunnel and waits for its serve loop to fully exit before
// returning, so a subsequent Start can never race a not-yet-unwound
// previous one. Idempotent: calling Stop when nothing is running is a
// no-op that returns nil.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	tun := m.tun
	done := m.done
	if tun != nil {
		m.stopping = true
	}
	m.mu.Unlock()
	if tun == nil {
		return nil
	}

	closeErr := tun.Close()
	if done != nil {
		<-done
	}

	m.mu.Lock()
	if m.tun == tun {
		m.tun = nil
	}
	m.stopping = false
	m.status = Status{}
	status := m.status
	m.mu.Unlock()
	m.notify(status)

	return closeErr
}

// tunnelOptions reads the region/reserved-domain to connect with from
// Settings — "" for either (letting the ngrok SDK pick its own defaults)
// when Settings is nil or its Load fails, since neither is ever required
// for a tunnel to start.
func (m *Manager) tunnelOptions() (region, domain string) {
	if m.deps.Settings == nil {
		return "", ""
	}
	s, err := m.deps.Settings.Load()
	if err != nil {
		return "", ""
	}
	return s.Ngrok.Region, s.Ngrok.Domain
}
