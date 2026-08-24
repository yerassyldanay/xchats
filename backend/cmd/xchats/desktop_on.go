//go:build desktop

package main

import (
	"context"
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

// runUntilShutdown runs the Wails window on this goroutine (macOS needs the
// UI loop on the main thread) and returns when the user closes it or a
// signal cancels ctx. stop() then cancels ctx for the workers runServe
// started, so the teardown that follows is the same one Ctrl-C triggers on
// the server build.
func runUntilShutdown(ctx context.Context, stop context.CancelFunc, d shellDeps) {
	err := desktop.Run(ctx, desktop.Deps{Router: d.Router, Hub: d.Hub, Log: d.Log, Addr: d.Addr})
	stop()
	if err != nil && d.Log != nil {
		d.Log.Error("desktop window exited with an error", "err", err)
	}
}
