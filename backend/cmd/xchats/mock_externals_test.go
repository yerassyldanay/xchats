package main

// mock_externals_test.go covers the hermetic-profiling-harness startup gate
// (checkMockExternalsAllowed) and the --mock-externals/--pprof-addr CLI
// precedence (applyCLIOverrides) — mirrors production_config_test.go's and
// data_dir_flag_test.go's own shape for the same kind of composition-root
// logic.

import (
	"flag"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
)

func TestCheckMockExternalsAllowed(t *testing.T) {
	cases := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{"mock off, production", &config.Config{Environment: "production"}, false},
		{"mock on, development", &config.Config{Environment: "development", System: config.SystemConfig{MockExternals: true}}, false},
		{"mock on, unset environment (defaults to development)", &config.Config{System: config.SystemConfig{MockExternals: true}}, false},
		{"mock on, production", &config.Config{Environment: "production", System: config.SystemConfig{MockExternals: true}}, true},
		{"mock on, Production (case-insensitive)", &config.Config{Environment: "Production", System: config.SystemConfig{MockExternals: true}}, true},
		{"mock off, development", &config.Config{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkMockExternalsAllowed(tc.cfg)
			if (err != nil) != tc.wantErr {
				t.Errorf("checkMockExternalsAllowed() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

// newTestFlags builds the exact --mock-externals/--pprof-addr flag pair main()
// registers, on a throwaway FlagSet so tests never touch the process's real
// flag.CommandLine.
func newTestFlags() (fs *flag.FlagSet, mockExternals *bool, pprofAddr *string) {
	fs = flag.NewFlagSet("test", flag.ContinueOnError)
	mockExternals = fs.Bool("mock-externals", false, "")
	pprofAddr = fs.String("pprof-addr", "", "")
	return fs, mockExternals, pprofAddr
}

func TestApplyCLIOverrides_UnsetFlagsLeaveConfigUntouched(t *testing.T) {
	fs, mockExternals, pprofAddr := newTestFlags()
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg := &config.Config{System: config.SystemConfig{MockExternals: true, PprofAddr: "127.0.0.1:9000"}}
	applyCLIOverrides(fs, cfg, mockExternals, pprofAddr)
	if !cfg.System.MockExternals || cfg.System.PprofAddr != "127.0.0.1:9000" {
		t.Errorf("unset CLI flags must not override config/env values, got %+v", cfg.System)
	}
}

func TestApplyCLIOverrides_ExplicitFlagsWinOverConfig(t *testing.T) {
	fs, mockExternals, pprofAddr := newTestFlags()
	if err := fs.Parse([]string{"-mock-externals=true", "-pprof-addr=127.0.0.1:6060"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Config/env resolved the opposite values — CLI must still win.
	cfg := &config.Config{System: config.SystemConfig{MockExternals: false, PprofAddr: "127.0.0.1:9000"}}
	applyCLIOverrides(fs, cfg, mockExternals, pprofAddr)
	if !cfg.System.MockExternals {
		t.Error("explicit -mock-externals=true must override config/env")
	}
	if cfg.System.PprofAddr != "127.0.0.1:6060" {
		t.Errorf("explicit -pprof-addr must override config/env, got %q", cfg.System.PprofAddr)
	}
}

// TestApplyCLIOverrides_ExplicitFalseWinsOverConfig is the case a sentinel
// zero-value check would get wrong: an operator explicitly passing
// -mock-externals=false must be able to turn OFF a config.yaml/env value of
// true — flag.Visit (rather than "is the value non-zero") is what makes this
// work.
func TestApplyCLIOverrides_ExplicitFalseWinsOverConfig(t *testing.T) {
	fs, mockExternals, pprofAddr := newTestFlags()
	if err := fs.Parse([]string{"-mock-externals=false"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg := &config.Config{System: config.SystemConfig{MockExternals: true}}
	applyCLIOverrides(fs, cfg, mockExternals, pprofAddr)
	if cfg.System.MockExternals {
		t.Error("explicit -mock-externals=false must override a config/env value of true")
	}
}
