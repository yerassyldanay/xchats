package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// seedTemplateFixture seeds an org and a user — templates carry no account
// at all, so this is lighter than seedCampaignFixture.
func seedTemplateFixture(t *testing.T, st *store.Store, ctx context.Context) (orgID, userID uuid.UUID) {
	t.Helper()
	org, err := st.SeedOrganization(ctx, "templates-test-org-"+uuid.NewString())
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	user, err := st.SeedUser(ctx, org.ID, uuid.NewString()+"@example.com", "hash", "Tester")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return org.ID, user.ID
}

func mustCreateTemplate(t *testing.T, st *store.Store, ctx context.Context, orgID, userID uuid.UUID, name, body string) store.CampaignTemplate {
	t.Helper()
	tmpl, err := st.CreateCampaignTemplate(ctx, store.CampaignTemplate{
		OrganizationID: orgID, Name: name, MessageBody: body, CreatedBy: userID,
	})
	if err != nil {
		t.Fatalf("create campaign template: %v", err)
	}
	return tmpl
}

func TestCreateAndReadCampaignTemplate(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID := seedTemplateFixture(t, st, ctx)

	tmpl := mustCreateTemplate(t, st, ctx, orgID, userID, "Summer promo", "Hi {{name}}, {{discount}}% off!")
	if tmpl.IsArchived {
		t.Error("a freshly created template must not be archived")
	}
	if len(tmpl.Variables) != 2 || tmpl.Variables[0] != "name" || tmpl.Variables[1] != "discount" {
		t.Errorf("Variables = %v, want [name discount]", tmpl.Variables)
	}

	got, err := st.CampaignTemplateByIDForOrg(ctx, tmpl.ID, orgID)
	if err != nil {
		t.Fatalf("CampaignTemplateByIDForOrg: %v", err)
	}
	if got.Name != "Summer promo" {
		t.Errorf("Name = %q, want Summer promo", got.Name)
	}

	if _, err := st.CampaignTemplateByIDForOrg(ctx, tmpl.ID, uuid.New()); err != store.ErrNotFound {
		t.Errorf("cross-org read = %v, want ErrNotFound", err)
	}
}

func TestUpdateCampaignTemplate(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID := seedTemplateFixture(t, st, ctx)
	tmpl := mustCreateTemplate(t, st, ctx, orgID, userID, "Original", "Hi {{name}}")

	newName := "Renamed"
	newBody := "Hello {{name}}, {{offer}} awaits"
	updated, err := st.UpdateCampaignTemplate(ctx, tmpl.ID, store.CampaignTemplatePatch{Name: &newName, MessageBody: &newBody})
	if err != nil {
		t.Fatalf("UpdateCampaignTemplate: %v", err)
	}
	if updated.Name != newName || updated.MessageBody != newBody {
		t.Errorf("updated = %+v, want name=%q body=%q", updated, newName, newBody)
	}
	if len(updated.Variables) != 2 || updated.Variables[0] != "name" || updated.Variables[1] != "offer" {
		t.Errorf("Variables after update = %v, want [name offer] (re-extracted, not stale)", updated.Variables)
	}

	// A partial patch (name only) must leave the body/variables untouched.
	anotherName := "Renamed again"
	updated2, err := st.UpdateCampaignTemplate(ctx, tmpl.ID, store.CampaignTemplatePatch{Name: &anotherName})
	if err != nil {
		t.Fatalf("UpdateCampaignTemplate (name only): %v", err)
	}
	if updated2.MessageBody != newBody {
		t.Errorf("MessageBody after name-only patch = %q, want unchanged %q", updated2.MessageBody, newBody)
	}

	if _, err := st.UpdateCampaignTemplate(ctx, uuid.New(), store.CampaignTemplatePatch{Name: &newName}); err != store.ErrNotFound {
		t.Errorf("update unknown id = %v, want ErrNotFound", err)
	}
}

func TestArchiveAndRestoreCampaignTemplate(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID := seedTemplateFixture(t, st, ctx)
	tmpl := mustCreateTemplate(t, st, ctx, orgID, userID, "Seasonal", "Happy holidays {{name}}")

	archived, err := st.SetCampaignTemplateArchived(ctx, tmpl.ID, true)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !archived.IsArchived {
		t.Error("IsArchived = false after archiving, want true")
	}

	// Idempotent: archiving an already-archived template is a no-op success.
	archivedAgain, err := st.SetCampaignTemplateArchived(ctx, tmpl.ID, true)
	if err != nil || !archivedAgain.IsArchived {
		t.Errorf("re-archiving = %+v, %v, want IsArchived=true, no error", archivedAgain, err)
	}

	restored, err := st.SetCampaignTemplateArchived(ctx, tmpl.ID, false)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.IsArchived {
		t.Error("IsArchived = true after restoring, want false")
	}

	if _, err := st.SetCampaignTemplateArchived(ctx, uuid.New(), true); err != store.ErrNotFound {
		t.Errorf("archive unknown id = %v, want ErrNotFound", err)
	}
}

func TestListCampaignTemplatesForOrg_ArchivedFilterAndSearch(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID := seedTemplateFixture(t, st, ctx)
	otherOrgID, otherUserID := seedTemplateFixture(t, st, ctx)

	active := mustCreateTemplate(t, st, ctx, orgID, userID, "Летняя акция", "Привет {{name}}")
	toArchive := mustCreateTemplate(t, st, ctx, orgID, userID, "Old winter sale", "Bye {{name}}")
	if _, err := st.SetCampaignTemplateArchived(ctx, toArchive.ID, true); err != nil {
		t.Fatalf("archive: %v", err)
	}
	_ = mustCreateTemplate(t, st, ctx, otherOrgID, otherUserID, "Another org's template", "Hi")

	activeList, activeTotal, err := st.ListCampaignTemplatesForOrg(ctx, orgID, false, "", 50, 0)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if activeTotal != 1 || len(activeList) != 1 || activeList[0].ID != active.ID {
		t.Errorf("active list = %+v (total %d), want just %q", activeList, activeTotal, active.Name)
	}

	archivedList, archivedTotal, err := st.ListCampaignTemplatesForOrg(ctx, orgID, true, "", 50, 0)
	if err != nil {
		t.Fatalf("list archived: %v", err)
	}
	if archivedTotal != 1 || len(archivedList) != 1 || archivedList[0].ID != toArchive.ID {
		t.Errorf("archived list = %+v (total %d), want just %q", archivedList, archivedTotal, toArchive.Name)
	}

	// Search is case- and script-insensitive (unicode_lower) — "летн" (lowercase,
	// partial) must still find "Летняя акция" (capitalized Cyrillic).
	found, foundTotal, err := st.ListCampaignTemplatesForOrg(ctx, orgID, false, "летн", 50, 0)
	if err != nil {
		t.Fatalf("list with search: %v", err)
	}
	if foundTotal != 1 || len(found) != 1 || found[0].ID != active.ID {
		t.Errorf("search 'летн' = %+v (total %d), want just %q", found, foundTotal, active.Name)
	}

	notFound, notFoundTotal, err := st.ListCampaignTemplatesForOrg(ctx, orgID, false, "nonexistent", 50, 0)
	if err != nil {
		t.Fatalf("list with no-match search: %v", err)
	}
	if notFoundTotal != 0 || len(notFound) != 0 {
		t.Errorf("search 'nonexistent' = %+v (total %d), want empty", notFound, notFoundTotal)
	}
}
