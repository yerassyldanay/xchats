package httpapi_test

// organization_test.go covers POST /organization/active (Task 15's frontend
// org-switcher endpoint): a logged-in user re-scoping their OWN session to a
// different organization they belong to. The MCP review-handoff redirect
// (mcp_review_handoff_test.go) exercises the same underlying
// store.SetActiveOrganization/ActiveOrganizationForSession machinery via a
// signed token instead of a direct POST — this file is the direct-call
// counterpart the plan's frontend selector actually drives.

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type meResponse struct {
	Organization struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"organization"`
	Organizations []struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"organizations"`
}

func TestSetActiveOrganization_SwitchesAndPersists(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	u, err := h.store.UserByEmail(ctx, adminEmail)
	if err != nil {
		t.Fatalf("user by email: %v", err)
	}
	org2, err := h.store.SeedOrganization(ctx, "xchats-second")
	if err != nil {
		t.Fatalf("seed second org: %v", err)
	}
	if _, err := h.db.Exec(ctx, `
		INSERT INTO organization_users (organization_id, user_id)
		VALUES ($1, $2) ON CONFLICT DO NOTHING`, org2.ID, u.ID); err != nil {
		t.Fatalf("add membership: %v", err)
	}

	var before meResponse
	h.get("/xchats/api/v1/me", &before)
	if before.Organization.ID != h.orgID {
		t.Fatalf("initial active org=%s, want default %s", before.Organization.ID, h.orgID)
	}
	if len(before.Organizations) != 2 {
		t.Fatalf("organizations count=%d, want 2 (body=%+v)", len(before.Organizations), before)
	}

	resp, env := h.postJSON("/xchats/api/v1/organization/active", map[string]any{"organization_id": org2.ID})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("switch status=%d", resp.StatusCode)
	}
	var switched meResponse
	mustPayload(t, env, &switched)
	if switched.Organization.ID != org2.ID {
		t.Fatalf("switched org=%s, want %s", switched.Organization.ID, org2.ID)
	}

	// Persisted server-side, not just echoed in the switch response: a fresh
	// /me on the same session must reflect it too.
	var after meResponse
	h.get("/xchats/api/v1/me", &after)
	if after.Organization.ID != org2.ID {
		t.Fatalf("active org after refetch=%s, want %s", after.Organization.ID, org2.ID)
	}
}

func TestSetActiveOrganization_RejectsNonMemberOrg(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	foreignOrg, err := h.store.SeedOrganization(ctx, "xchats-foreign")
	if err != nil {
		t.Fatalf("seed foreign org: %v", err)
	}
	// Deliberately NOT adding the admin user to foreignOrg.

	resp, _ := h.postJSON("/xchats/api/v1/organization/active", map[string]any{"organization_id": foreignOrg.ID})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", resp.StatusCode)
	}

	var after meResponse
	h.get("/xchats/api/v1/me", &after)
	if after.Organization.ID != h.orgID {
		t.Fatalf("active org=%s, want unchanged %s", after.Organization.ID, h.orgID)
	}
}

func TestSetActiveOrganization_RejectsMissingOrganizationID(t *testing.T) {
	h := newHarness(t)
	resp, _ := h.postJSON("/xchats/api/v1/organization/active", map[string]any{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", resp.StatusCode)
	}
}
