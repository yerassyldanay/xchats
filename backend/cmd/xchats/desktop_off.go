//go:build !desktop

package main

import (
	"context"
	"net/http"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/desktop"
)

// This file is the server build: `go build ./cmd/xchats`, the Docker image,
// and everything CI runs. It carries no Wails import, so the default build
// stays exactly the binary it was before desktop packaging existed — same
// dependency graph, same CGO_ENABLED=0 cross-compile, same distroless image.
// The desktop build swaps in desktop_on.go behind the `desktop` build tag.

// resolveConfigPath is the unchanged config resolution chain: -config flag,
// then $XCHATS_CONFIG, then ./config.yaml, then the OS config directory.
func resolveConfigPath(explicit string) string { return config.ResolveConfigPath(explicit) }

// applyDesktopDefaults is a no-op here: a server deployment's paths and
// listen address come from config.yaml and the environment, exactly as
// documented.
func applyDesktopDefaults(*config.Config) error { return nil }

// wrapE2EHTTP is a no-op here: there is no embedded SPA bundle in the
// server build for XCHATS_DESKTOP_E2E_HTTP to serve (assets.go is
// desktop-tagged), so the flag has nothing to do and the listener's
// handler is returned exactly as given.
func wrapE2EHTTP(next http.Handler, _ string, _ *desktop.Readiness) http.Handler { return next }

// runUntilShutdown blocks until SIGINT/SIGTERM — the behavior runServe has
// always had at this point.
func runUntilShutdown(ctx context.Context, _ context.CancelFunc, _ shellDeps) {
	<-ctx.Done()
}
