package dbtest

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	sqlitemigrations "github.com/yerassyldanay/xchats/backend/migrations/sqlite"
)

// whatsmeowMigration is the forward migration that replaced the Evolution
// gateway. Everything lexically before it is the "pre-whatsmeow" schema an
// already-deployed database is sitting on.
const whatsmeowMigration = "0007_whatsmeow.up.sql"

// preWhatsmeowFS returns the migration set as it existed before the
// whatsmeow cutover, so a test can stand up a database in the exact state a
// deployed one was in and then migrate it forward for real.
func preWhatsmeowFS(t testing.TB) fs.FS {
	t.Helper()
	entries, err := fs.ReadDir(sqlitemigrations.FS, ".")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}
	prior := fstest.MapFS{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") || name >= whatsmeowMigration {
			continue
		}
		body, err := fs.ReadFile(sqlitemigrations.FS, name)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		prior[name] = &fstest.MapFile{Data: body}
	}
	if len(prior) == 0 {
		t.Fatal("no pre-whatsmeow migrations found — did the naming scheme change?")
	}
	return prior
}

// TestWhatsmeowMigrationUpgradesADeployedDatabase is the regression guard for
// a bug that shipped: the whatsmeow schema changes were first written as an
// in-place edit to 0002_channels.up.sql. Fresh databases got them, but
// dbx.RunMigrations records applied versions and never re-runs one, so every
// already-deployed database silently kept the Evolution-era schema — and the
// app then failed at runtime with "no such table: wa_credentials".
//
// This test refuses to let that class of bug return: it migrates a database
// to the pre-whatsmeow schema, fills it with Evolution-era rows, and asserts
// that running the current migration set upgrades it in place WITH ITS DATA
// INTACT. A future in-place edit to an already-shipped migration fails here.
func TestWhatsmeowMigrationUpgradesADeployedDatabase(t *testing.T) {
	ctx := context.Background()
	db, err := dbx.Open(ctx, filepath.Join(t.TempDir(), "xchats.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})

	// 1. A database as deployed before the cutover.
	if err := dbx.RunMigrations(ctx, db, preWhatsmeowFS(t)); err != nil {
		t.Fatalf("apply pre-whatsmeow migrations: %v", err)
	}
	if hasTable(t, db, "wa_credentials") {
		t.Fatal("setup is wrong: wa_credentials must not exist before the whatsmeow migration")
	}

	// 2. Evolution-era rows, including the columns the migration rewrites.
	// A child chat/contact/message chain is seeded deliberately: SQLite's
	// RENAME COLUMN / DROP COLUMN rewrite the table's schema, and this
	// proves they do so without disturbing foreign keys or row data.
	mustExec(t, db, ctx, `INSERT INTO organizations (id, name) VALUES ('org-1', 'xchats')`)
	mustExec(t, db, ctx, `INSERT INTO wa_accounts (id, organization_id, owner_jid, phone_number, evolution_instance_name, evolution_instance_id)
	             VALUES ('acct-1', 'org-1', '77011111111@s.whatsapp.net', '77011111111', 'xpayment', 'inst-abc')`)
	mustExec(t, db, ctx, `INSERT INTO wa_contacts (id, account_id, phone_jid, phone_number)
	             VALUES ('contact-1', 'acct-1', '77000000000@s.whatsapp.net', '77000000000')`)
	mustExec(t, db, ctx, `INSERT INTO wa_chats (id, account_id, contact_id, remote_jid)
	             VALUES ('chat-1', 'acct-1', 'contact-1', '77000000000@s.whatsapp.net')`)
	mustExec(t, db, ctx, `INSERT INTO wa_messages (id, account_id, chat_id, direction, sender_kind, evolution_message_id, body)
	             VALUES ('msg-1', 'acct-1', 'chat-1', 'in', 'contact', 'EVO-MSG-1', 'привет')`)

	// 3. The upgrade under test: the real runner, the real migration set.
	if err := dbx.RunMigrations(ctx, db, sqlitemigrations.FS); err != nil {
		t.Fatalf("upgrade an already-migrated database: %v", err)
	}

	// 4. The new schema is present...
	if !hasTable(t, db, "wa_credentials") {
		t.Fatal("wa_credentials missing after the upgrade — this is the shipped bug")
	}
	mustExec(t, db, ctx, `INSERT INTO wa_credentials (account_id, device_jid) VALUES ('acct-1', '77011111111:38@s.whatsapp.net')`)

	for _, col := range []string{"evolution_instance_name", "evolution_instance_id"} {
		if hasColumn(t, db, "wa_accounts", col) {
			t.Errorf("wa_accounts.%s should have been dropped", col)
		}
	}
	if hasColumn(t, db, "wa_messages", "evolution_message_id") {
		t.Error("wa_messages.evolution_message_id should have been renamed away")
	}

	// 5. ...and the pre-existing data came with it, under the new name.
	var body, extID string
	if err := db.QueryRow(ctx,
		`SELECT body, external_message_id FROM wa_messages WHERE id = 'msg-1'`).Scan(&body, &extID); err != nil {
		t.Fatalf("read migrated message: %v", err)
	}
	if body != "привет" || extID != "EVO-MSG-1" {
		t.Errorf("message survived as (%q, %q), want (%q, %q)", body, extID, "привет", "EVO-MSG-1")
	}

	// The echo-collapse guarantee rides on this UNIQUE constraint; RENAME
	// COLUMN must have carried it across to the new column name.
	if _, err := db.Exec(ctx,
		`INSERT INTO wa_messages (id, account_id, chat_id, direction, sender_kind, external_message_id)
		 VALUES ('msg-dup', 'acct-1', 'chat-1', 'in', 'contact', 'EVO-MSG-1')`); err == nil {
		t.Error("UNIQUE (account_id, external_message_id) was lost in the rename — duplicate insert succeeded")
	}

	// 6. The rebuilt views read the renamed column and dropped the instance.
	var viewExtID string
	if err := db.QueryRow(ctx,
		`SELECT external_message_id FROM inbox_messages_v WHERE id = 'msg-1'`).Scan(&viewExtID); err != nil {
		t.Fatalf("read inbox_messages_v: %v", err)
	}
	if viewExtID != "EVO-MSG-1" {
		t.Errorf("inbox_messages_v.external_message_id = %q, want EVO-MSG-1", viewExtID)
	}
	var handle string
	if err := db.QueryRow(ctx,
		`SELECT external_handle FROM inbox_accounts_v WHERE id = 'acct-1'`).Scan(&handle); err != nil {
		t.Fatalf("read inbox_accounts_v: %v", err)
	}
	if handle != "77011111111" {
		t.Errorf("inbox_accounts_v.external_handle = %q", handle)
	}

	// 7. Re-running the whole set stays a no-op (the runner's core promise).
	if err := reapplyMigrations(t, db); err != nil {
		t.Fatalf("re-running migrations after the upgrade must be a no-op: %v", err)
	}
}

func hasTable(t testing.TB, db *dbx.DB, name string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(context.Background(),
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = $1`, name).Scan(&n); err != nil {
		t.Fatalf("look up table %s: %v", name, err)
	}
	return n > 0
}

func hasColumn(t testing.TB, db *dbx.DB, table, column string) bool {
	t.Helper()
	rows, err := db.Query(context.Background(), `SELECT name FROM pragma_table_info($1)`, table)
	if err != nil {
		t.Fatalf("pragma_table_info(%s): %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan column name: %v", err)
		}
		if name == column {
			return true
		}
	}
	return false
}
