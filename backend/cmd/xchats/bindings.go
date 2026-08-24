//go:build bindings

package main

import (
	"fmt"
	"os"
)

// This file exists only for Wails' binding generator, and only to stop it.
//
// `wails build`/`wails dev` compile the module a second time with
// `-tags bindings` and then RUN the resulting binary to emit the JS/TS
// wrappers for whatever was passed to options.App.Bind. Two things make that
// a problem here:
//
//   - The desktop shell binds nothing. The frontend reaches the backend over
//     ordinary same-origin HTTP (internal/desktop/handler.go), the same calls
//     the browser build makes, so there is no Go method surface to wrap.
//   - The generator strips the `desktop` tag before compiling
//     (pkg/commands/bindings), so the binary it runs is the plain server. Its
//     main() would boot the whole application — open the user's real SQLite
//     database, take its file lock, start the workers — and then block in
//     `xchats serve` forever, hanging the build.
//
// init() runs before main(), so exiting here happens before any of that. The
// generator only reads this binary's stdout for verbose logging (the wrapper
// files are written by the binary itself, which is exactly what is being
// skipped), so a clean exit leaves nothing broken.
//
// Passing -skipbindings, as the Makefile and the desktop workflow do, avoids
// the generator entirely; this is the safety net for a bare `wails build`.
func init() {
	fmt.Println("{}")
	os.Exit(0)
}
