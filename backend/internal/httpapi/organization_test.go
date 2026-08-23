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

func TestUpdateOrg_NameAndTimezone(t *testing.T) {
	h := newHarness(t)

	var before struct {
		Timezone string `json:"timezone"`
	}
	h.get("/xchats/api/v1/organization", &before)
	if before.Timezone != "Asia/Almaty" {
		t.Fatalf("default timezone = %q, want Asia/Almaty (migration 0012's own default)", before.Timezone)
	}

	tz := "Europe/Moscow"
	resp, env := h.putJSON("/xchats/api/v1/organization", map[string]any{"name": "Renamed Org", "timezone": tz})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status=%d body=%s", resp.StatusCode, env["message"])
	}
	var updated struct {
		Name     string `json:"name"`
		Timezone string `json:"timezone"`
	}
	mustPayload(t, env, &updated)
	if updated.Name != "Renamed Org" || updated.Timezone != tz {
		t.Fatalf("updated = %+v", updated)
	}

	// Persisted, not just echoed.
	var after struct {
		Timezone string `json:"timezone"`
	}
	h.get("/xchats/api/v1/organization", &after)
	if after.Timezone != tz {
		t.Errorf("timezone after refetch = %q, want %q", after.Timezone, tz)
	}

	// An invalid IANA zone name is rejected without touching the stored value.
	resp, _ = h.putJSON("/xchats/api/v1/organization", map[string]any{"name": "Renamed Org", "timezone": "Not/AZone"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid timezone status = %d, want 400", resp.StatusCode)
	}
	h.get("/xchats/api/v1/organization", &after)
	if after.Timezone != tz {
		t.Errorf("timezone after rejected update = %q, want unchanged %q", after.Timezone, tz)
	}

	// Omitting timezone entirely leaves it untouched.
	resp, env = h.putJSON("/xchats/api/v1/organization", map[string]any{"name": "Renamed Again"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status=%d body=%s", resp.StatusCode, env["message"])
	}
	mustPayload(t, env, &updated)
	if updated.Timezone != tz {
		t.Errorf("timezone after name-only update = %q, want unchanged %q", updated.Timezone, tz)
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
