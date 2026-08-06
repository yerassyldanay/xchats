// Package version is xchats' own build version — the single source both
// the update-check surface (internal/updatecheck) and, eventually, release
// tooling read from.
package version

// Version is this build's version. "0.1.0" until a real release process
// (Track 3) starts stamping it from a git tag at build time — overridable
// with:
//
//	go build -ldflags "-X github.com/yerassyldanay/xchats/backend/internal/version.Version=1.2.3"
var Version = "0.1.0"
