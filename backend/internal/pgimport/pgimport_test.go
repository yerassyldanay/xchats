package pgimport_test

import (
	"context"
	"os"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/pgimport"
)

// pgDSN returns the Postgres connection string this package's tests import
// FROM, or "" if none is configured. Unlike every other ported package,
// pgimport genuinely cannot be tested without a real Postgres instance —
// there is no way to fake "reading real rows out of Postgres" the way
// internal/dbtest fakes SQLite fixtures for everything else in this
// module. This is the one deliberate, permanent exception to "no
// DATABASE_URL-gated tests" elsewhere in the sqlite-cutover branch, not
// leftover legacy pattern.
//
// To run these tests: apply migrations/postgres's *.up.sql files (in
// order) to an empty Postgres database, then testdata/seed.sql (see its
// own header comment for the exact commands), then set PGIMPORT_TEST_DSN
// to that database's connection string.
func pgDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("PGIMPORT_TEST_DSN")
	if dsn == "" {
		t.Skip("PGIMPORT_TEST_DSN not set; skipping pgimport test (needs a real Postgres instance with migrations/postgres + testdata/seed.sql applied — see this function's doc comment)")
	}
	return dsn
}

func TestImport_RoundTrip(t *testing.T) {
	dsn := pgDSN(t)
	ctx := context.Background()
	_, db := dbtest.Open(t)

	result, err := pgimport.Import(ctx, dsn, db)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.TotalRows() == 0 {
		t.Fatal("Import reported zero total rows — the source Postgres database is expected to have test fixtures seeded (see the task description's manual psql setup)")
	}
	t.Logf("imported %d total rows across %d tables", result.TotalRows(), len(result.Tables))

	// The bootstrap admin/org must be gone, replaced by whatever the
	// source Postgres database's own organizations/users contain.
	var bootstrapCount int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM organizations WHERE id = '00000000-0000-0000-0000-000000000001'`).Scan(&bootstrapCount); err != nil {
		t.Fatalf("count bootstrap org: %v", err)
	}
	if bootstrapCount != 0 {
		t.Error("bootstrap organization (000...001) survived the import")
	}

	// uuid (cast to text), citext, text: organizations/users.
	var orgName string
	if err := db.QueryRow(ctx, `SELECT name FROM organizations WHERE id = '00000000-0000-0000-0000-0000000000aa'`).Scan(&orgName); err != nil {
		t.Fatalf("read imported organization: %v", err)
	}
	if orgName != "Test Org" {
		t.Errorf("organization name = %q, want %q", orgName, "Test Org")
	}
	var email string
	if err := db.QueryRow(ctx, `SELECT email FROM users WHERE id = '00000000-0000-0000-0000-0000000000bb'`).Scan(&email); err != nil {
		t.Fatalf("read imported user: %v", err)
	}
	if email != "Mixed@Example.com" {
		t.Errorf("user email = %q, want the exact original casing %q (citext preserves stored case)", email, "Mixed@Example.com")
	}

	// jsonb -> TEXT.
	var extraction string
	if err := db.QueryRow(ctx, `SELECT extraction FROM kbd_materials WHERE id = '00000000-0000-0000-0000-0000000000cc'`).Scan(&extraction); err != nil {
		t.Fatalf("read imported material: %v", err)
	}
	if extraction == "" {
		t.Error("kbd_materials.extraction imported as empty")
	}

	// uuid[] -> JSON-array TEXT (dbx.UUIDArray-shaped, written via
	// dbx.StringArray — see convert.go).
	rows, err := db.Query(ctx, `SELECT value FROM ai_topics, json_each(ai_topics.reference_documents) WHERE ai_topics.id = '00000000-0000-0000-0000-0000000000dd'`)
	if err != nil {
		t.Fatalf("query imported topic's reference_documents: %v", err)
	}
	var refDocs []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan: %v", err)
		}
		refDocs = append(refDocs, v)
	}
	rows.Close()
	if len(refDocs) != 2 {
		t.Errorf("ai_topics.reference_documents has %d elements, want 2 (a 2-element array with the SAME uuid repeated)", len(refDocs))
	}

	// text[] -> JSON-array TEXT.
	rows2, err := db.Query(ctx, `SELECT value FROM mcp_oauth_clients, json_each(mcp_oauth_clients.redirect_uris) WHERE mcp_oauth_clients.client_id = 'client1'`)
	if err != nil {
		t.Fatalf("query imported client's redirect_uris: %v", err)
	}
	var uris []string
	for rows2.Next() {
		var v string
		if err := rows2.Scan(&v); err != nil {
			t.Fatalf("scan: %v", err)
		}
		uris = append(uris, v)
	}
	rows2.Close()
	if len(uris) != 2 || uris[0] != "https://a.example/cb" || uris[1] != "https://b.example/cb" {
		t.Errorf("mcp_oauth_clients.redirect_uris = %v, want the 2 original URIs in order", uris)
	}

	// numeric -> REAL, boolean, NULL uuid stays NULL.
	var confidence float64
	var escalate bool
	var sentMessageID *string
	if err := db.QueryRow(ctx, `SELECT confidence, escalate, sent_message_id FROM ai_drafts WHERE id = '00000000-0000-0000-0000-0000000000ee'`).
		Scan(&confidence, &escalate, &sentMessageID); err != nil {
		t.Fatalf("read imported draft: %v", err)
	}
	if confidence < 0.8765 || confidence > 0.8766 {
		t.Errorf("ai_drafts.confidence = %v, want ~0.87654321", confidence)
	}
	if escalate {
		t.Error("ai_drafts.escalate = true, want false")
	}
	if sentMessageID != nil {
		t.Errorf("ai_drafts.sent_message_id = %v, want NULL preserved as NULL", *sentMessageID)
	}

	// bytea -> BLOB, integer.
	var tokenEnc []byte
	var keyVersion int
	if err := db.QueryRow(ctx, `SELECT bot_token_enc, encryption_key_version FROM tg_credentials WHERE account_id = '00000000-0000-0000-0000-0000000000ff'`).
		Scan(&tokenEnc, &keyVersion); err != nil {
		t.Fatalf("read imported credentials: %v", err)
	}
	wantBytes := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0xde, 0xad, 0xbe, 0xef}
	if len(tokenEnc) != len(wantBytes) {
		t.Fatalf("tg_credentials.bot_token_enc length = %d, want %d", len(tokenEnc), len(wantBytes))
	}
	for i := range wantBytes {
		if tokenEnc[i] != wantBytes[i] {
			t.Fatalf("tg_credentials.bot_token_enc = %x, want %x", tokenEnc, wantBytes)
		}
	}
	if keyVersion != 1 {
		t.Errorf("encryption_key_version = %d, want 1", keyVersion)
	}

	// bigint.
	var botID int64
	if err := db.QueryRow(ctx, `SELECT bot_id FROM tg_accounts WHERE id = '00000000-0000-0000-0000-0000000000ff'`).Scan(&botID); err != nil {
		t.Fatalf("read imported tg_account: %v", err)
	}
	if botID != 12345 {
		t.Errorf("tg_accounts.bot_id = %d, want 12345", botID)
	}

	// time without time zone -> "HH:MM:SS" TEXT.
	var start, end string
	if err := db.QueryRow(ctx, `SELECT respond_window_start, respond_window_end FROM organizations WHERE id = '00000000-0000-0000-0000-0000000000aa'`).
		Scan(&start, &end); err != nil {
		t.Fatalf("read imported org window: %v", err)
	}
	if start != "09:00:00" || end != "18:30:00" {
		t.Errorf("respond_window = %q..%q, want 09:00:00..18:30:00", start, end)
	}

	// timestamptz round-trips through the whole wa_* chain (multi-table
	// import, foreign keys preserved across tables via
	// PRAGMA defer_foreign_keys — see pgimport.go).
	var msgBody string
	if err := db.QueryRow(ctx, `SELECT m.body FROM wa_messages m
		JOIN wa_chats c ON c.id = m.chat_id
		JOIN wa_contacts ct ON ct.id = c.contact_id
		JOIN wa_accounts a ON a.id = c.account_id
		WHERE m.id = '00000000-0000-0000-0000-000000000104'`).Scan(&msgBody); err != nil {
		t.Fatalf("read imported wa_messages via its full FK chain (account/contact/chat all correctly imported and linked): %v", err)
	}
	if msgBody != "Привет! Сколько стоит?" {
		t.Errorf("wa_messages.body = %q, want the original text", msgBody)
	}
	var mediaCount int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM message_media WHERE message_id = '00000000-0000-0000-0000-000000000104'`).Scan(&mediaCount); err != nil {
		t.Fatalf("count imported message_media: %v", err)
	}
	if mediaCount != 1 {
		t.Errorf("message_media count for the imported message = %d, want 1", mediaCount)
	}
}

// TestImport_SecondRunFailsWithoutDuplicatingOrPartiallyImporting pins a
// deliberate, important behavior: Import is NOT idempotent — a second run
// against an already-imported target must fail (every primary key already
// exists), not silently duplicate rows or silently no-op. And because the
// whole import is one transaction (see pgimport.go's doc comment), the
// failed second attempt must leave the target in EXACTLY its
// already-imported state, never a half-applied second copy layered on top.
func TestImport_SecondRunFailsWithoutDuplicatingOrPartiallyImporting(t *testing.T) {
	dsn := pgDSN(t)
	ctx := context.Background()
	_, db := dbtest.Open(t)

	if _, err := pgimport.Import(ctx, dsn, db); err != nil {
		t.Fatalf("first Import: %v", err)
	}

	_, err := pgimport.Import(ctx, dsn, db)
	if err == nil {
		t.Fatal("second Import against an already-imported target unexpectedly succeeded")
	}
	t.Logf("second Import correctly failed: %v", err)

	var orgCount int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM organizations WHERE id = '00000000-0000-0000-0000-0000000000aa'`).Scan(&orgCount); err != nil {
		t.Fatalf("count organizations after the failed second import: %v", err)
	}
	if orgCount != 1 {
		t.Fatalf("organizations count for the test org = %d after a failed second import, want exactly 1 (no duplicate, no partial rollback artifact)", orgCount)
	}

	var wa int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM wa_messages`).Scan(&wa); err != nil {
		t.Fatalf("count wa_messages after the failed second import: %v", err)
	}
	if wa != 1 {
		t.Fatalf("wa_messages count after a failed second import = %d, want exactly 1 (unchanged from the first import — the failed second attempt's own INSERTs must have rolled back completely)", wa)
	}
}
