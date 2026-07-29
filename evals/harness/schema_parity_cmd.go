package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"xchats-evals-harness/internal/schemamanifest"
)

// cmdSchemaParity is the opt-in, explicit DB-parity check: it never runs as
// part of `run`/`judge`/the default test suite (today's schema is
// intentionally known to drift — see plan Step 7), so an operator invokes it
// by name once the additive migration (0011) or the enforcing migration
// (0012) has landed against the target database.
func cmdSchemaParity(args []string) error {
	fs := flag.NewFlagSet("schema-parity", flag.ExitOnError)
	dsn := fs.String("db", "", "DATABASE_URL to compare (required)")
	modeFlag := fs.String("mode", "transitional", "transitional (post-0011, legacy columns/tables still present) | strict (post-0012)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dsn == "" {
		return fmt.Errorf("schema-parity: -db is required")
	}
	var mode schemamanifest.Mode
	switch *modeFlag {
	case "transitional":
		mode = schemamanifest.ModeTransitional
	case "strict":
		mode = schemamanifest.ModeStrict
	default:
		return fmt.Errorf("schema-parity: -mode must be transitional or strict, got %q", *modeFlag)
	}

	manifest, err := schemamanifest.BuildManifest()
	if err != nil {
		return fmt.Errorf("schema-parity: build manifest: %w", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		return fmt.Errorf("schema-parity: connect: %w", err)
	}
	defer pool.Close()

	diffs, err := schemamanifest.CompareToDB(ctx, pool, manifest, mode)
	if err != nil {
		return fmt.Errorf("schema-parity: compare: %w", err)
	}
	if len(diffs) == 0 {
		fmt.Printf("schema-parity: OK — %d tables match the eval manifest (%s mode)\n", len(manifest.Tables), *modeFlag)
		return nil
	}
	fmt.Printf("schema-parity: %d mismatch(es) against the eval manifest (%s mode):\n", len(diffs), *modeFlag)
	for _, d := range diffs {
		fmt.Println("  " + d)
	}
	return fmt.Errorf("schema-parity: %d mismatch(es)", len(diffs))
}
