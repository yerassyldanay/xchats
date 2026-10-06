package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var descriptionPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Create writes one migration file, shared by every dialect, named with a UTC
// timestamp prefix. Exclusive creation prevents overwrites, including a
// same-second same-name collision. It returns the identifier (the file name
// without .sql).
func Create(root, description string, now time.Time) (string, error) {
	if !descriptionPattern.MatchString(description) {
		return "", fmt.Errorf("use a lowercase snake_case migration name")
	}
	id := now.UTC().Format("20060102150405") + "_" + description
	path := filepath.Join(root, id+".sql")
	// #nosec G304 -- root is an explicit developer CLI path; the description is constrained.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	_, err = fmt.Fprintf(f, "-- %s\n-- Shared by SQLite and PostgreSQL; use the dialect macros from docs/database.md.\n-- Write idempotent SQL; the runner supplies the transaction.\n", description)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return id, nil
}
