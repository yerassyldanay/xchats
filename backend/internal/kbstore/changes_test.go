package kbstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/brain"
	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// TestSingularDeleteKind_TableDriven guards against a future entity kind
// silently inheriting a fragile strings.TrimSuffix(kind, "s") — the exact bug
// this table replaces, which turned "policies" into "policie".
func TestSingularDeleteKind_TableDriven(t *testing.T) {
	cases := []struct {
		kind     string
		singular string
		ok       bool
	}{
		{"topics", "topic", true},
		{"tariffs", "tariff", true},
		{"products", "product", true},
		{"contacts", "contact", true},
		{"policies", "policy", true},
		{"delivery_zones", "delivery_zone", true},
		{"config", "", false},
		{"", "", false},
		{"bogus", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			got, ok := kbstore.SingularDeleteKind(tc.kind)
			if ok != tc.ok {
				t.Fatalf("SingularDeleteKind(%q) ok = %v, want %v", tc.kind, ok, tc.ok)
			}
			if ok && got != tc.singular {
				t.Fatalf("SingularDeleteKind(%q) = %q, want %q", tc.kind, got, tc.singular)
			}
		})
	}
}

func TestPluralChangeKind_IsSingularDeleteKindsInverse(t *testing.T) {
	for plural, singular := range map[string]string{
		"topics": "topic", "tariffs": "tariff", "products": "product",
		"contacts": "contact", "policies": "policy", "delivery_zones": "delivery_zone",
	} {
		if got := kbstore.PluralChangeKind(singular); got != plural {
			t.Fatalf("PluralChangeKind(%q) = %q, want %q", singular, got, plural)
		}
	}
}

func TestIsSingletonKind(t *testing.T) {
	for singular, want := range map[string]bool{
		"contact": true, "policy": true,
		"topic": false, "tariff": false, "product": false, "delivery_zone": false,
	} {
		if got := kbstore.IsSingletonKind(singular); got != want {
			t.Fatalf("IsSingletonKind(%q) = %v, want %v", singular, got, want)
		}
	}
}

// --- DraftChanges ------------------------------------------------------------

func TestDraftChanges_EmptyDraftHasNoEntries(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("DraftChanges: %v", err)
	}
	if changes.Config != nil {
		t.Fatalf("Config = %+v, want nil", changes.Config)
	}
	if len(changes.Topics) != 0 || len(changes.Tariffs) != 0 || len(changes.Products) != 0 ||
		len(changes.Contacts) != 0 || len(changes.Policies) != 0 || len(changes.Zones) != 0 || len(changes.Deletes) != 0 {
		t.Fatalf("expected a fully empty change set, got %+v", changes)
	}
	if changes.BaseVersion != 0 {
		t.Fatalf("BaseVersion = %d, want 0 for an org with no draft activity", changes.BaseVersion)
	}
}

// A published-but-untouched KB must never leak into the review payload: only
// what is actually staged appears, regardless of how much live data exists.
func TestDraftChanges_ReturnsOnlyPendingRowsNeverLive(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	seed := brain.SeedSnapshot()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	live, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view: %v", err)
	}
	if len(live.Topics) == 0 || len(live.Tariffs) == 0 {
		t.Fatal("seed produced no live topics/tariffs to distinguish from the draft")
	}

	if err := kb.UpsertTariff(ctx, orgID, uuid.Nil, kbstore.TariffInput{Ref: "brand-new", Name: "Новый тариф", Price: "1000"}); err != nil {
		t.Fatalf("upsert tariff: %v", err)
	}

	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("DraftChanges: %v", err)
	}
	if len(changes.Tariffs) != 1 || changes.Tariffs[0].Ref != "brand-new" {
		t.Fatalf("Tariffs = %+v, want exactly the one staged tariff", changes.Tariffs)
	}
	if len(changes.Topics) != 0 {
		t.Fatalf("Topics = %+v, want none — no topic was ever staged, only seeded live", changes.Topics)
	}
	if len(changes.Products) != 0 || len(changes.Contacts) != 0 || len(changes.Policies) != 0 || len(changes.Zones) != 0 {
		t.Fatalf("every other kind should be empty, got %+v", changes)
	}
}

func TestDraftChanges_SurfacesDeleteMarkersInPluralKind(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	live, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view: %v", err)
	}
	if len(live.Tariffs) == 0 {
		t.Fatal("seed has no live tariff to delete")
	}
	ref := live.Tariffs[0].Ref
	if err := kb.DeleteTariff(ctx, orgID, uuid.Nil, ref); err != nil {
		t.Fatalf("delete tariff: %v", err)
	}
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("DraftChanges: %v", err)
	}
	if len(changes.Deletes) != 1 {
		t.Fatalf("Deletes = %+v, want exactly one marker", changes.Deletes)
	}
	if changes.Deletes[0].Kind != "tariffs" || changes.Deletes[0].Key != ref {
		t.Fatalf("Deletes[0] = %+v, want {tariffs %s} — the PLURAL kind vocabulary", changes.Deletes[0], ref)
	}
}

// A singleton delete marker's key must always come back as the canonical
// natural-key slug (domain.ContactSlug), regardless of which spelling
// actually wrote it (MCPDelete writes NaturalKeyMain, "main") — so the
// frontend can address the SAME card by (kind,key) for its upsert row, its
// Publish call, and its Cancel call, without special-casing singletons.
func TestDraftChanges_SingletonDeleteKeyIsCanonicalSlug(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypeContacts, kbstore.NaturalKeyMain, nil); err != nil {
		t.Fatalf("mcp delete contacts: %v", err)
	}
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("DraftChanges: %v", err)
	}
	if len(changes.Deletes) != 1 {
		t.Fatalf("Deletes = %+v, want exactly one marker", changes.Deletes)
	}
	if changes.Deletes[0].Kind != "contacts" || changes.Deletes[0].Key != domain.ContactSlug {
		t.Fatalf("Deletes[0] = %+v, want {contacts %s} even though MCPDelete wrote key %q", changes.Deletes[0], domain.ContactSlug, kbstore.NaturalKeyMain)
	}
}

func TestDraftChanges_ConfigIsNilWithoutAPendingPatch(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("DraftChanges: %v", err)
	}
	if changes.Config != nil {
		t.Fatalf("Config = %+v, want nil — nothing was ever patched", changes.Config)
	}
}

func TestDraftChanges_ConfigCarriesOnlyPatchedFields(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	persona := "Дружелюбный ассистент"
	if err := kb.PatchConfig(ctx, orgID, uuid.Nil, kbstore.ConfigPatch{Persona: &persona}); err != nil {
		t.Fatalf("patch config: %v", err)
	}
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("DraftChanges: %v", err)
	}
	if changes.Config == nil {
		t.Fatal("Config = nil, want a pending patch")
	}
	if changes.Config.Persona == nil || *changes.Config.Persona != persona {
		t.Fatalf("Config.Persona = %v, want %q", changes.Config.Persona, persona)
	}
	if changes.Config.Mission != nil || changes.Config.Guardrails != nil ||
		changes.Config.LanguagePolicy != nil || changes.Config.ReplyMaxWords != nil {
		t.Fatalf("Config = %+v, want only Persona set", changes.Config)
	}
}

// --- CancelChange ------------------------------------------------------------

func TestCancelChange_CancelsAnAddition(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "new-topic", Title: "T", BodyMD: "Body."}); err != nil {
		t.Fatalf("upsert topic: %v", err)
	}
	result, err := kb.CancelChange(ctx, orgID, uuid.Nil, "topics", "new-topic", nil)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if !result.Changed {
		t.Fatal("Changed = false, want true — the addition existed")
	}
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("DraftChanges: %v", err)
	}
	if len(changes.Topics) != 0 {
		t.Fatalf("Topics = %+v, want none after cancelling the only addition", changes.Topics)
	}
}

func TestCancelChange_CancelsAnUpdateLeavingLiveIntact(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	live, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view: %v", err)
	}
	if len(live.Topics) == 0 {
		t.Fatal("seed has no live topic to edit")
	}
	slug, originalTitle := live.Topics[0].Slug, live.Topics[0].Title
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: slug, Title: "Изменённый заголовок", BodyMD: live.Topics[0].BodyMD}); err != nil {
		t.Fatalf("upsert topic: %v", err)
	}

	result, err := kb.CancelChange(ctx, orgID, uuid.Nil, "topics", slug, nil)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if !result.Changed {
		t.Fatal("Changed = false, want true")
	}
	live2, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view: %v", err)
	}
	for _, tp := range live2.Topics {
		if tp.Slug == slug && tp.Title != originalTitle {
			t.Fatalf("live topic %q title = %q, want unchanged %q", slug, tp.Title, originalTitle)
		}
	}
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("DraftChanges: %v", err)
	}
	for _, tp := range changes.Topics {
		if tp.Slug == slug {
			t.Fatalf("topic %q still pending after cancel: %+v", slug, tp)
		}
	}
}

func TestCancelChange_CancelsAStagedRemoval(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	live, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view: %v", err)
	}
	if len(live.Tariffs) == 0 {
		t.Fatal("seed has no live tariff to delete")
	}
	ref := live.Tariffs[0].Ref
	if err := kb.DeleteTariff(ctx, orgID, uuid.Nil, ref); err != nil {
		t.Fatalf("delete tariff: %v", err)
	}

	result, err := kb.CancelChange(ctx, orgID, uuid.Nil, "tariffs", ref, nil)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if !result.Changed {
		t.Fatal("Changed = false, want true — a staged removal existed")
	}
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("DraftChanges: %v", err)
	}
	if len(changes.Deletes) != 0 {
		t.Fatalf("Deletes = %+v, want none after cancel", changes.Deletes)
	}
	live2, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view: %v", err)
	}
	found := false
	for _, tr := range live2.Tariffs {
		if tr.Ref == ref {
			found = true
		}
	}
	if !found {
		t.Fatalf("live tariff %q vanished — CancelChange must never touch live tables", ref)
	}
}

func TestCancelChange_ConfigSingleFieldLeavesOtherFields(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	persona, mission := "Persona", "Mission"
	if err := kb.PatchConfig(ctx, orgID, uuid.Nil, kbstore.ConfigPatch{Persona: &persona, Mission: &mission}); err != nil {
		t.Fatalf("patch config: %v", err)
	}
	result, err := kb.CancelChange(ctx, orgID, uuid.Nil, "config", "persona", nil)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if !result.Changed {
		t.Fatal("Changed = false, want true")
	}
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("DraftChanges: %v", err)
	}
	if changes.Config == nil {
		t.Fatal("Config = nil, want mission still pending")
	}
	if changes.Config.Persona != nil {
		t.Fatalf("Persona = %v, want nil after cancelling it", changes.Config.Persona)
	}
	if changes.Config.Mission == nil || *changes.Config.Mission != mission {
		t.Fatalf("Mission = %v, want %q (untouched)", changes.Config.Mission, mission)
	}
}

func TestCancelChange_ConfigMainClearsWholePatch(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	persona, mission := "Persona", "Mission"
	if err := kb.PatchConfig(ctx, orgID, uuid.Nil, kbstore.ConfigPatch{Persona: &persona, Mission: &mission}); err != nil {
		t.Fatalf("patch config: %v", err)
	}
	result, err := kb.CancelChange(ctx, orgID, uuid.Nil, "config", kbstore.NaturalKeyMain, nil)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if !result.Changed {
		t.Fatal("Changed = false, want true")
	}
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("DraftChanges: %v", err)
	}
	if changes.Config != nil {
		t.Fatalf("Config = %+v, want nil — the whole patch was cancelled", changes.Config)
	}
}

func TestCancelChange_UnknownKindIsErrUnknownKind(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	_, err := kb.CancelChange(ctx, orgID, uuid.Nil, "bogus", "whatever", nil)
	if !errors.Is(err, kbstore.ErrUnknownKind) {
		t.Fatalf("err = %v, want ErrUnknownKind", err)
	}
}

func TestCancelChange_UnknownConfigFieldIsErrUnknownKind(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	_, err := kb.CancelChange(ctx, orgID, uuid.Nil, "config", "not_a_field", nil)
	if !errors.Is(err, kbstore.ErrUnknownKind) {
		t.Fatalf("err = %v, want ErrUnknownKind", err)
	}
}

// A repeated cancel must be free: the second call is a genuine no-op — it
// must not advance base_version, or a client retry after a lost response
// would stale every other open tab.
func TestCancelChange_RepeatDoesNotAdvanceVersion(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "t", Title: "T", BodyMD: "Body."}); err != nil {
		t.Fatalf("upsert topic: %v", err)
	}
	first, err := kb.CancelChange(ctx, orgID, uuid.Nil, "topics", "t", nil)
	if err != nil {
		t.Fatalf("first cancel: %v", err)
	}
	if !first.Changed {
		t.Fatal("first cancel: Changed = false, want true")
	}
	second, err := kb.CancelChange(ctx, orgID, uuid.Nil, "topics", "t", nil)
	if err != nil {
		t.Fatalf("second cancel: %v", err)
	}
	if second.Changed {
		t.Fatal("second cancel: Changed = true, want false — nothing left to cancel")
	}
	if second.BaseVersion != first.BaseVersion {
		t.Fatalf("second cancel BaseVersion = %d, want unchanged %d", second.BaseVersion, first.BaseVersion)
	}
}

// Cancelling an already-absent target is not a conflict — there is no
// payload to clobber — so a stale If-Match must not turn it into an error.
func TestCancelChange_AlreadyAbsentIgnoresStaleIfMatch(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	wildlyStale := int64(999)
	result, err := kb.CancelChange(ctx, orgID, uuid.Nil, "topics", "never-existed", &wildlyStale)
	if err != nil {
		t.Fatalf("err = %v, want nil — an absent target short-circuits before the version check", err)
	}
	if result.Changed {
		t.Fatal("Changed = true, want false")
	}
}

// A PRESENT target with a stale If-Match is a genuine conflict and must
// surface as ErrStale — only the already-absent case is exempt.
func TestCancelChange_PresentTargetWithStaleVersionIsErrStale(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "t", Title: "T", BodyMD: "Body."}); err != nil {
		t.Fatalf("upsert topic: %v", err)
	}
	current, err := kb.DraftBaseVersion(ctx, orgID)
	if err != nil {
		t.Fatalf("base version: %v", err)
	}
	stale := current - 1
	_, err = kb.CancelChange(ctx, orgID, uuid.Nil, "topics", "t", &stale)
	if !errors.Is(err, kbstore.ErrStale) {
		t.Fatalf("err = %v, want ErrStale", err)
	}
}

// Cancelling a key that was never staged must never lazily create an empty
// kbd_draft row — there is nothing to persist.
func TestCancelChange_UnknownKeyDoesNotCreateAnEmptyDraftRow(t *testing.T) {
	kb, orgID, st := newTestKB(t)
	ctx := context.Background()
	result, err := kb.CancelChange(ctx, orgID, uuid.Nil, "topics", "never-existed", nil)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if result.Changed {
		t.Fatal("Changed = true, want false")
	}
	if result.BaseVersion != 0 {
		t.Fatalf("BaseVersion = %d, want 0 (no draft activity ever happened)", result.BaseVersion)
	}
	var count int
	if err := st.Pool().QueryRow(ctx, `SELECT count(*) FROM xchats.kbd_draft WHERE organization_id = $1`, orgID).Scan(&count); err != nil {
		t.Fatalf("count kbd_draft rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("kbd_draft rows = %d, want 0 — cancelling an unknown key must not create one", count)
	}
}

// --- Regressions: the "policie" bug + singleton key-spelling mismatches ----

// Verified bug: strings.TrimSuffix("policies", "s") == "policie", so a staged
// policy deletion never matched its own delete marker at approve time — the
// approve silently no-op'd and the live policy row survived.
func TestApproveEntity_PoliciesDeleteMarkerApplies(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	live, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view: %v", err)
	}
	if len(live.Policies) == 0 {
		t.Fatal("seed has no live policy to delete")
	}

	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypePolicies, kbstore.NaturalKeyMain, nil); err != nil {
		t.Fatalf("mcp delete policies: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{Kind: "policies", Key: domain.PolicySlug}); err != nil {
		t.Fatalf("approve policies: %v", err)
	}

	live2, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view after approve: %v", err)
	}
	if len(live2.Policies) != 0 {
		t.Fatalf("live still has %d polic(y/ies) after approving its deletion — the marker never applied", len(live2.Policies))
	}
}

// Verified bug: MCPDelete always writes the contact delete marker keyed
// "main" (kbstore.NaturalKeyMain); the approve route's caller addresses the
// same contact by domain.ContactSlug ("support"). Before deleteMatches
// ignored key for singleton kinds, those two spellings never matched.
func TestApproveEntity_ContactsDeleteMarkerWrittenByMCPKeyApplies(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	live, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view: %v", err)
	}
	if len(live.Contacts) == 0 {
		t.Fatal("seed has no live contact to delete")
	}

	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypeContacts, kbstore.NaturalKeyMain, nil); err != nil {
		t.Fatalf("mcp delete contacts: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{Kind: "contacts", Key: domain.ContactSlug}); err != nil {
		t.Fatalf("approve contacts: %v", err)
	}

	live2, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view after approve: %v", err)
	}
	if len(live2.Contacts) != 0 {
		t.Fatalf("live still has %d contact(s) after approving its deletion — the marker never applied", len(live2.Contacts))
	}
}

// Verified bug: approveNote's audit-log summary said "approved policie main".
func TestApproveSummary_SaysPolicyNotPolicie(t *testing.T) {
	kb, orgID, st := newTestKB(t)
	ctx := context.Background()
	cost := "500"
	if err := kb.PatchPolicies(ctx, orgID, uuid.Nil, kbstore.PolicyPatch{DeliveryCost: &cost}); err != nil {
		t.Fatalf("patch policies: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{Kind: "policies", Key: domain.PolicySlug}); err != nil {
		t.Fatalf("approve policies: %v", err)
	}
	var note string
	if err := st.Pool().QueryRow(ctx, `SELECT note FROM xchats.ai_audit_log WHERE organization_id = $1 AND action = 'approve' ORDER BY created_at DESC LIMIT 1`, orgID).Scan(&note); err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if strings.Contains(note, "policie ") || strings.HasSuffix(note, "policie") {
		t.Fatalf("audit note = %q, contains the fragile TrimSuffix artifact %q", note, "policie")
	}
	if !strings.Contains(note, "policy") {
		t.Fatalf("audit note = %q, want it to say \"policy\"", note)
	}
}

// The draftOnly/DraftOnly extraction (draftRowsFromBlob) must not change
// what kb_read(source=draft) and the MCP KB Manager widget observe: every
// kind still projects exactly its pending entries, config is Draft:true only
// when something was patched, and deleted entries are suppressed —
// including the singleton bugfix now folded into the SAME shared code path.
func TestDraftOnly_ContractUnchangedAfterRefactor(t *testing.T) {
	kb, orgID, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "new-topic", Title: "New", BodyMD: "Body."}); err != nil {
		t.Fatalf("upsert topic: %v", err)
	}
	persona := "P"
	if err := kb.PatchConfig(ctx, orgID, uuid.Nil, kbstore.ConfigPatch{Persona: &persona}); err != nil {
		t.Fatalf("patch config: %v", err)
	}
	live, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("live view: %v", err)
	}
	if len(live.Tariffs) == 0 {
		t.Fatal("seed has no live tariff to delete")
	}
	ref := live.Tariffs[0].Ref
	if err := kb.DeleteTariff(ctx, orgID, uuid.Nil, ref); err != nil {
		t.Fatalf("delete tariff: %v", err)
	}
	// A singleton delete marker written under a DIFFERENT key spelling than
	// domain.ContactSlug — the exact scenario the singleton bugfix covers —
	// must also stay suppressed through the refactored draftOnly.
	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypeContacts, kbstore.NaturalKeyMain, nil); err != nil {
		t.Fatalf("mcp delete contacts: %v", err)
	}

	draft, err := kb.DraftOnly(ctx, orgID)
	if err != nil {
		t.Fatalf("DraftOnly: %v", err)
	}
	if len(draft.Topics) != 1 || draft.Topics[0].Slug != "new-topic" || !draft.Topics[0].Draft {
		t.Fatalf("Topics = %+v, want exactly the one staged topic, Draft:true", draft.Topics)
	}
	if !draft.Config.Draft || draft.Config.Persona != persona {
		t.Fatalf("Config = %+v, want Draft:true Persona:%q", draft.Config, persona)
	}
	for _, tr := range draft.Tariffs {
		if tr.Ref == ref {
			t.Fatalf("tariff %q staged for deletion must not appear in DraftOnly", ref)
		}
	}
	if len(draft.Contacts) != 0 {
		t.Fatalf("Contacts = %+v, want none — the pending contact delete (written with a different key spelling) must be suppressed", draft.Contacts)
	}
	// Every collection must be a non-nil empty slice, never JSON null.
	if draft.Products == nil || draft.Zones == nil || draft.Policies == nil {
		t.Fatalf("collections must default to non-nil empty slices, got products=%v zones=%v policies=%v", draft.Products, draft.Zones, draft.Policies)
	}
}
