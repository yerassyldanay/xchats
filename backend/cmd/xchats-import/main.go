// Command xchats-import is a one-time, standalone data migration tool: it
// reads every row of every table out of a Postgres database running the
// frozen pre-cutover schema (migrations/postgres) and writes it into a
// SQLite database, preserving every primary key exactly (see
// internal/pgimport's own package doc for the full design). It is never
// linked into the xchats server binary (cmd/xchats) — a separate binary
// specifically so the server never carries a pgx dependency.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/pgimport"
	sqlitemigrations "github.com/yerassyldanay/xchats/backend/migrations/sqlite"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run holds main's whole logic, taking args/stdout/stderr explicitly (a
// fresh flag.FlagSet per call, not the global flag.CommandLine — the
// standard shape for testing a flag-based CLI without cross-test global
// state) rather than reading os.Args/os.Std* directly, so main_test.go can
// exercise it repeatedly.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("xchats-import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pgURL := fs.String("postgres-url", "", "source Postgres connection string (postgres://user:pass@host:port/dbname)")
	sqlitePath := fs.String("sqlite-path", "", "destination SQLite database file path — created and migrated if it does not already exist")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *pgURL == "" || *sqlitePath == "" {
		fmt.Fprintln(stderr, "usage: xchats-import -postgres-url postgres://user:pass@host:port/dbname -sqlite-path ./xchats.db")
		fs.PrintDefaults()
		return 2
	}

	ctx := context.Background()

	db, err := dbx.Open(ctx, *sqlitePath)
	if err != nil {
		if dbx.IsSingleProcessLockErr(err) {
			fmt.Fprintf(stderr, "xchats-import: %s is already open by another process — stop it first: %v\n", *sqlitePath, err)
			return 1
		}
		fmt.Fprintf(stderr, "xchats-import: open %s: %v\n", *sqlitePath, err)
		return 1
	}
	defer db.Close()

	if err := dbx.RunMigrations(ctx, db, sqlitemigrations.FS); err != nil {
		fmt.Fprintf(stderr, "xchats-import: migrate %s: %v\n", *sqlitePath, err)
		return 1
	}

	result, err := pgimport.Import(ctx, *pgURL, db)
	if err != nil {
		fmt.Fprintf(stderr, "xchats-import: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "xchats-import: imported %d rows across %d tables into %s\n", result.TotalRows(), len(result.Tables), *sqlitePath)
	for _, t := range result.Tables {
		if t.Rows > 0 {
			fmt.Fprintf(stdout, "  %-30s %d\n", t.Table, t.Rows)
		}
	}
	return 0
}
