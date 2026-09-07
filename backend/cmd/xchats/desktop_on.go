//go:build desktop

package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/desktop"
)

// This file is the desktop build (`wails build`, which passes -tags desktop
// via wails.json's build:tags). It is the only difference between the
// packaged desktop app and the server binary: same main, same subcommands,
// same runServe boot sequence — plus a window.

// resolveConfigPath narrows the resolution chain for a packaged app: an
// explicit -config flag, then desktop.ConfigPath's $XCHATS_CONFIG-or-OS-
// config-directory pair. The ./config.yaml probe is deliberately dropped —
// see desktop.ConfigPath.
func resolveConfigPath(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return desktop.ConfigPath()
}

// applyDesktopDefaults rebases process-relative storage paths onto the OS
// application data directory and pins the listener to loopback.
func applyDesktopDefaults(cfg *config.Config) error { return desktop.ApplyDefaults(cfg) }

// wrapE2EHTTP mounts XCHATS_DESKTOP_E2E_HTTP=1's SPA-over-loopback surface
// onto the same handler runServe already binds the listener to — a no-op
// (returns next unchanged) unless the env var is set and addr is loopback;
// see desktop.ServeE2EHTTP's own doc comment. api and next are the same
// router value: there is nothing "next" beyond the plain API router at this
// call site, unlike the Wails asset server's next (its own default 404).
func wrapE2EHTTP(next http.Handler, addr string, ready *desktop.Readiness) http.Handler {
	return desktop.ServeE2EHTTP(next, next, desktop.Assets, addr, ready)
}

// runUntilShutdown runs the Wails window on this goroutine (macOS needs the
// UI loop on the main thread) and returns when the user closes it or a
// signal cancels ctx. stop() then cancels ctx for the workers runServe
// started, so the teardown that follows is the same one Ctrl-C triggers on
// the server build.
func runUntilShutdown(ctx context.Context, stop context.CancelFunc, d shellDeps) {
	err := desktop.Run(ctx, desktop.Deps{Router: d.Router, Hub: d.Hub, Log: d.Log, Addr: d.Addr, Ready: d.Ready})
	stop()
	if err != nil && d.Log != nil {
		d.Log.Error("desktop window exited with an error", "err", err)
	}
}
