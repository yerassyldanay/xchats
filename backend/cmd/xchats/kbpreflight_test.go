package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/migrations"
)

func newKBPreflightTestStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping kb-preflight DB test")
	}
	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, err := st.Pool().Exec(ctx, `DROP SCHEMA IF EXISTS xchats CASCADE; DROP TABLE IF EXISTS public.xchats_schema_migrations`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := store.RunMigrations(ctx, st.Pool(), migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(st.Close)
	return st
}

func mustOrg(t *testing.T, st *store.Store) uuid.UUID {
	t.Helper()
	org, err := st.SeedOrganization(context.Background(), "kb-preflight-test-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	return org.ID
}

func TestKBPreflight_PassesOnCleanDB(t *testing.T) {
	st := newKBPreflightTestStore(t)
	mustOrg(t, st)

	report, err := kbPreflightCheck(context.Background(), st)
	if err != nil {
		t.Fatalf("kbPreflightCheck: %v", err)
	}
	if len(report.Blocking) != 0 {
		t.Fatalf("want no blocking rows on a clean DB, got %v", report.Blocking)
	}
	if report.RenamedRequests != 0 {
		t.Fatalf("want 0 renamed requests on a clean DB, got %d", report.RenamedRequests)
	}
}

func TestKBPreflight_BlocksOnAiAssetsRow(t *testing.T) {
	st := newKBPreflightTestStore(t)
	ctx := context.Background()
	orgID := mustOrg(t, st)

	if _, err := st.Pool().Exec(ctx, `INSERT INTO xchats.ai_assets
		(organization_id, ref, owner_kind, owner_ref) VALUES ($1, 'stray_asset', '', '')`, orgID); err != nil {
		t.Fatalf("insert stray ai_assets row: %v", err)
	}

	report, err := kbPreflightCheck(ctx, st)
	if err != nil {
		t.Fatalf("kbPreflightCheck: %v", err)
	}
	if len(report.Blocking) != 1 {
		t.Fatalf("want exactly 1 blocking row, got %v", report.Blocking)
	}
	if got := report.Blocking[0]; !strings.Contains(got, orgID.String()) || !strings.Contains(got, "ai_assets") {
		t.Fatalf("blocking message must name the org and the table, got %q", got)
	}
}

func TestKBPreflight_BlocksOnAiDraftAssetsRow(t *testing.T) {
	st := newKBPreflightTestStore(t)
	ctx := context.Background()
	orgID := mustOrg(t, st)

	var chatID, contactID, accountID uuid.UUID
	if err := st.Pool().QueryRow(ctx, `INSERT INTO xchats.wa_accounts (id, organization_id, owner_jid)
		VALUES (gen_random_uuid(), $1, 'stray-owner@s.whatsapp.net') RETURNING id`, orgID).Scan(&accountID); err != nil {
		t.Fatalf("insert wa_account: %v", err)
	}
	if err := st.Pool().QueryRow(ctx, `INSERT INTO xchats.wa_contacts (account_id, phone_jid)
		VALUES ($1, 'stray@s.whatsapp.net') RETURNING id`, accountID).Scan(&contactID); err != nil {
		t.Fatalf("insert wa_contact: %v", err)
	}
	if err := st.Pool().QueryRow(ctx, `INSERT INTO xchats.wa_chats (account_id, contact_id, remote_jid)
		VALUES ($1, $2, 'stray@s.whatsapp.net') RETURNING id`, accountID, contactID).Scan(&chatID); err != nil {
		t.Fatalf("insert wa_chat: %v", err)
	}
	var draftID uuid.UUID
	if err := st.Pool().QueryRow(ctx, `INSERT INTO xchats.ai_drafts (chat_id, option_ordinal)
		VALUES ($1, 0) RETURNING id`, chatID).Scan(&draftID); err != nil {
		t.Fatalf("insert ai_draft: %v", err)
	}
	if _, err := st.Pool().Exec(ctx, `INSERT INTO xchats.ai_draft_assets (draft_id, ordinal)
		VALUES ($1, 0)`, draftID); err != nil {
		t.Fatalf("insert stray ai_draft_assets row: %v", err)
	}

	report, err := kbPreflightCheck(ctx, st)
	if err != nil {
		t.Fatalf("kbPreflightCheck: %v", err)
	}
	if len(report.Blocking) != 1 {
		t.Fatalf("want exactly 1 blocking row, got %v", report.Blocking)
	}
	if got := report.Blocking[0]; !strings.Contains(got, orgID.String()) || !strings.Contains(got, "ai_draft_assets") {
		t.Fatalf("blocking message must name the org and the table, got %q", got)
	}
}

func TestKBPreflight_BlocksOnDraftBlobAssets(t *testing.T) {
	st := newKBPreflightTestStore(t)
	ctx := context.Background()
	orgID := mustOrg(t, st)

	if _, err := st.Pool().Exec(ctx, `INSERT INTO xchats.kbd_draft (organization_id, draft)
		VALUES ($1, '{"assets":[{"ref":"stray"}]}'::jsonb)`, orgID); err != nil {
		t.Fatalf("insert stray kbd_draft row: %v", err)
	}

	report, err := kbPreflightCheck(ctx, st)
	if err != nil {
		t.Fatalf("kbPreflightCheck: %v", err)
	}
	if len(report.Blocking) != 1 {
		t.Fatalf("want exactly 1 blocking row, got %v", report.Blocking)
	}
	if got := report.Blocking[0]; !strings.Contains(got, orgID.String()) || !strings.Contains(got, "kbd_draft") {
		t.Fatalf("blocking message must name the org and the table, got %q", got)
	}
}

func TestKBPreflight_RenamesDescribeMediaRequests(t *testing.T) {
	st := newKBPreflightTestStore(t)
	ctx := context.Background()
	orgID := mustOrg(t, st)

	if _, err := st.Pool().Exec(ctx, `INSERT INTO xchats.kbd_requests (organization_id, req_type)
		VALUES ($1, 'describe_media'), ($1, 'confirm_fact')`, orgID); err != nil {
		t.Fatalf("insert kbd_requests rows: %v", err)
	}

	report, err := kbPreflightCheck(ctx, st)
	if err != nil {
		t.Fatalf("kbPreflightCheck: %v", err)
	}
	if len(report.Blocking) != 0 {
		t.Fatalf("renaming describe_media must not block, got %v", report.Blocking)
	}
	if report.RenamedRequests != 1 {
		t.Fatalf("want exactly 1 renamed request, got %d", report.RenamedRequests)
	}

	var reqTypes []string
	rows, err := st.Pool().Query(ctx, `SELECT req_type FROM xchats.kbd_requests WHERE organization_id = $1 ORDER BY req_type`, orgID)
	if err != nil {
		t.Fatalf("query kbd_requests: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var rt string
		if err := rows.Scan(&rt); err != nil {
			t.Fatalf("scan req_type: %v", err)
		}
		reqTypes = append(reqTypes, rt)
	}
	if len(reqTypes) != 2 || reqTypes[0] != "confirm_fact" || reqTypes[1] != "describe_file" {
		t.Fatalf("want [confirm_fact describe_file], got %v", reqTypes)
	}

	// Idempotent: running again finds nothing left to rename.
	report2, err := kbPreflightCheck(ctx, st)
	if err != nil {
		t.Fatalf("kbPreflightCheck (2nd run): %v", err)
	}
	if report2.RenamedRequests != 0 {
		t.Fatalf("2nd run should rename 0 rows, got %d", report2.RenamedRequests)
	}
}

