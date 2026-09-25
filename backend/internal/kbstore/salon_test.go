package kbstore_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/password"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// testActor creates a real user for orgID and returns their id — every
// live/draft write's actor_user_id is a real FK into users(id) (ai_audit_log
// enforces it), unlike a plain uuid.New() which only happens to work for
// callers whose OWN validation error fires before the audit-row insert.
func testActor(t *testing.T, st *store.Store, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	hash, err := password.Hash("salon-test-actor-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	u, err := st.CreateUser(context.Background(), orgID, uuid.New().String()+"@salon-test.local", hash, "Salon Test Actor", "admin")
	if err != nil {
		t.Fatalf("create test actor: %v", err)
	}
	return u.ID
}

func findSpecialist(rows []kbstore.SpecialistRow, ref string) *kbstore.SpecialistRow {
	for i := range rows {
		if rows[i].Ref == ref {
			return &rows[i]
		}
	}
	return nil
}

func findService(rows []kbstore.ServiceRow, ref string) *kbstore.ServiceRow {
	for i := range rows {
		if rows[i].Ref == ref {
			return &rows[i]
		}
	}
	return nil
}

// TestPutLiveSpecialist_PortfolioImages is a regression test for a real bug
// caught during manual end-to-end testing (TEST.md §4.2): the closed
// mediaAttachmentFields registry (mcp_media.go) had no KBTypeSpecialist
// entry, so ANY portfolio_images write failed validation with "not a
// recognized media field" even for a perfectly valid, same-organization,
// completed-upload image material — see TestMediaColumnKind_ClosedList
// (mcp_media_test.go), the closed-list pin this same registry feeds, which
// now also expects portfolio_images.
func TestPutLiveSpecialist_PortfolioImages(t *testing.T) {
	kb, orgID, st, _ := newTestKB(t)
	actor := testActor(t, st, orgID)
	ctx := context.Background()
	photo1 := mkAttachMaterial(t, kb, orgID, "image/png", "visible", true)
	photo2 := mkAttachMaterial(t, kb, orgID, "image/png", "visible", true)

	err := kb.PutLiveSpecialist(ctx, orgID, actor, kbstore.SpecialistInput{
		Ref: "alina-kim", FullName: "Алина Ким", Title: "Топ-стилист", Experience: "7 лет",
		Schedule:    aiprompt.Schedule{{Ref: "tue", Day: "Вторник", Start: "10:00", End: "19:00"}},
		BookingURL:  "https://example.com/book/alina",
		SalesStatus: "active",
		Media:       kbstore.SpecialistMedia{PortfolioImages: &[]uuid.UUID{photo1, photo2}},
	})
	if err != nil {
		t.Fatalf("PutLiveSpecialist with valid portfolio images: %v", err)
	}

	live, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("LiveView: %v", err)
	}
	sp := findSpecialist(live.Specialists, "alina-kim")
	if sp == nil {
		t.Fatal("specialist alina-kim not found in live view")
	}
	if len(sp.PortfolioImages) != 2 {
		t.Fatalf("PortfolioImages = %v, want 2 entries", sp.PortfolioImages)
	}
}

// TestPutLiveSpecialist_RejectsForeignMaterial proves portfolio_images still
// enforces same-organization ownership like every other media column.
func TestPutLiveSpecialist_RejectsForeignMaterial(t *testing.T) {
	kb, orgID, st, _ := newTestKB(t)
	actor := testActor(t, st, orgID)
	ctx := context.Background()
	otherOrgRow, err := st.SeedOrganization(ctx, "other-salon-org")
	if err != nil {
		t.Fatalf("seed other org: %v", err)
	}
	// otherOrgRow must belong to the SAME database as kb (a second
	// newTestKB(t) call would open an entirely separate isolated DB, which
	// would make this test pass for the wrong reason — a material that
	// doesn't even exist in kb's own DB, not one this org isolation check
	// actually had to reject).
	foreignPhoto := mkAttachMaterial(t, kb, otherOrgRow.ID, "image/png", "visible", true)

	err = kb.PutLiveSpecialist(ctx, orgID, actor, kbstore.SpecialistInput{
		Ref: "alina-kim", FullName: "Алина Ким", SalesStatus: "active",
		Media: kbstore.SpecialistMedia{PortfolioImages: &[]uuid.UUID{foreignPhoto}},
	})
	if err == nil {
		t.Fatal("want an error attaching another organization's material")
	}
}

// TestUpsertSpecialist_NormalizesSchedule proves the draft lane rejects an
// invalid schedule (PLAN.md's validation rules) and stores the
// canonically-ordered result for a valid one.
func TestUpsertSpecialist_NormalizesSchedule(t *testing.T) {
	kb, orgID, st, _ := newTestKB(t)
	actor := testActor(t, st, orgID)
	ctx := context.Background()

	err := kb.UpsertSpecialist(ctx, orgID, actor, kbstore.SpecialistInput{
		Ref: "bad-schedule", SalesStatus: "active",
		Schedule: aiprompt.Schedule{{Ref: "mon", Start: "10:00", End: "09:00"}}, // overnight: invalid
	})
	if err == nil {
		t.Fatal("want an error for an overnight (start >= end) schedule day")
	}

	// Friday before Monday in the input; the stored/read-back result must be
	// canonically Monday-first (NormalizeSchedule's own contract).
	err = kb.UpsertSpecialist(ctx, orgID, actor, kbstore.SpecialistInput{
		Ref: "diana-nur", SalesStatus: "active",
		Schedule: aiprompt.Schedule{
			{Ref: "fri", Day: "Пятница", Start: "11:00", End: "20:00"},
			{Ref: "mon", Day: "Понедельник", Start: "11:00", End: "20:00"},
		},
	})
	if err != nil {
		t.Fatalf("UpsertSpecialist with a valid (unordered) schedule: %v", err)
	}
	draft, err := kb.Draft(ctx, orgID)
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	sp := findSpecialist(draft.Specialists, "diana-nur")
	if sp == nil {
		t.Fatal("diana-nur not found in draft view")
	}
	if len(sp.Schedule) != 2 || sp.Schedule[0].Ref != "mon" || sp.Schedule[1].Ref != "fri" {
		t.Fatalf("schedule not canonically ordered: %+v", sp.Schedule)
	}
}

// TestValidateService_Hierarchy exercises the write-time hierarchy rules
// (salon_validate.go's validateService) through the live lane — the same
// rules aiprompt.buildServiceFacts re-checks at render time.
func TestValidateService_Hierarchy(t *testing.T) {
	kb, orgID, st, _ := newTestKB(t)
	actor := testActor(t, st, orgID)
	ctx := context.Background()

	mustPutLiveService := func(t *testing.T, in kbstore.ServiceInput) {
		t.Helper()
		if err := kb.PutLiveService(ctx, orgID, actor, in); err != nil {
			t.Fatalf("PutLiveService(%s): %v", in.Ref, err)
		}
	}
	mustPutLiveService(t, kbstore.ServiceInput{Ref: "haircut-women", ServiceType: "base", Name: "Женская стрижка", SalesStatus: "active"})

	cases := []struct {
		name string
		in   kbstore.ServiceInput
	}{
		{"variant without parent_ref", kbstore.ServiceInput{Ref: "x1", ServiceType: "variant", SalesStatus: "active"}},
		{"addon with unknown parent_ref", kbstore.ServiceInput{Ref: "x2", ServiceType: "addon", ParentRef: "does-not-exist", SalesStatus: "active"}},
		{"invalid service_type", kbstore.ServiceInput{Ref: "x3", ServiceType: "combo", SalesStatus: "active"}},
		{"non-positive duration", func() kbstore.ServiceInput {
			d := 0
			return kbstore.ServiceInput{Ref: "x4", ServiceType: "base", Duration: &d, SalesStatus: "active"}
		}()},
		{"unknown specialist_ref", kbstore.ServiceInput{Ref: "x5", ServiceType: "base", SpecialistRefs: []string{"nobody"}, SalesStatus: "active"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := kb.PutLiveService(ctx, orgID, actor, tc.in); err == nil {
				t.Fatal("want an error, got none")
			}
		})
	}

	// A variant whose parent IS itself a variant (two hierarchy levels) must
	// also be rejected — "only one hierarchy level is supported".
	mustPutLiveService(t, kbstore.ServiceInput{Ref: "haircut-short", ParentRef: "haircut-women", ServiceType: "variant", Name: "Короткая", SalesStatus: "active"})
	if err := kb.PutLiveService(ctx, orgID, actor, kbstore.ServiceInput{
		Ref: "x6", ParentRef: "haircut-short", ServiceType: "addon", SalesStatus: "active",
	}); err == nil {
		t.Fatal("want an error nesting an addon under a variant (two hierarchy levels)")
	}

	// The valid case: an addon under the real base.
	mustPutLiveService(t, kbstore.ServiceInput{Ref: "hair-spa-mask", ParentRef: "haircut-women", ServiceType: "addon", Name: "Спа-уход", SalesStatus: "active"})
}

// TestPutLiveService_ArchiveCascade pins PLAN.md's exact rule: archiving a
// base atomically archives its currently-active children; restoring the
// base never auto-restores them.
func TestPutLiveService_ArchiveCascade(t *testing.T) {
	kb, orgID, st, _ := newTestKB(t)
	actor := testActor(t, st, orgID)
	ctx := context.Background()

	must := func(in kbstore.ServiceInput) {
		t.Helper()
		if err := kb.PutLiveService(ctx, orgID, actor, in); err != nil {
			t.Fatalf("PutLiveService(%s): %v", in.Ref, err)
		}
	}
	must(kbstore.ServiceInput{Ref: "haircut-women", ServiceType: "base", Name: "Женская стрижка", SalesStatus: "active"})
	must(kbstore.ServiceInput{Ref: "haircut-short", ParentRef: "haircut-women", ServiceType: "variant", Name: "Короткая", SalesStatus: "active"})
	must(kbstore.ServiceInput{Ref: "hair-spa-mask", ParentRef: "haircut-women", ServiceType: "addon", Name: "Спа", SalesStatus: "active"})

	// Archive the base.
	must(kbstore.ServiceInput{Ref: "haircut-women", ServiceType: "base", Name: "Женская стрижка", SalesStatus: "inactive"})

	live, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("LiveView: %v", err)
	}
	for _, ref := range []string{"haircut-women", "haircut-short", "hair-spa-mask"} {
		sv := findService(live.Services, ref)
		if sv == nil {
			t.Fatalf("%s missing from live view", ref)
		}
		if sv.SalesStatus != "inactive" {
			t.Errorf("%s.sales_status = %q, want inactive (archive must cascade)", ref, sv.SalesStatus)
		}
	}

	// Restore the base — children must NOT auto-restore.
	must(kbstore.ServiceInput{Ref: "haircut-women", ServiceType: "base", Name: "Женская стрижка", SalesStatus: "active"})
	live, err = kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("LiveView: %v", err)
	}
	if sv := findService(live.Services, "haircut-women"); sv == nil || sv.SalesStatus != "active" {
		t.Fatalf("base did not restore: %+v", sv)
	}
	for _, ref := range []string{"haircut-short", "hair-spa-mask"} {
		if sv := findService(live.Services, ref); sv == nil || sv.SalesStatus != "inactive" {
			t.Errorf("%s.sales_status = %+v, want to STAY inactive — restoring the base must not auto-restore children", ref, sv)
		}
	}
}

// TestService_SpecialistRefSurvivesArchivedSpecialist proves PLAN.md's rule
// that archiving a specialist "preserves service relationships" — a
// specialist_ref set at service creation time must not be invalidated
// retroactively by later archiving that specialist.
func TestService_SpecialistRefSurvivesArchivedSpecialist(t *testing.T) {
	kb, orgID, st, _ := newTestKB(t)
	actor := testActor(t, st, orgID)
	ctx := context.Background()

	if err := kb.PutLiveSpecialist(ctx, orgID, actor, kbstore.SpecialistInput{Ref: "alina-kim", FullName: "Алина Ким", SalesStatus: "active"}); err != nil {
		t.Fatalf("PutLiveSpecialist: %v", err)
	}
	if err := kb.PutLiveService(ctx, orgID, actor, kbstore.ServiceInput{
		Ref: "haircut-women", ServiceType: "base", Name: "Женская стрижка",
		SpecialistRefs: []string{"alina-kim"}, SalesStatus: "active",
	}); err != nil {
		t.Fatalf("PutLiveService: %v", err)
	}

	// Archive the specialist.
	if err := kb.PutLiveSpecialist(ctx, orgID, actor, kbstore.SpecialistInput{Ref: "alina-kim", FullName: "Алина Ким", SalesStatus: "inactive"}); err != nil {
		t.Fatalf("archive specialist: %v", err)
	}

	// Editing the SAME service again (without touching specialist_refs)
	// must not fail just because alina-kim is now archived.
	if err := kb.PutLiveService(ctx, orgID, actor, kbstore.ServiceInput{
		Ref: "haircut-women", ServiceType: "base", Name: "Женская стрижка", Price: "10 000 ₸",
		SpecialistRefs: []string{"alina-kim"}, SalesStatus: "active",
	}); err != nil {
		t.Fatalf("re-saving a service referencing an archived specialist must not fail: %v", err)
	}
	live, err := kb.LiveView(ctx, orgID)
	if err != nil {
		t.Fatalf("LiveView: %v", err)
	}
	sv := findService(live.Services, "haircut-women")
	if sv == nil || len(sv.SpecialistRefs) != 1 || sv.SpecialistRefs[0] != "alina-kim" {
		t.Fatalf("specialist_refs not preserved: %+v", sv)
	}
}
