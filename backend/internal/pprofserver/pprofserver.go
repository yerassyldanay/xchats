// Package pprofserver is the hermetic profiling harness's dedicated pprof
// HTTP listener (cmd/xchats' system.pprof_addr/PPROF_ADDR/--pprof-addr) —
// entirely separate from the main application router, loopback-only, and
// mounting the complete standard net/http/pprof surface on a private mux.
package pprofserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"runtime"
)

// mutexProfileFraction/blockProfileRate are the sampling rates enabled for
// as long as the pprof server is active — see runtime.SetMutexProfileFraction
// and runtime.SetBlockProfileRate's own docs for what each samples. Reset
// to 0 (off) on Shutdown so a profiling run never leaves sampling enabled
// for the rest of the process's life.
const (
	mutexProfileFraction = 5
	blockProfileRate     = 1
)

// Server is a running pprof listener.
type Server struct {
	httpServer *http.Server
	listener   net.Listener
}

// ValidateLoopbackAddr reports an error unless addr is a host:port whose
// host is a literal loopback IP or "localhost" — never an empty host
// (":6060", which binds every interface) or any other name/IP. pprof
// exposes full runtime internals (goroutine stacks, command-line
// arguments, heap contents via allocation site) with no authentication of
// its own, so accepting anything wider than loopback here would turn a
// local profiling convenience into a real exposure.
func ValidateLoopbackAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("pprof addr %q: %w", addr, err)
	}
	if host == "" {
		return fmt.Errorf("pprof addr %q must bind an explicit loopback host (e.g. 127.0.0.1:6060), not all interfaces", addr)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("pprof addr %q: host %q is not a literal loopback IP", addr, host)
	}
	if !ip.IsLoopback() {
		return fmt.Errorf("pprof addr %q: host %q is not a loopback address", addr, host)
	}
	return nil
}

// Start validates addr, binds SYNCHRONOUSLY (an address conflict is
// returned to the caller immediately, never surfacing later as a failed
// first request), mounts the complete standard net/http/pprof surface
// (index/cmdline/profile/symbol/trace, and — via Index's own dispatch —
// every named runtime/pprof.Profile: heap, goroutine, threadcreate, block,
// mutex, allocs) on a private http.ServeMux, enables mutex/block profiling
// for as long as the server runs, and starts serving in the background.
// Call Shutdown to stop serving and reset the sampling rates.
func Start(addr string) (*Server, error) {
	if err := ValidateLoopbackAddr(addr); err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("pprof listen %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	runtime.SetMutexProfileFraction(mutexProfileFraction)
	runtime.SetBlockProfileRate(blockProfileRate)

	s := &Server{
		httpServer: &http.Server{Handler: mux},
		listener:   ln,
	}
	go func() {
		_ = s.httpServer.Serve(ln)
	}()
	return s, nil
}

// Addr returns the listener's actual bound address — useful when Start was
// called with a ":0"-style ephemeral port (tests; never the real profiling
// target, which always names an explicit port).
func (s *Server) Addr() string { return s.listener.Addr().String() }

// Shutdown stops serving and resets the mutex/block sampling rates to 0
// (off) — called from cmd/xchats alongside the main application server's
// own shutdown, so a profiling run never leaves elevated sampling active
// past the process it was for.
func (s *Server) Shutdown(ctx context.Context) error {
	defer func() {
		runtime.SetMutexProfileFraction(0)
		runtime.SetBlockProfileRate(0)
	}()
	return s.httpServer.Shutdown(ctx)
}
