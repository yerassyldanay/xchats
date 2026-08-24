//go:build desktop

package desktop

import (
	"embed"
	"io/fs"
)

// bundle is the built Vue SPA, compiled into the desktop binary so a shipped
// app is a single self-contained executable with no assets to install
// alongside it.
//
// The files are a mirror of frontend/dist, copied here by
// frontend/scripts/sync-desktop-assets.mjs (run as part of `npm run
// build:desktop`, which wails.json wires to frontend:build). A copy rather
// than a second Vite output target on purpose: go:embed cannot reach outside
// its own package directory, and repointing Vite's outDir would have changed
// the web and Docker builds — which this packaging work is meant to leave
// alone. The directory is gitignored; only .gitkeep is committed, so the
// package still compiles under `-tags desktop` before a frontend build has
// ever run (the window then reports the missing bundle instead of failing to
// link).
//
// all: is required — Vite emits hashed asset filenames and the blog build
// writes nested directories, and the default embed pattern silently skips
// anything starting with "." or "_".
//
//go:embed all:dist
var bundle embed.FS

// Assets is the SPA bundle rooted at index.html, ready for both Wails'
// asset server and NewSPAHandler's history-mode fallback.
var Assets = mustSub(bundle, "dist")

func mustSub(f fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		// Only reachable if the embed directive above and this path disagree,
		// which is a compile-time-adjacent programming error, not a runtime
		// condition an installed app can hit.
		panic("desktop: embed root " + dir + ": " + err.Error())
	}
	return sub
}
