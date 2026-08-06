package httpapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/password"
)

// accountAutomationDTO mirrors dto.AccountAutomation for test decoding.
type accountAutomationDTO struct {
	Mode                string              `json:"mode"`
	WaitSeconds         int                 `json:"wait_seconds"`
	WaitSecondsOverride *int                `json:"wait_seconds_override"`
	DefaultWaitSeconds  int                 `json:"default_wait_seconds"`
	Schedule            []scheduleWindowDTO `json:"schedule"`
}

type scheduleWindowDTO struct {
	Weekday     int `json:"weekday"`
	StartMinute int `json:"start_minute"`
	EndMinute   int `json:"end_minute"`
}

type accountDTO struct {
	ID         string               `json:"id"`
	Channel    string               `json:"channel"`
	Automation accountAutomationDTO `json:"automation"`
}

func (h *harness) listNeutralAccounts() []accountDTO {
	var out struct {
		Items []accountDTO `json:"items"`
	}
	h.get("/xchats/api/v1/accounts", &out)
	return out.Items
}

func TestGetAccountsIncludesEffectiveAutomationDefaults(t *testing.T) {
	h := newHarness(t)

	accts := h.listNeutralAccounts()
	if len(accts) != 1 {
		t.Fatalf("want 1 account, got %d", len(accts))
	}
	a := accts[0].Automation
	if a.Mode != "suggestions" {
		t.Errorf("default mode = %q, want suggestions", a.Mode)
	}
	if a.WaitSecondsOverride != nil {
		t.Errorf("default override = %v, want nil", a.WaitSecondsOverride)
	}
	if a.WaitSeconds != 5 || a.DefaultWaitSeconds != 5 {
		t.Errorf("wait_seconds=%d default_wait_seconds=%d, want both 5 (the harness's configured system default)", a.WaitSeconds, a.DefaultWaitSeconds)
	}
	if len(a.Schedule) != 0 {
		t.Errorf("default schedule = %+v, want empty", a.Schedule)
	}
}

func TestUpdateAccountAutomationPersistsModeOverrideAndSchedule(t *testing.T) {
	h := newHarness(t)

	resp, env := h.putJSON("/xchats/api/v1/accounts/"+h.accountID.String()+"/automation", map[string]any{
		"mode":                  "scheduled_auto",
		"wait_seconds_override": 30,
		"schedule": []map[string]any{
			{"weekday": 1, "start_minute": 540, "end_minute": 1020},
			{"weekday": 5, "start_minute": 1320, "end_minute": 360}, // overnight
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update automation status = %d: %v", resp.StatusCode, env)
	}
	var updated accountDTO
	mustPayload(t, env, &updated)
	if updated.Automation.Mode != "scheduled_auto" {
		t.Errorf("mode = %q", updated.Automation.Mode)
	}
	if updated.Automation.WaitSecondsOverride == nil || *updated.Automation.WaitSecondsOverride != 30 {
		t.Errorf("override = %v, want 30", updated.Automation.WaitSecondsOverride)
	}
	if updated.Automation.WaitSeconds != 30 {
		t.Errorf("effective wait_seconds = %d, want 30 (the override)", updated.Automation.WaitSeconds)
	}
	if len(updated.Automation.Schedule) != 2 {
		t.Fatalf("schedule = %+v, want 2 windows", updated.Automation.Schedule)
	}

	// Reload via GET /accounts confirms it's durably persisted, not just
	// echoed back from the PUT response.
	accts := h.listNeutralAccounts()
	if len(accts) != 1 || accts[0].Automation.Mode != "scheduled_auto" || len(accts[0].Automation.Schedule) != 2 {
		t.Fatalf("reread accounts = %+v", accts)
	}

	// A second save fully replaces the schedule and clears the override.
	resp, env = h.putJSON("/xchats/api/v1/accounts/"+h.accountID.String()+"/automation", map[string]any{
		"mode":                  "suggestions",
		"wait_seconds_override": nil,
		"schedule":              []map[string]any{},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("second update status = %d: %v", resp.StatusCode, env)
	}
	var cleared accountDTO
	mustPayload(t, env, &cleared)
	if cleared.Automation.Mode != "suggestions" {
		t.Errorf("mode after second update = %q", cleared.Automation.Mode)
	}
	if cleared.Automation.WaitSecondsOverride != nil {
		t.Errorf("override after clearing = %v, want nil", cleared.Automation.WaitSecondsOverride)
	}
	if len(cleared.Automation.Schedule) != 0 {
		t.Errorf("schedule after clearing = %+v, want empty", cleared.Automation.Schedule)
	}
}

func TestUpdateAccountAutomationValidatesModeWaitAndWindows(t *testing.T) {
	h := newHarness(t)
	path := "/xchats/api/v1/accounts/" + h.accountID.String() + "/automation"

	cases := []struct {
		name string
		body map[string]any
	}{
		{"unknown mode", map[string]any{"mode": "always_on"}},
		{"negative wait override", map[string]any{"mode": "suggestions", "wait_seconds_override": -1}},
		{"wait override over max", map[string]any{"mode": "suggestions", "wait_seconds_override": 61}},
		{"window weekday out of range", map[string]any{"mode": "scheduled_auto", "schedule": []map[string]any{{"weekday": 7, "start_minute": 0, "end_minute": 60}}}},
		{"window start out of range", map[string]any{"mode": "scheduled_auto", "schedule": []map[string]any{{"weekday": 0, "start_minute": 1440, "end_minute": 60}}}},
		{"window end zero", map[string]any{"mode": "scheduled_auto", "schedule": []map[string]any{{"weekday": 0, "start_minute": 0, "end_minute": 0}}}},
		{"window start equals end", map[string]any{"mode": "scheduled_auto", "schedule": []map[string]any{{"weekday": 0, "start_minute": 600, "end_minute": 600}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, env := h.putJSON(path, tc.body)
			if resp.StatusCode != http.StatusBadRequest || errcodeOf(env) != "VALIDATION_ERROR" {
				t.Fatalf("status=%d errcode=%q, want 400 VALIDATION_ERROR", resp.StatusCode, errcodeOf(env))
			}
		})
	}

	// Nothing from the rejected requests should have been persisted.
	accts := h.listNeutralAccounts()
	if accts[0].Automation.Mode != "suggestions" {
		t.Fatalf("a rejected update mutated settings: %+v", accts[0].Automation)
	}
}

func TestUpdateAccountAutomationRejectsCrossOrgAccount(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	org2, err := h.store.SeedOrganization(ctx, "xchats-automation-second")
	if err != nil {
		t.Fatalf("seed second org: %v", err)
	}
	hash, _ := password.Hash("second-org-password-1")
	if _, err := h.store.CreateUser(ctx, org2.ID, "second-org-admin@xchats.test", hash, "Second Org Admin", "admin"); err != nil {
		t.Fatalf("create second-org user: %v", err)
	}
	otherClient, status := attemptLogin(t, h.srv.URL, "second-org-admin@xchats.test", "second-org-password-1")
	if status != http.StatusOK {
		t.Fatalf("second-org login: status=%d", status)
	}

	resp := requestAs(t, otherClient, http.MethodPut, h.srv.URL+"/xchats/api/v1/accounts/"+h.accountID.String()+"/automation", map[string]any{"mode": "off"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("cross-org update status = %d, want 404 (an account belonging to a different org must look like it doesn't exist)", resp.StatusCode)
	}

	// The owning org's settings must be untouched.
	accts := h.listNeutralAccounts()
	if accts[0].Automation.Mode != "suggestions" {
		t.Fatalf("cross-org attempt mutated the real owner's settings: %+v", accts[0].Automation)
	}
}

func TestUpdateAccountAutomationUnknownAccountIs404(t *testing.T) {
	h := newHarness(t)
	resp, env := h.putJSON("/xchats/api/v1/accounts/00000000-0000-0000-0000-000000000000/automation", map[string]any{"mode": "off"})
	if resp.StatusCode != http.StatusNotFound || errcodeOf(env) != "NOT_FOUND" {
		t.Fatalf("status=%d errcode=%q, want 404 NOT_FOUND", resp.StatusCode, errcodeOf(env))
	}
}

// TestOffModeRejectsManualSuggest exercises the httpapi-level half of "off
// means no automatic OR manual drafts": POST /chats/:id/ai-drafts (the
// panel's Suggest/Regenerate button) must be rejected once the channel is
// off — internal/automation's own arm/generate gating is covered at the
// store/automation package level; this pins the HTTP contract.
func TestOffModeRejectsManualSuggest(t *testing.T) {
	h := newHarness(t)
	chatID, _ := h.inject(customerJID, "OFFMODE1", "привет", false)
	h.queue.Wait() // let the (still-suggestions-mode) auto-draft from inject settle first

	resp, env := h.putJSON("/xchats/api/v1/accounts/"+h.accountID.String()+"/automation", map[string]any{"mode": "off"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set off status = %d: %v", resp.StatusCode, env)
	}

	resp, env = h.postJSON("/xchats/api/v1/chats/"+chatID+"/ai-drafts?force=true", map[string]any{})
	if resp.StatusCode != http.StatusConflict || errcodeOf(env) != "AUTOMATION_OFF" {
		t.Fatalf("manual suggest while off: status=%d errcode=%q, want 409 AUTOMATION_OFF", resp.StatusCode, errcodeOf(env))
	}
}
