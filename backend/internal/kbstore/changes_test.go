package kbstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/brain"
	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// TestSingularDeleteKind_TableDriven guards the entity-kind vocabulary table
// itself — every kind used anywhere in the Playground/MCP surface, so a
// future entity can never silently inherit fragile string manipulation
// (the previous strings.TrimSuffix(kind, "s") turned "policies" into
// "policie" — see the two regression tests below).
func TestSingularDeleteKind_TableDriven(t *testing.T) {
	cases := []struct {
		kind         string
		wantSingular string
		wantOK       bool
	}{
		{"topics", "topic", true},
		{"tariffs", "tariff", true},
		{"products", "product", true},
		{"contacts", "contact", true},
		{"policies", "policy", true},
		{"delivery_zones", "delivery_zone", true},
		{"config", "", false}, // never deletable — no DraftDelete.Kind of its own
		{"unknown", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			got, ok := kbstore.SingularDeleteKind(tc.kind)
			if ok != tc.wantOK {
				t.Fatalf("SingularDeleteKind(%q) ok = %v, want %v", tc.kind, ok, tc.wantOK)
			}
			if ok && got != tc.wantSingular {
				t.Fatalf("SingularDeleteKind(%q) = %q, want %q", tc.kind, got, tc.wantSingular)
			}
		})
	}
}

// TestPluralChangeKind_IsSingularDeleteKindsInverse guards the round trip
// every real DraftDelete.Kind must survive: singular -> plural -> singular.
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
	for _, k := range []string{"contact", "policy"} {
		if !kbstore.IsSingletonKind(k) {
			t.Fatalf("IsSingletonKind(%q) = false, want true", k)
		}
	}
	for _, k := range []string{"topic", "tariff", "product", "delivery_zone", "config", ""} {
		if kbstore.IsSingletonKind(k) {
			t.Fatalf("IsSingletonKind(%q) = true, want false", k)
		}
	}
}

// TestApproveEntity_PoliciesDeleteMarkerApplies is the "policie" regression:
// strings.TrimSuffix(kind, "s") turned "policies" into "policie", so a staged
// policy deletion's marker (Kind: "policy") never matched, and an
// entity-scoped approve of it silently no-op'd (errApproveNothingPending ->
// nil -> HTTP 200, live row untouched).
func TestApproveEntity_PoliciesDeleteMarkerApplies(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	if err := kb.PatchPolicies(ctx, orgID, uuid.Nil, kbstore.PolicyPatch{Warranty: strp("12 месяцев")}); err != nil {
		t.Fatalf("patch policies: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve (publish live policy): %v", err)
	}

	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypePolicies, kbstore.NaturalKeyMain, nil); err != nil {
		t.Fatalf("stage policy deletion: %v", err)
	}

	// The entity-scoped approve a Черновик "Опубликовать" click issues —
	// kind is the HTTP-facing plural, key is domain.PolicySlug ("main"),
	// exactly what handlePlaygroundApproveEntity forwards.
	if err := kb.ApproveVersioned(ctx, orgID, kbstore.ApproveSelector{Kind: "policies", Key: domain.PolicySlug}, nil, uuid.Nil); err != nil {
		t.Fatalf("approve entity policies/%s: %v", domain.PolicySlug, err)
	}

	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM ai_policies WHERE organization_id = $1`, orgID).Scan(&n); err != nil {
		t.Fatalf("count ai_policies: %v", err)
	}
	if n != 0 {
		t.Fatalf("ai_policies row still present after approving its staged deletion (the \"policie\" bug)")
	}
}

// TestApproveEntity_ContactsDeleteMarkerWrittenByMCPKeyApplies is the second
// distinct bug: selectApproved compared d.Key == sel.Key, but MCPDelete
// stages contact deletions under NaturalKeyMain ("main") while the approve
// route (mirroring the row's own ID) sends domain.ContactSlug ("support") —
// two different, both legitimate, spellings of "the one contact singleton"
// that must be treated as the same entity.
func TestApproveEntity_ContactsDeleteMarkerWrittenByMCPKeyApplies(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	if err := kb.PatchContacts(ctx, orgID, uuid.Nil, kbstore.ContactPatch{Phone: strp("+7 700 000 00 00")}); err != nil {
		t.Fatalf("patch contacts: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve (publish live contact): %v", err)
	}

	if _, err := kb.MCPDelete(ctx, orgID, uuid.Nil, kbstore.KBTypeContacts, kbstore.NaturalKeyMain, nil); err != nil {
		t.Fatalf("stage contact deletion: %v", err)
	}

	if err := kb.ApproveVersioned(ctx, orgID, kbstore.ApproveSelector{Kind: "contacts", Key: domain.ContactSlug}, nil, uuid.Nil); err != nil {
		t.Fatalf("approve entity contacts/%s: %v", domain.ContactSlug, err)
	}

	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM ai_contacts WHERE organization_id = $1`, orgID).Scan(&n); err != nil {
		t.Fatalf("count ai_contacts: %v", err)
	}
	if n != 0 {
		t.Fatalf("ai_contacts row still present after approving its staged deletion (the support/main key-mismatch bug)")
	}
}

// TestApproveSummary_SaysPolicyNotPolicie guards approveNote's audit text —
// the same TrimSuffix bug also wrote "approved policie main" to
// ai_audit_log.note.
func TestApproveSummary_SaysPolicyNotPolicie(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	if err := kb.PatchPolicies(ctx, orgID, uuid.Nil, kbstore.PolicyPatch{Warranty: strp("6 месяцев")}); err != nil {
		t.Fatalf("patch policies: %v", err)
	}
	if err := kb.ApproveVersioned(ctx, orgID, kbstore.ApproveSelector{Kind: "policies", Key: domain.PolicySlug}, nil, uuid.Nil); err != nil {
		t.Fatalf("approve entity policies/%s: %v", domain.PolicySlug, err)
	}

	var note string
	if err := db.QueryRow(ctx, `SELECT note FROM ai_audit_log
		WHERE organization_id = $1 AND action = 'approve' ORDER BY created_at DESC LIMIT 1`, orgID).
		Scan(&note); err != nil {
		t.Fatalf("read ai_audit_log: %v", err)
	}
	want := "approved policy " + domain.PolicySlug
	if note != want {
		t.Fatalf("ai_audit_log.note = %q, want %q", note, want)
	}
}

// --- DraftChanges: the Черновик review payload ------------------------------

func TestDraftChanges_EmptyDraftHasNoEntries(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("draft changes: %v", err)
	}
	if len(changes.Topics) != 0 || len(changes.Tariffs) != 0 || len(changes.Products) != 0 ||
		len(changes.Contacts) != 0 || len(changes.Policies) != 0 || len(changes.Zones) != 0 || len(changes.Deletes) != 0 {
		t.Fatalf("expected an empty change set despite a fully seeded live KB, got %+v", changes)
	}
	if changes.Config != nil {
		t.Fatalf("config = %+v, want nil", changes.Config)
	}
}

func TestDraftChanges_ReturnsOnlyPendingRowsNeverLive(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.SeedLiveIfEmpty(ctx, orgID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "e2e-only-pending", Title: "T", BodyMD: "B"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("draft changes: %v", err)
	}
	if len(changes.Topics) != 1 || changes.Topics[0].Slug != "e2e-only-pending" {
		t.Fatalf("topics = %+v, want exactly the one pending topic (the seeded live topics must not appear)", changes.Topics)
	}
}

func TestDraftChanges_SurfacesDeleteMarkersInPluralKind(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.DeleteTopic(ctx, orgID, uuid.Nil, "pricing"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("draft changes: %v", err)
	}
	if len(changes.Deletes) != 1 || changes.Deletes[0].Kind != "topics" || changes.Deletes[0].Key != "pricing" {
		t.Fatalf("deletes = %+v, want [{topics pricing}] — the HTTP-facing PLURAL kind, not the blob's singular \"topic\"", changes.Deletes)
	}
}

func TestDraftChanges_ConfigIsNilWithoutAPendingPatch(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("draft changes: %v", err)
	}
	if changes.Config != nil {
		t.Fatalf("config = %+v, want nil", changes.Config)
	}
}

func TestDraftChanges_ConfigCarriesOnlyPatchedFields(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.PatchConfig(ctx, orgID, uuid.Nil, kbstore.ConfigPatch{Persona: strp("Дружелюбный")}); err != nil {
		t.Fatalf("patch config: %v", err)
	}

	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("draft changes: %v", err)
	}
	if changes.Config == nil {
		t.Fatalf("config = nil, want a pending patch")
	}
	if changes.Config.Persona == nil || *changes.Config.Persona != "Дружелюбный" {
		t.Fatalf("config.Persona = %v, want \"Дружелюбный\"", changes.Config.Persona)
	}
	if changes.Config.Mission != nil {
		t.Fatalf("config.Mission = %v, want nil (never touched)", changes.Config.Mission)
	}
}

// --- CancelChange -------------------------------------------------------

func TestCancelChange_CancelsAnAddition(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "e2e-cancel-addition", Title: "T", BodyMD: "B"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	res, err := kb.CancelChange(ctx, orgID, uuid.Nil, "topics", "e2e-cancel-addition", nil)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if !res.Changed {
		t.Fatalf("changed = false, want true")
	}
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("draft changes: %v", err)
	}
	if len(changes.Topics) != 0 {
		t.Fatalf("topics = %+v, want none pending after cancel", changes.Topics)
	}
}

func TestCancelChange_CancelsAnUpdateLeavingLiveIntact(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "e2e-cancel-update", Title: "Original", BodyMD: "B"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "e2e-cancel-update", Title: "Edited", BodyMD: "B2"}); err != nil {
		t.Fatalf("upsert edit: %v", err)
	}

	res, err := kb.CancelChange(ctx, orgID, uuid.Nil, "topics", "e2e-cancel-update", nil)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if !res.Changed {
		t.Fatalf("changed = false, want true")
	}

	var liveTitle string
	if err := db.QueryRow(ctx, `SELECT title FROM ai_topics WHERE organization_id=$1 AND slug=$2`,
		orgID, "e2e-cancel-update").Scan(&liveTitle); err != nil {
		t.Fatalf("read live topic: %v", err)
	}
	if liveTitle != "Original" {
		t.Fatalf("live title = %q, want unchanged \"Original\"", liveTitle)
	}
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("draft changes: %v", err)
	}
	if len(changes.Topics) != 0 {
		t.Fatalf("topics = %+v, want none pending after cancel", changes.Topics)
	}
}

func TestCancelChange_CancelsAStagedRemoval(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "e2e-cancel-removal", Title: "T", BodyMD: "B"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := kb.Approve(ctx, orgID, kbstore.ApproveSelector{}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := kb.DeleteTopic(ctx, orgID, uuid.Nil, "e2e-cancel-removal"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	res, err := kb.CancelChange(ctx, orgID, uuid.Nil, "topics", "e2e-cancel-removal", nil)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if !res.Changed {
		t.Fatalf("changed = false, want true")
	}

	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM ai_topics WHERE organization_id=$1 AND slug=$2`,
		orgID, "e2e-cancel-removal").Scan(&n); err != nil {
		t.Fatalf("count live topic: %v", err)
	}
	if n != 1 {
		t.Fatalf("live topic count = %d, want 1 — cancelling a staged removal must leave live intact", n)
	}
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("draft changes: %v", err)
	}
	if len(changes.Deletes) != 0 {
		t.Fatalf("deletes = %+v, want none pending after cancel", changes.Deletes)
	}
}

func TestCancelChange_ConfigSingleFieldLeavesOtherFields(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.PatchConfig(ctx, orgID, uuid.Nil, kbstore.ConfigPatch{
		Persona: strp("Персона"), Mission: strp("Миссия"),
	}); err != nil {
		t.Fatalf("patch config: %v", err)
	}

	res, err := kb.CancelChange(ctx, orgID, uuid.Nil, "config", "persona", nil)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if !res.Changed {
		t.Fatalf("changed = false, want true")
	}
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("draft changes: %v", err)
	}
	if changes.Config == nil {
		t.Fatalf("config = nil, want mission still pending")
	}
	if changes.Config.Persona != nil {
		t.Fatalf("config.Persona = %v, want nil (cancelled)", changes.Config.Persona)
	}
	if changes.Config.Mission == nil || *changes.Config.Mission != "Миссия" {
		t.Fatalf("config.Mission = %v, want \"Миссия\" (untouched)", changes.Config.Mission)
	}
}

func TestCancelChange_ConfigMainClearsWholePatch(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.PatchConfig(ctx, orgID, uuid.Nil, kbstore.ConfigPatch{
		Persona: strp("Персона"), Mission: strp("Миссия"),
	}); err != nil {
		t.Fatalf("patch config: %v", err)
	}

	res, err := kb.CancelChange(ctx, orgID, uuid.Nil, "config", kbstore.NaturalKeyMain, nil)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if !res.Changed {
		t.Fatalf("changed = false, want true")
	}
	changes, err := kb.DraftChanges(ctx, orgID)
	if err != nil {
		t.Fatalf("draft changes: %v", err)
	}
	if changes.Config != nil {
		t.Fatalf("config = %+v, want nil (whole patch cleared)", changes.Config)
	}
}

func TestCancelChange_UnknownKindIsErrUnknownKind(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if _, err := kb.CancelChange(ctx, orgID, uuid.Nil, "bogus", "whatever", nil); !errors.Is(err, kbstore.ErrUnknownKind) {
		t.Fatalf("err = %v, want ErrUnknownKind", err)
	}
}

func TestCancelChange_UnknownConfigFieldIsErrUnknownKind(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if _, err := kb.CancelChange(ctx, orgID, uuid.Nil, "config", "not-a-real-field", nil); !errors.Is(err, kbstore.ErrUnknownKind) {
		t.Fatalf("err = %v, want ErrUnknownKind", err)
	}
}

// --- CancelChange idempotency — the bar every mutation reachable by a client
// retry must clear (see kb-draft-review-boundaries: "a repeated logical
// cancel/delete must not write, must not advance an optimistic-concurrency
// version, and must report changed:false"). ---------------------------------

func TestCancelChange_RepeatDoesNotAdvanceVersion(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "e2e-repeat-cancel", Title: "T", BodyMD: "B"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	first, err := kb.CancelChange(ctx, orgID, uuid.Nil, "topics", "e2e-repeat-cancel", nil)
	if err != nil {
		t.Fatalf("first cancel: %v", err)
	}
	if !first.Changed {
		t.Fatalf("first cancel: changed = false, want true")
	}

	second, err := kb.CancelChange(ctx, orgID, uuid.Nil, "topics", "e2e-repeat-cancel", nil)
	if err != nil {
		t.Fatalf("second cancel: %v", err)
	}
	if second.Changed {
		t.Fatalf("second cancel: changed = true, want false — repeating a cancel must be a no-op")
	}
	if second.BaseVersion != first.BaseVersion {
		t.Fatalf("second cancel advanced base_version: %d -> %d, want unchanged", first.BaseVersion, second.BaseVersion)
	}
}

func TestCancelChange_AlreadyAbsentIgnoresStaleIfMatch(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	stale := int64(999999)

	res, err := kb.CancelChange(ctx, orgID, uuid.Nil, "topics", "never-existed", &stale)
	if err != nil {
		t.Fatalf("cancel: %v, want no error even with a wildly wrong If-Match — there is nothing to clobber", err)
	}
	if res.Changed {
		t.Fatalf("changed = true, want false")
	}
}

func TestCancelChange_PresentTargetWithStaleVersionIsErrStale(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()
	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "e2e-stale-cancel", Title: "T", BodyMD: "B"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	stale := int64(999999)

	if _, err := kb.CancelChange(ctx, orgID, uuid.Nil, "topics", "e2e-stale-cancel", &stale); !errors.Is(err, kbstore.ErrStale) {
		t.Fatalf("err = %v, want ErrStale — a real write was about to happen, so the conflict is genuine", err)
	}
}

func TestCancelChange_UnknownKeyDoesNotCreateAnEmptyDraftRow(t *testing.T) {
	kb, orgID, _, db := newTestKB(t)
	ctx := context.Background()

	if _, err := kb.CancelChange(ctx, orgID, uuid.Nil, "topics", "never-existed", nil); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM kbd_draft WHERE organization_id=$1`, orgID).Scan(&n); err != nil {
		t.Fatalf("count kbd_draft: %v", err)
	}
	if n != 0 {
		t.Fatalf("kbd_draft row count = %d, want 0 — cancelling an unknown key must not lazily create a draft row", n)
	}
}

// TestDraftOnly_ContractUnchangedAfterRefactor guards the byte-for-byte
// extraction of draftRowsFromBlob out of draftOnly: kb_read(source=draft)
// and the KB Manager MCP widget both go through DraftOnly, and must keep
// seeing exactly what they saw before — every pending row present with
// Draft:true, no live rows, deletions suppressed per kind including the two
// singletons.
func TestDraftOnly_ContractUnchangedAfterRefactor(t *testing.T) {
	kb, orgID, _, _ := newTestKB(t)
	ctx := context.Background()

	if err := kb.UpsertTopic(ctx, orgID, uuid.Nil, kbstore.TopicInput{Slug: "e2e-draftonly-topic", Title: "T", BodyMD: "B"}); err != nil {
		t.Fatalf("upsert topic: %v", err)
	}
	if err := kb.PatchPolicies(ctx, orgID, uuid.Nil, kbstore.PolicyPatch{Warranty: strp("3 месяца")}); err != nil {
		t.Fatalf("patch policies: %v", err)
	}
	if err := kb.PatchConfig(ctx, orgID, uuid.Nil, kbstore.ConfigPatch{Persona: strp("П")}); err != nil {
		t.Fatalf("patch config: %v", err)
	}
	// An unrelated delete marker must not over-suppress the pending topic
	// above — the same deleted[kind:key] exact-match discipline
	// draftRowsFromBlob inherited byte-for-byte from draftOnly's
	// pre-extraction body.
	if err := kb.DeleteTopic(ctx, orgID, uuid.Nil, "not-live-either"); err != nil {
		t.Fatalf("delete topic: %v", err)
	}

	view, err := kb.DraftOnly(ctx, orgID)
	if err != nil {
		t.Fatalf("draft only: %v", err)
	}
	if len(view.Topics) != 1 || view.Topics[0].Slug != "e2e-draftonly-topic" || !view.Topics[0].Draft {
		t.Fatalf("topics = %+v, want exactly the pending one, Draft:true", view.Topics)
	}
	if len(view.Policies) != 1 || !view.Policies[0].Draft || view.Policies[0].Warranty != "3 месяца" {
		t.Fatalf("policies = %+v, want the pending patch, Draft:true", view.Policies)
	}
	if !view.Config.Draft || view.Config.Persona != "П" {
		t.Fatalf("config = %+v, want the pending persona, Draft:true", view.Config)
	}
	if len(view.Materials) != 0 || len(view.Requests) != 0 {
		t.Fatalf("materials/requests = %+v/%+v, want empty — draftOnly's own guarantee", view.Materials, view.Requests)
	}
}
