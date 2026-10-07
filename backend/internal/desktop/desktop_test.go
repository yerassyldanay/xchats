package desktop

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
)

func loadDefaults(t *testing.T) *config.Config {
	t.Helper()
	// "" => no file read, built-in defaults plus process env. The env
	// overrides that could reach these fields are cleared per-test below.
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func clearStorageEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"DATABASE_TARGET", "WA_DEVICE_DATABASE_TARGET", "BLOB_DIR", "HTTP_ADDR", "API_BASE_URL"} {
		t.Setenv(k, "")
	}
}

func TestApplyDefaultsRebasesRelativeStorageOntoTheDataDir(t *testing.T) {
	clearStorageEnv(t)
	dataDir := t.TempDir()
	t.Setenv("XCHATS_DATA_DIR", dataDir)

	cfg := loadDefaults(t)
	cfg.Server.HTTPAddr = ":0"
	if err := ApplyDefaults(cfg); err != nil {
		t.Fatalf("ApplyDefaults: %v", err)
	}

	// config.yaml's committed defaults are "./data/xchats.db",
	// "./data/whatsmeow.db" and "./blobdata" — process-relative, and so
	// meaningless once the launcher picks the working directory.
	if want := filepath.Join(dataDir, "data", "xchats.db"); cfg.Storage.DatabaseTarget != want {
		t.Errorf("DatabaseTarget = %q, want %q", cfg.Storage.DatabaseTarget, want)
	}
	if want := filepath.Join(dataDir, "data", "whatsmeow.db"); cfg.Storage.WADeviceDatabaseTarget != want {
		t.Errorf("WADeviceDatabaseTarget = %q, want %q", cfg.Storage.WADeviceDatabaseTarget, want)
	}
	if want := filepath.Join(dataDir, "blobdata"); cfg.Storage.BlobDir != want {
		t.Errorf("BlobDir = %q, want %q", cfg.Storage.BlobDir, want)
	}
	if want := "127.0.0.1:0"; cfg.Server.HTTPAddr != want {
		t.Errorf("HTTPAddr = %q, want %q", cfg.Server.HTTPAddr, want)
	}
}

func TestApplyDefaultsLeavesAbsolutePathsAlone(t *testing.T) {
	clearStorageEnv(t)
	t.Setenv("XCHATS_DATA_DIR", t.TempDir())

	pinned := filepath.Join(t.TempDir(), "pinned", "xchats.db")
	cfg := loadDefaults(t)
	cfg.Storage.DatabaseTarget = pinned
	cfg.Server.HTTPAddr = ":0"
	if err := ApplyDefaults(cfg); err != nil {
		t.Fatalf("ApplyDefaults: %v", err)
	}
	if cfg.Storage.DatabaseTarget != pinned {
		t.Errorf("DatabaseTarget = %q, want the configured absolute path %q untouched", cfg.Storage.DatabaseTarget, pinned)
	}
}

// A PostgreSQL URL is not a path: joining it onto the data directory would turn the database
// into a directory name and the desktop app would silently open a file there.
func TestApplyDefaultsLeavesDatabaseURLAlone(t *testing.T) {
	clearStorageEnv(t)
	t.Setenv("XCHATS_DATA_DIR", t.TempDir())

	const url = "postgres://app:pw@db.example:5432/xchats"
	cfg := loadDefaults(t)
	cfg.Storage.DatabaseTarget = url
	cfg.Server.HTTPAddr = ":0"
	if err := ApplyDefaults(cfg); err != nil {
		t.Fatalf("ApplyDefaults: %v", err)
	}
	if cfg.Storage.DatabaseTarget != url {
		t.Errorf("DatabaseTarget = %q, want the URL %q untouched", cfg.Storage.DatabaseTarget, url)
	}
}

func TestApplyDefaultsCreatesTheDataDirectory(t *testing.T) {
	clearStorageEnv(t)
	dataDir := filepath.Join(t.TempDir(), "not", "created", "yet")
	t.Setenv("XCHATS_DATA_DIR", dataDir)

	cfg := loadDefaults(t)
	cfg.Server.HTTPAddr = ":0"
	if err := ApplyDefaults(cfg); err != nil {
		t.Fatalf("ApplyDefaults: %v", err)
	}
	// EnsureDir is what makes the first launch work on a machine that has
	// never run xchats: dbx.Open creates the DB's own parent, but the
	// credential store and settings.json expect the profile root to exist.
	fi, err := os.Stat(dataDir)
	if err != nil {
		t.Fatalf("data dir %q was not created: %v", dataDir, err)
	}
	if !fi.IsDir() {
		t.Errorf("%q exists but is not a directory", dataDir)
	}
}

func TestConfigPathPrefersTheExplicitEnvOverride(t *testing.T) {
	t.Setenv("XCHATS_CONFIG", "/somewhere/else/config.yaml")
	if got, want := ConfigPath(), "/somewhere/else/config.yaml"; got != want {
		t.Errorf("ConfigPath = %q, want %q", got, want)
	}
}

func TestConfigPathFallsBackToTheOSConfigDirectory(t *testing.T) {
	t.Setenv("XCHATS_CONFIG", "")
	dir := t.TempDir()
	t.Setenv("XCHATS_CONFIG_DIR", dir)
	if got, want := ConfigPath(), filepath.Join(dir, "config.yaml"); got != want {
		t.Errorf("ConfigPath = %q, want %q", got, want)
	}
}

func TestLoopbackAddr(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "127.0.0.1:8080"},
		{":8080", "127.0.0.1:8080"}, // config.yaml's committed default
		{"0.0.0.0:8080", "127.0.0.1:8080"},
		{"[::]:8080", "127.0.0.1:8080"},
		{"127.0.0.1:9999", "127.0.0.1:9999"},       // already loopback: untouched
		{"192.168.1.10:8080", "192.168.1.10:8080"}, // a deliberate LAN bind stays
		{"not-an-address", "not-an-address"},       // let ListenAndServe report it
	}
	for _, c := range cases {
		if got := loopbackAddr(c.in); got != c.want {
			t.Errorf("loopbackAddr(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFreeAddrKeepsAnAvailablePort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if got := freeAddr(addr); got != addr {
		t.Errorf("freeAddr(%q) = %q, want the configured address kept when it is bindable", addr, got)
	}
}

func TestFreeAddrFallsBackWhenThePortIsTaken(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	taken := ln.Addr().String()

	got := freeAddr(taken)
	if got == taken {
		t.Fatalf("freeAddr(%q) returned the occupied address; a desktop launch would die on ListenAndServe", taken)
	}
	host, _, err := net.SplitHostPort(got)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", got, err)
	}
	if host != "127.0.0.1" {
		t.Errorf("freeAddr fell back to host %q, want the configured 127.0.0.1", host)
	}
	// The fallback must actually be usable.
	probe, err := net.Listen("tcp", got)
	if err != nil {
		t.Fatalf("fallback address %q is not bindable: %v", got, err)
	}
	_ = probe.Close()
}

func TestDataDirIsOSAppropriate(t *testing.T) {
	t.Setenv("XCHATS_DATA_DIR", "")
	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	if filepath.Base(dir) != AppName {
		t.Errorf("DataDir = %q, want it to end in %q (see internal/appdirs for the per-OS rules; GOOS=%s)", dir, AppName, runtime.GOOS)
	}
}

func TestApplyDefaultsPointsTheAPIBaseURLAtTheRealPort(t *testing.T) {
	clearStorageEnv(t)
	t.Setenv("XCHATS_DATA_DIR", t.TempDir())

	// Occupy the configured port so freeAddr has to move the listener.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	taken := ln.Addr().String()

	cfg := loadDefaults(t)
	cfg.Server.HTTPAddr = taken
	if err := ApplyDefaults(cfg); err != nil {
		t.Fatalf("ApplyDefaults: %v", err)
	}
	if cfg.Server.HTTPAddr == taken {
		t.Fatalf("HTTPAddr stayed on the occupied port %q", taken)
	}
	// ResolvedAPIBaseURL prefers the configured value over the listen
	// address, so a stale one would mint MCP discovery and signed-media URLs
	// for a port nothing is listening on.
	if want := "http://" + cfg.Server.HTTPAddr; cfg.Server.APIBaseURL != want {
		t.Errorf("APIBaseURL = %q, want %q", cfg.Server.APIBaseURL, want)
	}
}

func TestApplyDefaultsKeepsAConfiguredPublicOrigin(t *testing.T) {
	clearStorageEnv(t)
	t.Setenv("XCHATS_DATA_DIR", t.TempDir())

	cfg := loadDefaults(t)
	cfg.Server.HTTPAddr = ":0"
	cfg.Server.APIBaseURL = "https://xchats.example.com"
	if err := ApplyDefaults(cfg); err != nil {
		t.Fatalf("ApplyDefaults: %v", err)
	}
	if cfg.Server.APIBaseURL != "https://xchats.example.com" {
		t.Errorf("APIBaseURL = %q, want the configured public origin untouched", cfg.Server.APIBaseURL)
	}
}

func TestLocalAPIBaseURL(t *testing.T) {
	cases := []struct{ configured, addr, want string }{
		// config.yaml's own default, and the shape ApplyDefaults produces.
		{"http://localhost:8080", "127.0.0.1:9123", "http://127.0.0.1:9123"},
		{"", "127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"http://127.0.0.1:8080", "127.0.0.1:8080", "http://127.0.0.1:8080"},
		// A real origin is a statement about the outside world, not the
		// local listener.
		{"https://xchats.example.com", "127.0.0.1:8080", "https://xchats.example.com"},
		{"https://abc.ngrok-free.app", "127.0.0.1:8080", "https://abc.ngrok-free.app"},
		// Nothing knowable to rewrite to.
		{"http://localhost:8080", "127.0.0.1:0", "http://localhost:8080"},
		{"http://localhost:8080", "garbage", "http://localhost:8080"},
	}
	for _, c := range cases {
		if got := localAPIBaseURL(c.configured, c.addr); got != c.want {
			t.Errorf("localAPIBaseURL(%q, %q) = %q, want %q", c.configured, c.addr, got, c.want)
		}
	}
}
