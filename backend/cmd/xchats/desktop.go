package main

import (
	"log/slog"
	"net/http"

	"github.com/yerassyldanay/xchats/backend/internal/realtime"
)

// shellDeps is what runServe hands the desktop shell once the backend is up:
// the router the window talks to, the realtime hub its live updates come
// from, and the logger/listen address for the startup line. The server build
// ignores every field — see desktop_off.go.
//
// Declared here, untagged, so the single call site in runServe compiles
// identically in both builds and the two implementations can never drift
// apart in signature.
type shellDeps struct {
	Router http.Handler
	Hub    *realtime.Hub
	Log    *slog.Logger
	Addr   string
}
