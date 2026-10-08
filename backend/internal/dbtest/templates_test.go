package dbtest

import (
	"context"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/migrations"
)

const (
	templatesMigration = "20261008000000_ai_prompt_templates.sql"
	defaultOrgID       = "00000000-0000-0000-0000-000000000001"
	secondOrgID        = "77777777-7777-7777-7777-777777777777"
)

func scanString(t *testing.T, db *dbx.DB, query string, args ...any) string {
	t.Helper()
	var s string
	if err := db.QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return s
}

func scanInt(t *testing.T, db *dbx.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// The migration's seed text is generated from backend/aiprompt/templates; this
// pins the two together so the shipped defaults cannot drift from what a
// migrated database contains.
func TestPromptTemplateSeedMatchesShippedDefaults(t *testing.T) {
	sql, err := migrations.FS.ReadFile(templatesMigration)
	if err != nil {
		t.Fatal(err)
	}
	db := OpenRaw(t)
	for _, id := range aiprompt.PromptTemplateIDs() {
		text, ok := aiprompt.DefaultTemplateInstructions(id)
		if !ok {
			t.Fatalf("no default for %s", id)
		}
		if !strings.Contains(string(sql), "'"+strings.ReplaceAll(text, "'", "''")+"'") {
			t.Errorf("%s: the migration's seed literal differs from aiprompt/templates/%s-ru.txt — regenerate the migration", id, id)
		}
		got := scanString(t, db, `SELECT instructions FROM ai_prompt_templates WHERE organization_id = $1 AND id = $2`, defaultOrgID, id)
		if got != text {
			t.Errorf("%s: the migrated row differs from the shipped default", id)
		}
	}
}

func TestFreshDatabaseHasTemplatesAndAssistantColumns(t *testing.T) {
	db := OpenRaw(t)
	if n := scanInt(t, db, `SELECT count(*) FROM ai_prompt_templates WHERE organization_id = $1`, defaultOrgID); n != 4 {
		t.Fatalf("default org has %d template rows, want 4", n)
	}
	// The new columns exist with their defaults: a bare insert is a configured,
	// General assistant.
	ctx := context.Background()
	mustExec(t, db, ctx, `INSERT INTO ai_assistants (id, organization_id, created_at, updated_at) VALUES ('a-1', $1, 1, 1)`, defaultOrgID)
	if got := scanString(t, db, `SELECT prompt_template_id FROM ai_assistants WHERE organization_id = $1`, defaultOrgID); got != "general" {
		t.Errorf("default prompt_template_id = %q, want general", got)
	}
	if n := scanInt(t, db, `SELECT count(*) FROM ai_assistants WHERE organization_id = $1 AND configured`, defaultOrgID); n != 1 {
		t.Error("a row written without the new column must read as configured")
	}
}

// A forced replay (dev workflow) and any redeploy must keep: every edited
// template, each organization's selected profile, and its setup status.
func TestReplayPreservesTemplatesSelectionAndSetupStatus(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()

	// Org 1: configured, online-shop selected, two edited templates.
	mustExec(t, db, ctx, `INSERT INTO ai_assistants (id, organization_id, persona, created_at, updated_at, prompt_template_id, configured)
		VALUES ('a-1', $1, 'Персона', 1, 1, 'online-shop', TRUE)`, defaultOrgID)
	mustExec(t, db, ctx, `UPDATE ai_prompt_templates SET instructions = 'ПРАВКА-SHOP' WHERE organization_id = $1 AND id = 'online-shop'`, defaultOrgID)
	mustExec(t, db, ctx, `UPDATE ai_prompt_templates SET instructions = 'ПРАВКА-GENERAL', updated_at = 42 WHERE organization_id = $1 AND id = 'general'`, defaultOrgID)
	// Org 2: created after the migration; a profile stub that is NOT configured.
	mustExec(t, db, ctx, `INSERT INTO organizations (id, name, created_at, updated_at) VALUES ($1, 'second', 1, 1)`, secondOrgID)
	mustExec(t, db, ctx, `INSERT INTO ai_assistants (id, organization_id, created_at, updated_at, prompt_template_id, configured)
		VALUES ('a-2', $1, 1, 1, 'service-business', FALSE)`, secondOrgID)

	for i := 0; i < 2; i++ { // replay twice: it must be a fixpoint
		if err := dbx.RunMigrationsWithOptions(ctx, db, migrations.FS, dbx.MigrationOptions{Force: "all"}); err != nil {
			t.Fatalf("replay %d: %v", i+1, err)
		}
	}

	type assistant struct {
		template string
		cfg      bool
		persona  string
	}
	read := func(org string) assistant {
		var a assistant
		if err := db.QueryRow(ctx, `SELECT prompt_template_id, configured, persona FROM ai_assistants WHERE organization_id = $1`, org).
			Scan(&a.template, &a.cfg, &a.persona); err != nil {
			t.Fatalf("read assistant %s: %v", org, err)
		}
		return a
	}
	if a := read(defaultOrgID); a != (assistant{"online-shop", true, "Персона"}) {
		t.Errorf("org 1 assistant after replay = %+v", a)
	}
	if a := read(secondOrgID); a.template != "service-business" || a.cfg {
		t.Errorf("org 2 stub after replay = %+v, want service-business and NOT configured", a)
	}
	if n := scanInt(t, db, `SELECT count(*) FROM ai_assistants`); n != 2 {
		t.Errorf("ai_assistants has %d rows after replay, want 2 (no duplicates)", n)
	}
	if got := scanString(t, db, `SELECT instructions FROM ai_prompt_templates WHERE organization_id = $1 AND id = 'online-shop'`, defaultOrgID); got != "ПРАВКА-SHOP" {
		t.Errorf("replay overwrote an edited template: %q", got)
	}
	if got := scanString(t, db, `SELECT instructions FROM ai_prompt_templates WHERE organization_id = $1 AND id = 'general'`, defaultOrgID); got != "ПРАВКА-GENERAL" {
		t.Errorf("replay overwrote an edited template: %q", got)
	}
	if n := scanInt(t, db, `SELECT updated_at FROM ai_prompt_templates WHERE organization_id = $1 AND id = 'general'`, defaultOrgID); n != 42 {
		t.Errorf("replay touched an edited row's updated_at (%d)", n)
	}
	// Both organizations end up with exactly their four rows: the replay seeds the
	// one created after the migration, and duplicates nothing.
	for _, org := range []string{defaultOrgID, secondOrgID} {
		if n := scanInt(t, db, `SELECT count(*) FROM ai_prompt_templates WHERE organization_id = $1`, org); n != 4 {
			t.Errorf("org %s has %d template rows, want 4", org, n)
		}
	}
	// The primary key really is tenant-scoped uniqueness.
	if _, err := db.Exec(ctx, `INSERT INTO ai_prompt_templates (organization_id, id, instructions, created_at, updated_at)
		VALUES ($1, 'general', 'dup', 1, 1)`, defaultOrgID); err == nil {
		t.Error("a duplicate (organization_id, id) must be rejected")
	}
}

// SeedOrganization gives a new business its four templates, and calling it again
// (every boot) neither duplicates nor overwrites an operator's edit.
func TestSeedOrganizationSeedsTemplatesWithoutOverwriting(t *testing.T) {
	st, db := Open(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "brand-new-business")
	if err != nil {
		t.Fatal(err)
	}
	if n := scanInt(t, db, `SELECT count(*) FROM ai_prompt_templates WHERE organization_id = $1`, org.ID); n != 4 {
		t.Fatalf("new business has %d templates, want 4", n)
	}
	mustExec(t, db, ctx, `UPDATE ai_prompt_templates SET instructions = 'ПРАВКА' WHERE organization_id = $1 AND id = 'online-service'`, org.ID)
	mustExec(t, db, ctx, `DELETE FROM ai_prompt_templates WHERE organization_id = $1 AND id = 'general'`, org.ID)

	again, err := st.SeedOrganization(ctx, "brand-new-business")
	if err != nil || again.ID != org.ID {
		t.Fatalf("second SeedOrganization: %v id=%v", err, again.ID)
	}
	if got := scanString(t, db, `SELECT instructions FROM ai_prompt_templates WHERE organization_id = $1 AND id = 'online-service'`, org.ID); got != "ПРАВКА" {
		t.Errorf("re-seeding overwrote an edit: %q", got)
	}
	if n := scanInt(t, db, `SELECT count(*) FROM ai_prompt_templates WHERE organization_id = $1`, org.ID); n != 4 {
		t.Errorf("after re-seeding there are %d rows, want 4 (the deleted one restored, nothing duplicated)", n)
	}
	want, _ := aiprompt.DefaultTemplateInstructions(aiprompt.TemplateGeneral)
	if got := scanString(t, db, `SELECT instructions FROM ai_prompt_templates WHERE organization_id = $1 AND id = 'general'`, org.ID); got != want {
		t.Error("a missing row must be re-seeded from the shipped default")
	}
}
