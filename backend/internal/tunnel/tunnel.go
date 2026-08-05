// Package tunnel embeds an ngrok tunnel (golang.ngrok.com/ngrok) that
// exposes the ENTIRE xchats HTTP router over a public ngrok URL — not a
// narrower MCP-only proxy in front of it.
//
// This is a deliberate deviation from an earlier implicit assumption
// (recorded in plan/DECISIONS.md's amendment, and surfaced again in the
// Settings UI itself — Track 2F/2H) that a public tunnel would expose only
// the MCP connector surface. It exposes the whole app instead: every route
// already sits behind xchats' own session auth or a signed token, and
// standing up a second, narrower proxy in front just for the tunnel leg
// would be a second surface to keep in sync with the real one — a bigger
// risk, not a smaller one, for a self-hosted single-tenant deployment.
package tunnel

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
)

// Status is a snapshot of the tunnel's current state. JSON-tagged (snake_case,
// matching every other httpapi response) since internal/httpapi/settings.go
// serializes it directly as GET /settings/tunnel's payload.
type Status struct {
	Running   bool       `json:"running"`
	PublicURL string     `json:"public_url,omitempty"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	// LastError is the most recent failure, sanitized (the ngrok authtoken
	// is never present in it even if the underlying error happened to
	// otherwise echo it back) — "" means no failure since the last
	// successful Start, or none has ever been attempted.
	LastError string `json:"last_error,omitempty"`
}

// Tunnel starts, stops, and reports on a single embedded ngrok tunnel.
type Tunnel interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Status() Status
}

// tokenReader is the exact slice of credentials.CredentialStore this
// package needs — a narrow, consumer-defined interface (matching e.g.
// internal/tgpoller.TokenSource) rather than the full read/write/delete
// CredentialStore, since Manager only ever reads the ngrok authtoken.
type tokenReader interface {
	Get(ctx context.Context, key credentials.Key) (string, error)
}

// settingsReader is the exact slice of *settings.Store this package needs.
type settingsReader interface {
	Load() (settings.Settings, error)
}

// Deps wires a Manager to its dependencies.
type Deps struct {
	// Creds resolves the ngrok authtoken at Start time — read fresh on
	// every call, never cached, so a token saved via the Settings UI after
	// a previous failed Start takes effect on the very next one.
	Creds tokenReader
	// Settings supplies the tunnel's region/reserved-domain configuration.
	// Optional: a nil Settings (or one whose Load fails) just means "no
	// region/domain override," never a Start failure.
	Settings settingsReader
	// Handler is served over the tunnel — the MAIN application router (see
	// this package's own doc comment for why that is deliberate).
	Handler http.Handler
	Log     *slog.Logger

	// connect is a test seam standing in for the real ngrok SDK connect+listen
	// sequence (realConnect, in connect.go). Production always leaves this
	// nil, which NewManager defaults to realConnect.
	connect connectFn
}

// Manager is xchats' one Tunnel implementation: an embedded ngrok session,
// lazily (re)connected on Start, serving Deps.Handler until Stop.
type Manager struct {
	deps Deps
	log  *slog.Logger

	mu       sync.Mutex
	tun      ngrokTunnel
	done     chan struct{} // closed when the current serve goroutine exits
	stopping bool          // Stop owns status while the current listener unwinds
	status   Status
}

// NewManager builds a Manager. It does not connect anything — call Start.
func NewManager(d Deps) *Manager {
	if d.connect == nil {
		d.connect = realConnect
	}
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Manager{deps: d, log: log}
}

var _ Tunnel = (*Manager)(nil)

// Status returns the tunnel's current state. Safe to call from any
// goroutine, including while Start is in progress on another one.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Manager) setStatus(s Status) {
	m.mu.Lock()
	m.status = s
	m.mu.Unlock()
}
