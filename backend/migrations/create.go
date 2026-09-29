package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var descriptionPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Create writes a pair of migration files using one UTC timestamp. Exclusive
// creation prevents overwrites, including a same-second same-name collision.
func Create(root, description string, now time.Time) (string, error) {
	if !descriptionPattern.MatchString(description) {
		return "", fmt.Errorf("use a lowercase snake_case migration name")
	}
	id := now.UTC().Format("20060102150405") + "_" + description
	var created []string
	for _, dialect := range []string{"sqlite", "postgres"} {
		path := filepath.Join(root, dialect, id+".sql")
		// #nosec G304 -- root is an explicit developer CLI path; dialect and description are constrained.
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			for _, p := range created {
				_ = os.Remove(p)
			}
			return "", err
		}
		created = append(created, path)
		_, err = fmt.Fprintf(f, "-- %s (%s)\n-- Write idempotent SQL; the runner supplies the transaction.\n", description, dialect)
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			for _, p := range created {
				_ = os.Remove(p)
			}
			return "", err
		}
	}
	return id, nil
}
