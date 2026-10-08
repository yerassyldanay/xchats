package config

import (
	"strings"
	"testing"
)

func TestIsPostgresURL(t *testing.T) {
	for target, want := range map[string]bool{
		"postgres://u:p@h:5432/db":     true,
		"postgresql://h/db?sslmode=on": true,
		"./data/xchats.db":             false,
		"/var/lib/xchats/xchats.db":    false,
		"C:\\xchats\\xchats.db":        false,
		"":                             false,
		"mypostgres://x":               false,
	} {
		if got := IsPostgresURL(target); got != want {
			t.Errorf("IsPostgresURL(%q) = %v, want %v", target, got, want)
		}
	}
}

// The storage locations are shown in Settings and end up in bug reports. A PostgreSQL URL
// carries the password (and may carry it in a query parameter), so only scheme, host and
// database name may ever be shown.
func TestDatabaseLocationNeverCarriesCredentials(t *testing.T) {
	const secret = "s3cr3t-Pa55"
	tests := []struct {
		name string
		s    StorageConfig
		db   string
		dev  string
	}{
		{"separate postgres urls", StorageConfig{DatabaseTarget: "postgres://app:" + secret + "@db.example:5432/xchats?sslmode=require&password=" + secret, WADeviceDatabaseTarget: "postgres://wa:" + secret + "@db.example:5432/whatsmeow?sslmode=require"},
			"postgres://db.example:5432/xchats", "postgres://db.example:5432/whatsmeow"},
		{"url and sqlite", StorageConfig{DatabaseTarget: "postgresql://app:" + secret + "@h/xchats", WADeviceDatabaseTarget: "/data/whatsmeow.db"},
			"postgresql://h/xchats", "/data/whatsmeow.db"},
		{"sqlite", StorageConfig{DatabaseTarget: "/data/xchats.db", WADeviceDatabaseTarget: "/data/whatsmeow.db"},
			"/data/xchats.db", "/data/whatsmeow.db"},
		{"unparseable url", StorageConfig{DatabaseTarget: "postgres://app:" + secret + "@exa mple/xchats", WADeviceDatabaseTarget: "postgres://wa:" + secret + "@exa mple/whatsmeow"},
			"postgres://", "postgres://"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotDB, gotDev := tc.s.DatabaseLocation(), tc.s.DeviceDatabaseLocation()
			if gotDB != tc.db || gotDev != tc.dev {
				t.Errorf("locations = %q and %q, want %q and %q", gotDB, gotDev, tc.db, tc.dev)
			}
			if strings.Contains(gotDB+gotDev, secret) {
				t.Errorf("a location leaks the password: %q %q", gotDB, gotDev)
			}
		})
	}
}
