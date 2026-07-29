package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"strings"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// runKBPreflight is a deploy-time gate for the ai_assets/ai_draft_assets
// retirement (see plan/database-schema.md and the PR-1 landing note it
// links). Exploration on 2026-07-29 found both tables empty and kbd_draft /
// kbd_requests empty too, but that snapshot goes stale the moment anyone
// touches the dormant Playground draft/approve path before this deploy
// lands. This command re-checks live, right before the code that reads
// those rows is removed:
//
//   - ai_assets / ai_draft_assets: any row blocks the deploy outright — PR 1
//     deletes every reader/writer, and the canonical semantic-media shape
//     those rows would need to migrate into does not exist until Steps 8-9.
//   - kbd_draft: any row whose draft blob contains a non-empty "assets" array
//     blocks the deploy for the same reason (that field is about to become
//     unreadable dead JSON).
//   - kbd_requests: rows already targeting the legacy req_type "describe_media"
//     are not blocking — they are transactionally renamed to the canonical
//     "describe_file" in the same run, since PR 2's handler only recognizes
//     the new name.
//
// Every blocking condition reports the organization_id/row id, never just a
// count, so an operator can go drain or export the exact rows instead of
// guessing.
func runKBPreflight(cfg *config.Config, log *slog.Logger, args []string) {
	fs := flag.NewFlagSet("kb-preflight", flag.ExitOnError)
	_ = fs.Parse(args)

	ctx := context.Background()
	st := mustStore(cfg, log)
	defer st.Close()

	report, err := kbPreflightCheck(ctx, st)
	if err != nil {
		fatal("kb-preflight", err)
	}
	if len(report.Blocking) > 0 {
		fatal("kb-preflight", errString("blocked:\n"+strings.Join(report.Blocking, "\n")))
	}
	log.Info("kb-preflight passed", "renamed_requests", report.RenamedRequests)
}

type kbPreflightReport struct {
	// Blocking holds one human-readable line per offending row; a non-empty
	// slice means the deploy must not proceed.
	Blocking []string
	// RenamedRequests is how many kbd_requests rows were flipped from the
	// legacy req_type "describe_media" to canonical "describe_file".
	RenamedRequests int
}

// kbPreflightCheck runs the checks documented on runKBPreflight against the
// already-migrated database st points at. It is factored out from
// runKBPreflight so the DB-backed test can call it directly against a
// scratch schema without going through flag parsing / log setup.
func kbPreflightCheck(ctx context.Context, st *store.Store) (kbPreflightReport, error) {
	var report kbPreflightReport
	pool := st.Pool()

	type offender struct {
		orgID, rowID string
	}
	collect := func(query, label string) error {
		rows, err := pool.Query(ctx, query)
		if err != nil {
			return fmt.Errorf("query %s: %w", label, err)
		}
		defer rows.Close()
		for rows.Next() {
			var o offender
			if err := rows.Scan(&o.orgID, &o.rowID); err != nil {
				return fmt.Errorf("scan %s: %w", label, err)
			}
			report.Blocking = append(report.Blocking,
				fmt.Sprintf("%s: organization_id=%s id=%s", label, o.orgID, o.rowID))
		}
		return rows.Err()
	}

	if err := collect(`SELECT organization_id::text, id::text FROM xchats.ai_assets ORDER BY organization_id, id`,
		"ai_assets has a row"); err != nil {
		return report, err
	}
	if err := collect(`SELECT COALESCE(wa.organization_id::text, 'unknown'), ad.id::text
		FROM xchats.ai_draft_assets a
		JOIN xchats.ai_drafts ad ON ad.id = a.draft_id
		JOIN xchats.wa_chats c ON c.id = ad.chat_id
		JOIN xchats.wa_accounts wa ON wa.id = c.account_id
		ORDER BY ad.id`,
		"ai_draft_assets has a row"); err != nil {
		return report, err
	}
	if err := collect(`SELECT organization_id::text, organization_id::text FROM xchats.kbd_draft
		WHERE jsonb_array_length(COALESCE(draft->'assets', '[]'::jsonb)) > 0
		ORDER BY organization_id`,
		"kbd_draft has a non-empty assets array"); err != nil {
		return report, err
	}

	// kbd_requests targeting the legacy req_type is not blocking — rename it
	// transactionally to the canonical name PR 2's handler expects.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return report, fmt.Errorf("begin rename tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE xchats.kbd_requests SET req_type = 'describe_file'
		WHERE req_type = 'describe_media'`)
	if err != nil {
		return report, fmt.Errorf("rename describe_media requests: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return report, fmt.Errorf("commit rename tx: %w", err)
	}
	report.RenamedRequests = int(tag.RowsAffected())

	return report, nil
}
