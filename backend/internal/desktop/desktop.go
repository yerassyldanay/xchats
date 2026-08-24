// Package desktop is the Wails shell that packages the existing xchats
// backend and Vue frontend as a native desktop application.
//
// It is a shell, not a second application: the desktop binary is the same
// cmd/xchats binary, booted through the same runServe path, with the same
// router, store, queue and workers. The only difference is what the process
// does once the backend is up — the default build blocks on SIGINT/SIGTERM,
// the `desktop` build opens a WebView window instead (see shell.go, built
// only under the `desktop` build tag).
//
// Three things need translating between "served over HTTP to a browser" and
// "served to an embedded WebView", and this package owns exactly those:
//
//  1. Addressing (desktop.go). A packaged app's working directory is
//     whatever the OS launcher happened to pick, so config.yaml's
//     process-relative storage paths ("./data/xchats.db") are meaningless.
//     ApplyDefaults rebases them onto the OS per-user application data
//     directory and pins the HTTP listener to loopback.
//
//  2. Transport (handler.go). The WebView loads the app from Wails' own
//     asset server, so every API call is same-origin only if the asset
//     server answers it. NewMiddleware hands the backend's own router every
//     request under an API prefix and lets the SPA bundle serve the rest.
//
//  3. Realtime (realtime.go). Wails' asset server cannot stream a response
//     on Windows (the WebView2 response writer buffers until the handler
//     returns), so SSE cannot survive the trip. PumpRealtime forwards the
//     same realtime.Hub events over Wails' own event channel instead; the
//     frontend picks the transport at runtime (frontend/src/lib/sse.ts).
//
// Everything except shell.go is plain net/http and standard library, with no
// Wails import and no build tag, so `go build ./...`, `go test ./...` and
// golangci-lint cover it in the ordinary server build.
package desktop

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/yerassyldanay/xchats/backend/internal/appdirs"
	"github.com/yerassyldanay/xchats/backend/internal/config"
)

// AppName is the directory name every OS-appropriate path is built under —
// %APPDATA%\xchats, ~/Library/Application Support/xchats,
// ~/.local/share/xchats. It matches the name cmd/xchats already passes to
// appdirs for the credential store and settings.json, so a desktop install
// and a `xchats serve` run off the same machine share one profile rather
// than quietly keeping two.
const AppName = "xchats"

// Default storage locations, relative to the application data directory.
// These are config.yaml's own committed defaults with the leading "./"
// dropped — a desktop install with no config.yaml at all lands on the same
// layout a repo checkout does, just rooted somewhere the user can find.
const (
	defaultDBPath         = "data/xchats.db"
	defaultWADeviceDBPath = "data/whatsmeow.db"
	defaultBlobDir        = "blobdata"
	defaultHTTPPort       = "8080"
)

// ConfigPath returns the config.yaml a desktop launch reads.
//
// Deliberately NOT config.ResolveConfigPath: that chain probes ./config.yaml
// first, and a packaged app's working directory is not its install directory
// — it is whatever Finder, Explorer or the .desktop entry happened to hand
// the process. Picking up a stray config.yaml from there would be silent and
// unreproducible. So the desktop shell reads exactly two locations: an
// explicit $XCHATS_CONFIG, then the OS per-user config directory.
//
// An empty return is not an error: config.Load treats a missing file as
// "use the built-in defaults", which is the zero-setup first-launch path.
func ConfigPath() string {
	if v := strings.TrimSpace(os.Getenv("XCHATS_CONFIG")); v != "" {
		return v
	}
	dir, err := appdirs.ConfigDir(AppName)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "config.yaml")
}

// ApplyDefaults rewrites cfg in place for a desktop launch: every
// process-relative storage path is resolved against the OS application data
// directory, and the HTTP listener is pinned to loopback on a port that is
// actually free.
//
// Absolute paths are left exactly as configured — an operator who pinned
// storage.db_path (or exported DB_PATH) meant that path, and $XCHATS_DATA_DIR
// still redirects the whole profile, so this adds a sane default without
// taking any existing override away.
func ApplyDefaults(cfg *config.Config) error {
	dataDir, err := appdirs.DataDir(AppName)
	if err != nil {
		return fmt.Errorf("resolve application data directory: %w", err)
	}
	if err := appdirs.EnsureDir(dataDir); err != nil {
		return fmt.Errorf("create %s: %w", dataDir, err)
	}

	cfg.Storage.DBPath = rebase(dataDir, cfg.Storage.DBPath, defaultDBPath)
	cfg.Storage.WADeviceDBPath = rebase(dataDir, cfg.Storage.WADeviceDBPath, defaultWADeviceDBPath)
	cfg.Storage.BlobDir = rebase(dataDir, cfg.Storage.BlobDir, defaultBlobDir)
	cfg.Server.HTTPAddr = freeAddr(loopbackAddr(cfg.Server.HTTPAddr))
	cfg.Server.APIBaseURL = localAPIBaseURL(cfg.Server.APIBaseURL, cfg.Server.HTTPAddr)
	return nil
}

// DataDir returns the resolved application data directory, for logging and
// for the "where does my data live" line in the docs.
func DataDir() (string, error) { return appdirs.DataDir(AppName) }

// rebase resolves p against base unless it is already absolute. An empty p
// falls back to def first, so a config.yaml that omits a storage key gets the
// same treatment as one that spells out the default.
func rebase(base, p, def string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		p = def
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(base, filepath.Clean(p))
}

// loopbackAddr forces a listen address onto 127.0.0.1.
//
// config.yaml's committed default is ":8080" — every interface — which is
// right for a container behind a reverse proxy and wrong for a desktop app:
// it would publish one user's inbox, credentials-backed settings and MCP
// endpoints to their whole network, and on Windows it is what triggers the
// firewall prompt on first launch. The desktop window reaches the backend
// through Wails' in-process asset server (see handler.go), so nothing outside
// this machine ever needs to connect.
//
// An address that isn't host:port at all is returned untouched — runServe's
// own ListenAndServe should be the one to report it, with the operator's
// original string in the error.
func loopbackAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return net.JoinHostPort("127.0.0.1", defaultHTTPPort)
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

// freeAddr returns addr if it can be bound right now, and the same host on
// an OS-assigned port if it cannot.
//
// A desktop app must not refuse to start because something else — most
// likely a `make dev-backend` or a docker-compose stack the same developer
// left running — already holds :8080. The configured port is still preferred
// (it keeps ResolvedAPIBaseURL, and therefore any MCP client registration,
// stable across launches); the fallback only trades that stability for
// actually booting.
//
// The probe-then-release window is a race in principle. In practice the only
// racer would be another process grabbing the port in the microseconds before
// runServe binds it, and the cost is the same startup failure we would have
// had anyway.
func freeAddr(addr string) string {
	ln, err := net.Listen("tcp", addr)
	if err == nil {
		_ = ln.Close()
		return addr
	}
	host, _, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		return addr
	}
	fallback, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return addr
	}
	defer func() { _ = fallback.Close() }()
	_, port, err := net.SplitHostPort(fallback.Addr().String())
	if err != nil {
		return addr
	}
	return net.JoinHostPort(host, port)
}

// localAPIBaseURL keeps api_base_url describing the port the backend
// actually bound.
//
// It exists because of freeAddr: when the configured port is already taken
// the app moves to another one, and every internally-constructed link — MCP
// discovery documents, the OAuth issuer, signed media URLs — is built from
// Config.ResolvedAPIBaseURL, which prefers this configured value over the
// listen address. Leaving it behind would hand out URLs for a port nothing
// is listening on.
//
// Only a loopback value is rewritten. A configured public origin (or the
// ngrok domain applyNgrokPublicOrigin installs at boot) is a deliberate
// statement about how the outside world reaches this install, not a
// description of the local listener, so it is left alone.
func localAPIBaseURL(configured, addr string) string {
	if configured != "" && !isLoopbackURL(configured) {
		return configured
	}
	host, port, err := net.SplitHostPort(addr)
	// Port 0 means "whatever the OS hands out at Listen time", which is not
	// knowable here — better a stale-but-well-formed URL than one naming
	// port 0.
	if err != nil || host == "" || port == "" || port == "0" {
		return configured
	}
	return "http://" + addr
}

// isLoopbackURL reports whether raw is empty, unparseable, or names a
// loopback host. It deliberately mirrors cmd/xchats' own isLocalBaseURL (the
// production-config gate) rather than sharing it: that one lives in package
// main and is a release-gate policy, while this one only decides whether a
// value is a local-listener description worth refreshing.
func isLoopbackURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return true
	}
	switch u.Hostname() {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
