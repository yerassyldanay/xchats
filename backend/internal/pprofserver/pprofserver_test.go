package pprofserver

import (
	"context"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestValidateLoopbackAddr(t *testing.T) {
	cases := map[string]bool{ // addr -> wantOK
		"127.0.0.1:6060":   true,
		"127.0.0.1:0":      true,
		"localhost:6060":   true,
		"[::1]:6060":       true,
		":6060":            false, // all interfaces
		"0.0.0.0:6060":     false, // all interfaces
		"example.com:6060": false,
		"not-a-valid-addr": false,
	}
	for addr, wantOK := range cases {
		err := ValidateLoopbackAddr(addr)
		if (err == nil) != wantOK {
			t.Errorf("ValidateLoopbackAddr(%q) error = %v, want ok=%v", addr, err, wantOK)
		}
	}
}

func TestStart_RejectsNonLoopbackAddr(t *testing.T) {
	if _, err := Start("0.0.0.0:0"); err == nil {
		t.Fatal("expected Start to reject a non-loopback address")
	}
}

func TestStart_BindsSynchronouslyAndServesPprofSurface(t *testing.T) {
	srv, err := Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	base := "http://" + srv.Addr()
	client := &http.Client{Timeout: 5 * time.Second}

	// Index page (lists every registered profile).
	resp, err := client.Get(base + "/debug/pprof/")
	if err != nil {
		t.Fatalf("GET /debug/pprof/: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /debug/pprof/ status = %d, body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "goroutine") {
		t.Errorf("index page missing expected profile listing, body=%s", body)
	}

	// cmdline: one of the five special net/http/pprof handlers.
	resp, err = client.Get(base + "/debug/pprof/cmdline")
	if err != nil {
		t.Fatalf("GET /debug/pprof/cmdline: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /debug/pprof/cmdline status = %d", resp.StatusCode)
	}

	// goroutine?debug=2: a named runtime/pprof.Profile, served through
	// Index's own dispatch (not one of the five special-cased handlers) —
	// this is the exact "goroutine?debug=2 stack dump" the harness captures.
	resp, err = client.Get(base + "/debug/pprof/goroutine?debug=2")
	if err != nil {
		t.Fatalf("GET /debug/pprof/goroutine?debug=2: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(body) == 0 {
		t.Fatalf("GET /debug/pprof/goroutine?debug=2 status=%d len=%d", resp.StatusCode, len(body))
	}

	// heap: another named profile via the same dispatch path.
	resp, err = client.Get(base + "/debug/pprof/heap")
	if err != nil {
		t.Fatalf("GET /debug/pprof/heap: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /debug/pprof/heap status = %d", resp.StatusCode)
	}
}

func TestStart_SetsMutexProfileFractionAndShutdownResets(t *testing.T) {
	srv, err := Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// SetMutexProfileFraction returns the PREVIOUS rate — calling it again
	// with the same value both confirms what Start set and leaves the rate
	// unchanged (idempotent probe).
	if got := runtime.SetMutexProfileFraction(mutexProfileFraction); got != mutexProfileFraction {
		t.Errorf("mutex profile fraction while active = %d, want %d", got, mutexProfileFraction)
	}
	if err := srv.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if got := runtime.SetMutexProfileFraction(0); got != 0 {
		t.Errorf("mutex profile fraction after Shutdown = %d, want 0 (reset)", got)
	}
}

func TestStart_AddressConflictFailsStartup(t *testing.T) {
	first, err := Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Start (first): %v", err)
	}
	defer func() { _ = first.Shutdown(context.Background()) }()

	if _, err := Start(first.Addr()); err == nil {
		t.Fatal("expected a second Start on the same address to fail synchronously")
	}
}

func TestShutdown_StopsServing(t *testing.T) {
	srv, err := Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	addr := srv.Addr()
	if err := srv.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	if _, err := client.Get("http://" + addr + "/debug/pprof/"); err == nil {
		t.Fatal("expected requests to fail after Shutdown")
	}
}
