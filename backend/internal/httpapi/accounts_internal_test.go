package httpapi

import (
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/evolution"
)

// TestIsStale exercises the pure stale-instance rule with no Evolution/DB.
func TestIsStale(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	old := now.Add(-30 * 24 * time.Hour)
	fresh := now.Add(-1 * time.Hour)

	cases := []struct {
		name string
		in   evolution.Instance
		want bool
	}{
		{"connected-old-not-stale", evolution.Instance{ConnectionStatus: "open", CreatedAt: old}, false},
		{"disconnected-old-stale", evolution.Instance{ConnectionStatus: "close", CreatedAt: old}, true},
		{"disconnected-fresh-not-stale", evolution.Instance{ConnectionStatus: "close", CreatedAt: fresh}, false},
		{"connecting-old-stale", evolution.Instance{ConnectionStatus: "connecting", CreatedAt: old}, true},
		{"unknown-age-not-stale", evolution.Instance{ConnectionStatus: "close"}, false},
	}
	for _, tc := range cases {
		if got := isStale(tc.in, now); got != tc.want {
			t.Errorf("%s: isStale = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestMapConnState(t *testing.T) {
	cases := map[string]string{
		"open": "connected", "connecting": "connecting", "close": "disconnected",
		"closed": "disconnected", "": "", "weird": "weird",
	}
	for in, want := range cases {
		if got := mapConnState(in); got != want {
			t.Errorf("mapConnState(%q) = %q, want %q", in, got, want)
		}
	}
}
