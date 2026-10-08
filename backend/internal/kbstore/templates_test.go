package kbstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

func queryBool(t *testing.T, db *dbx.DB, q string, args ...any) bool {
	t.Helper()
	var b bool
	if err := db.QueryRow(context.Background(), q, args...).Scan(&b); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return b
}

func TestPromptTemplates_FreshOrgListsFourDefaults(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	v, err := kb.ListPromptTemplates(context.Background(), orgID)
	if err != nil {
		t.Fatal(err)
	}
	if v.ActiveTemplateID != aiprompt.TemplateGeneral || v.KBConfigured || len(v.Templates) != 4 {
		t.Fatalf("view = active %q configured %v rows %d", v.ActiveTemplateID, v.KBConfigured, len(v.Templates))
	}
	for i, id := range aiprompt.PromptTemplateIDs() {
		want, _ := aiprompt.DefaultTemplateInstructions(id)
		if v.Templates[i].ID != id || v.Templates[i].Instructions != want || v.Templates[i].IsDefaultText || v.Templates[i].UpdatedAt.IsZero() {
			t.Errorf("row %d (%s) wrong: %+v", i, id, v.Templates[i].ID)
		}
	}
	active, err := kb.ActivePromptTemplate(context.Background(), orgID)
	if err != nil || active.ID != aiprompt.TemplateGeneral {
		t.Fatalf("ActivePromptTemplate = %+v, %v", active, err)
	}
}

func TestSavePromptTemplate_EditsInPlaceActivatesAndAudits(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()
	actor := uuid.Nil

	if err := kb.SavePromptTemplate(ctx, orgID, actor, "online-shop", "ПРАВИЛА-1", true); err != nil {
		t.Fatal(err)
	}
	v, _ := kb.ListPromptTemplates(ctx, orgID)
	if v.ActiveTemplateID != "online-shop" || v.Templates[1].Instructions != "ПРАВИЛА-1" {
		t.Fatalf("after save: active %q text %q", v.ActiveTemplateID, v.Templates[1].Instructions)
	}
	firstStamp := v.Templates[1].UpdatedAt

	// Re-saving identical text neither rewrites the row nor moves updated_at (it is
	// part of the prompt ref, so it must only move on a real edit).
	time.Sleep(5 * time.Millisecond)
	if err := kb.SavePromptTemplate(ctx, orgID, actor, "online-shop", "ПРАВИЛА-1", true); err != nil {
		t.Fatal(err)
	}
	if v, _ = kb.ListPromptTemplates(ctx, orgID); !v.Templates[1].UpdatedAt.Equal(firstStamp) {
		t.Error("unchanged text must not bump updated_at")
	}
	time.Sleep(5 * time.Millisecond)
	if err := kb.SavePromptTemplate(ctx, orgID, actor, "online-shop", "ПРАВИЛА-2", false); err != nil {
		t.Fatal(err)
	}
	if v, _ = kb.ListPromptTemplates(ctx, orgID); !v.Templates[1].UpdatedAt.After(firstStamp) || v.Templates[1].Instructions != "ПРАВИЛА-2" {
		t.Error("a real edit must update the row in place and move updated_at")
	}

	// Exactly one row per (org, id): edits are UPDATEs, never new rows.
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM ai_prompt_templates WHERE organization_id = $1`, orgID).Scan(&n); err != nil || n != 4 {
		t.Fatalf("template rows = %d (%v), want 4", n, err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM ai_audit_log WHERE organization_id = $1 AND note = 'prompt_template:online-shop'`, orgID).Scan(&n); err != nil || n != 2 {
		t.Errorf("audit rows for the two real edits = %d (%v), want 2", n, err)
	}
}

func TestSavePromptTemplate_RejectsBadInput(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SavePromptTemplate(ctx, orgID, uuid.Nil, "nope", "текст", true); !errors.Is(err, kbstore.ErrUnknownTemplate) {
		t.Errorf("unknown ID: %v", err)
	}
	for text, want := range map[string]error{
		"":                 aiprompt.ErrTemplateEmpty,
		"%%FACTS%%":        aiprompt.ErrTemplateReservedSyntax,
		"см. https://x.kz": aiprompt.ErrTemplateLeakShapedString,
		strings.Repeat("я", aiprompt.MaxTemplateRunes+1): aiprompt.ErrTemplateTooLong,
	} {
		if err := kb.SavePromptTemplate(ctx, orgID, uuid.Nil, "general", text, true); !errors.Is(err, want) {
			t.Errorf("%.20q: got %v, want %v", text, err, want)
		}
	}
	v, _ := kb.ListPromptTemplates(ctx, orgID)
	if want, _ := aiprompt.DefaultTemplateInstructions(aiprompt.TemplateGeneral); v.Templates[0].Instructions != want {
		t.Error("a rejected save changed the stored text")
	}
}

// Selecting a profile writes only the selection. Setup status belongs to settings.
func TestSavePromptTemplate_SelectionNeverChangesSetupStatus(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	// 1. No settings yet: selecting inserts a stub that is NOT configured.
	if err := kb.SavePromptTemplate(ctx, orgID, uuid.Nil, "service-business", "ПРАВИЛА", true); err != nil {
		t.Fatal(err)
	}
	if queryBool(t, db, `SELECT configured FROM ai_assistants WHERE organization_id = $1`, orgID) {
		t.Fatal("selecting a profile before configuration marked the assistant configured")
	}
	if v, _ := kb.ListPromptTemplates(ctx, orgID); v.KBConfigured || v.ActiveTemplateID != "service-business" {
		t.Fatalf("view: configured=%v active=%q", v.KBConfigured, v.ActiveTemplateID)
	}
	live, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatal(err)
	}
	if live.Config.Persona != "" || live.Config.ReplyMaxWords != 120 {
		t.Errorf("a stub row must read like no row: %+v", live.Config)
	}

	// The MCP existence check reads "configured" the same way: a stub is not a live assistant.
	assistantLive := func() bool {
		ids, err := kb.IdentityIndex(ctx, orgID, []string{kbstore.KBTypeAssistant}, "both", "")
		if err != nil || len(ids) != 1 {
			t.Fatalf("IdentityIndex: %v %+v", err, ids)
		}
		return ids[0].ExistsInLive
	}
	if assistantLive() {
		t.Fatal("MCP reports a live assistant for a profile-selection stub")
	}

	// 2. The first real settings save configures it and keeps the selection.
	persona := "Ты — ассистент."
	if err := kb.PatchLiveConfig(ctx, orgID, uuid.Nil, kbstore.ConfigPatch{Persona: &persona}); err != nil {
		t.Fatal(err)
	}
	if !queryBool(t, db, `SELECT configured FROM ai_assistants WHERE organization_id = $1`, orgID) || !assistantLive() {
		t.Fatal("a settings write must mark the assistant configured")
	}
	if v, _ := kb.ListPromptTemplates(ctx, orgID); !v.KBConfigured || v.ActiveTemplateID != "service-business" {
		t.Fatalf("after configuring: configured=%v active=%q", v.KBConfigured, v.ActiveTemplateID)
	}

	// 3. Switching on a configured org keeps it configured and keeps its settings.
	if err := kb.SavePromptTemplate(ctx, orgID, uuid.Nil, "online-service", "ПРАВИЛА-2", true); err != nil {
		t.Fatal(err)
	}
	if !queryBool(t, db, `SELECT configured FROM ai_assistants WHERE organization_id = $1`, orgID) {
		t.Fatal("switching profiles un-configured the assistant")
	}
	if live, _ = kb.LiveView(ctx, orgID); live.Config.Persona != persona {
		t.Errorf("switching profiles altered the settings: %+v", live.Config)
	}
}

func TestPromptTemplates_TenantIsolation(t *testing.T) {
	kb, orgA, st, _ := newTestKB(t)
	ctx := context.Background()
	b, err := st.SeedOrganization(ctx, "other-business")
	if err != nil {
		t.Fatal(err)
	}
	orgB := b.ID

	if err := kb.SavePromptTemplate(ctx, orgA, uuid.Nil, "general", "ТОЛЬКО-ДЛЯ-A", true); err != nil {
		t.Fatal(err)
	}
	if err := kb.SavePromptTemplate(ctx, orgA, uuid.Nil, "online-shop", "ТОЛЬКО-ДЛЯ-A-SHOP", true); err != nil {
		t.Fatal(err)
	}
	vb, _ := kb.ListPromptTemplates(ctx, orgB)
	if vb.ActiveTemplateID != aiprompt.TemplateGeneral {
		t.Errorf("org B's selection changed to %q", vb.ActiveTemplateID)
	}
	want, _ := aiprompt.DefaultTemplateInstructions(aiprompt.TemplateGeneral)
	if vb.Templates[0].Instructions != want {
		t.Error("org B sees org A's edit")
	}
	if err := kb.SavePromptTemplate(ctx, orgB, uuid.Nil, "general", "ТОЛЬКО-ДЛЯ-B", false); err != nil {
		t.Fatal(err)
	}
	va, _ := kb.ListPromptTemplates(ctx, orgA)
	if va.Templates[0].Instructions != "ТОЛЬКО-ДЛЯ-A" || va.ActiveTemplateID != "online-shop" {
		t.Errorf("org B's save leaked into org A: %q active=%q", va.Templates[0].Instructions, va.ActiveTemplateID)
	}
}

// An org whose rows were never seeded (created outside SeedOrganization) is still
// served — from the shipped defaults, in memory — and the first save creates the row.
func TestPromptTemplates_MissingRowsServedFromDefaults(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `DELETE FROM ai_prompt_templates WHERE organization_id = $1`, orgID); err != nil {
		t.Fatal(err)
	}
	v, err := kb.ListPromptTemplates(ctx, orgID)
	if err != nil || len(v.Templates) != 4 {
		t.Fatalf("list: %v rows=%d", err, len(v.Templates))
	}
	for _, r := range v.Templates {
		if !r.IsDefaultText || r.Instructions == "" {
			t.Errorf("%s: want the in-memory default flagged IsDefaultText", r.ID)
		}
	}
	if err := kb.SavePromptTemplate(ctx, orgID, uuid.Nil, "general", "ПЕРВАЯ-ПРАВКА", true); err != nil {
		t.Fatal(err)
	}
	v, _ = kb.ListPromptTemplates(ctx, orgID)
	if v.Templates[0].IsDefaultText || v.Templates[0].Instructions != "ПЕРВАЯ-ПРАВКА" || !v.Templates[1].IsDefaultText {
		t.Error("the first save must create exactly its own row")
	}
}
