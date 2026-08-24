package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// campaignRecipientsRequest builds a multipart request for
// POST .../preview or PUT .../recipients — a pasted "text" field, an
// uploaded "file" field, or both.
func (h *harness) campaignRecipientsRequest(t *testing.T, method, path, text string, files map[string][]byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if text != "" {
		if err := mw.WriteField("text", text); err != nil {
			t.Fatalf("write field text: %v", err)
		}
	}
	for filename, data := range files {
		fw, err := mw.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("create form file %s: %v", filename, err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatalf("write form file %s: %v", filename, err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req, err := http.NewRequest(method, h.srv.URL+path, &buf)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func (h *harness) doMultipart(t *testing.T, req *http.Request) (*http.Response, map[string]json.RawMessage) {
	t.Helper()
	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	var env map[string]json.RawMessage
	_ = json.NewDecoder(resp.Body).Decode(&env)
	return resp, env
}

type campaignDTO struct {
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	AccountID          string         `json:"account_id"`
	Channel            string         `json:"channel"`
	Status             string         `json:"status"`
	MessageBody        string         `json:"message_body"`
	Variables          []string       `json:"variables"`
	MinIntervalSeconds *int           `json:"min_interval_seconds"`
	JitterSeconds      *int           `json:"jitter_seconds"`
	ScheduleAt         *string        `json:"schedule_at"`
	RecipientCounts    map[string]int `json:"recipient_counts"`
}

type previewResultDTO struct {
	Rows []struct {
		Raw                string `json:"raw"`
		Name               string `json:"name"`
		NormalizedIdentity string `json:"normalized_identity"`
		Status             string `json:"status"`
		Reason             string `json:"reason"`
	} `json:"rows"`
	Total     int `json:"total"`
	Valid     int `json:"valid"`
	Invalid   int `json:"invalid"`
	Duplicate int `json:"duplicate"`
}

func (h *harness) createCampaign(t *testing.T, name, body string) campaignDTO {
	t.Helper()
	resp, env := h.postJSON("/xchats/api/v1/campaigns", map[string]any{
		"name": name, "account_id": h.accountID.String(), "message_body": body,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create campaign status=%d body=%s", resp.StatusCode, env["message"])
	}
	var c campaignDTO
	mustPayload(t, env, &c)
	return c
}

func TestCreateAndGetCampaign(t *testing.T) {
	h := newHarness(t)
	c := h.createCampaign(t, "Summer promo", "Hi {{name}}, 20% off!")
	if c.Status != "draft" {
		t.Errorf("status = %q, want draft", c.Status)
	}
	if len(c.Variables) != 1 || c.Variables[0] != "name" {
		t.Errorf("variables = %v, want [name]", c.Variables)
	}
	if c.RecipientCounts == nil {
		t.Error("recipient_counts should never be null")
	}

	var got campaignDTO
	h.get("/xchats/api/v1/campaigns/"+c.ID, &got)
	if got.ID != c.ID || got.Name != "Summer promo" {
		t.Errorf("get = %+v", got)
	}

	resp, _ := h.postJSON("/xchats/api/v1/campaigns", map[string]any{"name": "", "account_id": h.accountID.String(), "message_body": "hi"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty name status = %d, want 400", resp.StatusCode)
	}
}

func TestListCampaigns(t *testing.T) {
	h := newHarness(t)
	h.createCampaign(t, "A", "hi")
	h.createCampaign(t, "B", "hi")

	var page struct {
		Items []campaignDTO `json:"items"`
		Total int           `json:"total"`
	}
	h.get("/xchats/api/v1/campaigns", &page)
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("page = %+v", page)
	}
}

func TestUpdateCampaign_NameAndPace(t *testing.T) {
	h := newHarness(t)
	c := h.createCampaign(t, "Original", "Hi!")

	resp, env := h.patchJSON("/xchats/api/v1/campaigns/"+c.ID, map[string]any{
		"name": "Renamed", "min_interval_seconds": 45, "jitter_seconds": 5,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", resp.StatusCode, env["message"])
	}
	var updated campaignDTO
	mustPayload(t, env, &updated)
	if updated.Name != "Renamed" {
		t.Errorf("name = %q", updated.Name)
	}
	if updated.MinIntervalSeconds == nil || *updated.MinIntervalSeconds != 45 {
		t.Errorf("min_interval_seconds = %v", updated.MinIntervalSeconds)
	}

	// Clearing the pace override back to inherit-from-account: both fields
	// explicit null together.
	resp, env = h.patchJSON("/xchats/api/v1/campaigns/"+c.ID, map[string]any{
		"min_interval_seconds": nil, "jitter_seconds": nil,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("clear pace status=%d body=%s", resp.StatusCode, env["message"])
	}
	var cleared campaignDTO
	mustPayload(t, env, &cleared)
	if cleared.MinIntervalSeconds != nil {
		t.Errorf("min_interval_seconds after clear = %v, want nil", cleared.MinIntervalSeconds)
	}

	// One of the pair without the other is rejected.
	resp, _ = h.patchJSON("/xchats/api/v1/campaigns/"+c.ID, map[string]any{"min_interval_seconds": 30})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("lone min_interval_seconds status = %d, want 400", resp.StatusCode)
	}
}

func TestUpdateCampaign_ContentLockedAfterFirstSend(t *testing.T) {
	h := newHarness(t)
	c := h.createCampaign(t, "Locked", "Hi!")
	h.seedSentRecipient(t, c.ID)

	resp, env := h.patchJSON("/xchats/api/v1/campaigns/"+c.ID, map[string]any{"message_body": "New body"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409 CAMPAIGN_LOCKED", resp.StatusCode, env["message"])
	}
	if string(env["errcode"]) != `"CAMPAIGN_LOCKED"` {
		t.Errorf("errcode = %s", env["errcode"])
	}

	// Name/pace edits are still allowed (only content is frozen).
	resp, _ = h.patchJSON("/xchats/api/v1/campaigns/"+c.ID, map[string]any{"name": "Still editable"})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("name edit status = %d, want 200", resp.StatusCode)
	}
}

func TestCampaignPreview_TextInput(t *testing.T) {
	h := newHarness(t)
	c := h.createCampaign(t, "Promo", "Hi {{name}}!")

	req := h.campaignRecipientsRequest(t, http.MethodPost, "/xchats/api/v1/campaigns/"+c.ID+"/preview",
		"77011234567,Aigul\n77011234567,Aigul again\nnot-a-phone,Bad", nil)
	resp, env := h.doMultipart(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", resp.StatusCode, env["message"])
	}
	var result previewResultDTO
	mustPayload(t, env, &result)
	if result.Total != 3 || result.Valid != 1 || result.Duplicate != 1 || result.Invalid != 1 {
		t.Fatalf("result = %+v", result)
	}

	// Preview never persists anything.
	counts := h.campaignCounts(t, c.ID)
	if counts["pending"] != 0 {
		t.Errorf("pending after preview = %d, want 0 (preview must not persist)", counts["pending"])
	}
}

// TestCampaignPreview_SimulatorSkipsWhatsAppRegistrationCheck pins the
// channel split in parseCampaignRecipients. The simulator is cold-send-
// capable like whatsapp but has no provider connection, so the live
// IsOnWhatsApp registration check must not run for it — gating that branch
// on ColdSendCapable (which covers BOTH channels) made a real simulator
// preview fail outright with "whatsmeow: account ... is not connected".
//
// The assertion is that a phone the WhatsApp fake reports as NOT registered
// still previews as valid on a simulator campaign: that can only hold if
// the WhatsApp check was skipped entirely.
func TestCampaignPreview_SimulatorSkipsWhatsAppRegistrationCheck(t *testing.T) {
	h := newHarness(t)
	h.fake.NotOnWhatsApp = map[string]bool{"77011234567": true}

	simAcct, err := h.store.GetOrCreateSimulatorAccount(context.Background(), h.orgID)
	if err != nil {
		t.Fatalf("simulator account: %v", err)
	}

	resp, env := h.postJSON("/xchats/api/v1/campaigns", map[string]any{
		"name": "Sim promo", "account_id": simAcct.ID.String(), "message_body": "Hi {{name}}!",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create simulator campaign status=%d body=%s", resp.StatusCode, env["message"])
	}
	var c campaignDTO
	mustPayload(t, env, &c)

	req := h.campaignRecipientsRequest(t, http.MethodPost, "/xchats/api/v1/campaigns/"+c.ID+"/preview",
		"77011234567,Aigul\n77012223344,Bota", nil)
	resp, env = h.doMultipart(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("simulator preview status=%d body=%s", resp.StatusCode, env["message"])
	}
	var result previewResultDTO
	mustPayload(t, env, &result)
	if result.Valid != 2 || result.Invalid != 0 {
		t.Fatalf("simulator preview = %+v, want 2 valid / 0 invalid (the WhatsApp registration check must not run)", result)
	}
}

func TestCampaignRecipients_ReplaceAndList(t *testing.T) {
	h := newHarness(t)
	c := h.createCampaign(t, "Promo", "Hi {{name}}!")

	req := h.campaignRecipientsRequest(t, http.MethodPut, "/xchats/api/v1/campaigns/"+c.ID+"/recipients",
		"77011234567,Aigul\n77022222222,Bota", nil)
	resp, env := h.doMultipart(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("replace status=%d body=%s", resp.StatusCode, env["message"])
	}
	var result previewResultDTO
	mustPayload(t, env, &result)
	if result.Valid != 2 {
		t.Fatalf("result = %+v", result)
	}

	var page struct {
		Items []struct {
			NormalizedIdentity string `json:"normalized_identity"`
			Name               string `json:"name"`
			Status             string `json:"status"`
		} `json:"items"`
		Total int `json:"total"`
	}
	h.get("/xchats/api/v1/campaigns/"+c.ID+"/recipients", &page)
	if page.Total != 2 {
		t.Fatalf("recipients page = %+v", page)
	}

	events, _ := h.campaignEvents(t, c.ID)
	found := false
	for _, e := range events {
		if e["event"] == "recipients_replaced" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a recipients_replaced event, got %v", events)
	}
}

func TestCampaignRecipients_LockedWhileRunning(t *testing.T) {
	h := newHarness(t)
	c := h.createCampaign(t, "Promo", "Hi!")
	h.replaceRecipients(t, c.ID, "77011234567,Aigul")
	h.startCampaign(t, c.ID)

	req := h.campaignRecipientsRequest(t, http.MethodPut, "/xchats/api/v1/campaigns/"+c.ID+"/recipients", "77022222222,Bota", nil)
	resp, env := h.doMultipart(t, req)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409", resp.StatusCode, env["message"])
	}
}

func TestCampaignLifecycle_StartPauseResumeStop(t *testing.T) {
	h := newHarness(t)
	c := h.createCampaign(t, "Promo", "Hi!")
	h.replaceRecipients(t, c.ID, "77011234567,Aigul")

	started := h.startCampaign(t, c.ID)
	if started.Status != "running" {
		t.Fatalf("after start: status = %q, want running", started.Status)
	}

	resp, env := h.postJSON("/xchats/api/v1/campaigns/"+c.ID+"/pause", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pause status=%d body=%s", resp.StatusCode, env["message"])
	}
	var paused campaignDTO
	mustPayload(t, env, &paused)
	if paused.Status != "paused" {
		t.Errorf("status = %q, want paused", paused.Status)
	}

	resp, env = h.postJSON("/xchats/api/v1/campaigns/"+c.ID+"/resume", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resume status=%d body=%s", resp.StatusCode, env["message"])
	}
	var resumed campaignDTO
	mustPayload(t, env, &resumed)
	if resumed.Status != "running" {
		t.Errorf("status = %q, want running", resumed.Status)
	}

	resp, env = h.postJSON("/xchats/api/v1/campaigns/"+c.ID+"/stop", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stop status=%d body=%s", resp.StatusCode, env["message"])
	}
	var stopped campaignDTO
	mustPayload(t, env, &stopped)
	if stopped.Status != "cancelled" {
		t.Errorf("status = %q, want cancelled", stopped.Status)
	}
}

func TestCampaignLifecycle_InvalidTransitionIsConflict(t *testing.T) {
	h := newHarness(t)
	c := h.createCampaign(t, "Promo", "Hi!")
	h.replaceRecipients(t, c.ID, "77011234567,Aigul")

	// draft -> paused is not a valid direct transition.
	resp, env := h.postJSON("/xchats/api/v1/campaigns/"+c.ID+"/pause", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409", resp.StatusCode, env["message"])
	}
	if string(env["errcode"]) != `"CAMPAIGN_INVALID_TRANSITION"` {
		t.Errorf("errcode = %s", env["errcode"])
	}
}

func TestCampaignLifecycle_StartWithNoRecipientsFails(t *testing.T) {
	h := newHarness(t)
	c := h.createCampaign(t, "Empty", "Hi!")

	resp, env := h.postJSON("/xchats/api/v1/campaigns/"+c.ID+"/start", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409", resp.StatusCode, env["message"])
	}
	if string(env["errcode"]) != `"CAMPAIGN_EMPTY"` {
		t.Errorf("errcode = %s", env["errcode"])
	}
}

// TestDeleteCampaign covers the three outcomes that matter: an abandoned
// draft is removable, a campaign that already delivered is not (its
// send-ledger rows are what the account rate limiter counts against, so
// removing them would hand back headroom), and a live campaign must be
// stopped first rather than deleted out from under the scheduler.
func TestDeleteCampaign(t *testing.T) {
	h := newHarness(t)

	t.Run("draft is deleted", func(t *testing.T) {
		c := h.createCampaign(t, "Abandoned draft", "Hi {{name}}!")
		h.replaceRecipients(t, c.ID, "77011234567,Aigul\n77012223344,Bota")

		status, env := h.del("/xchats/api/v1/campaigns/" + c.ID)
		if status != http.StatusOK {
			t.Fatalf("delete draft status=%d body=%s", status, env["message"])
		}
		if r, _ := h.getRaw("/xchats/api/v1/campaigns/" + c.ID); r.StatusCode != http.StatusNotFound {
			t.Errorf("get after delete status = %d, want 404", r.StatusCode)
		}
	})

	t.Run("campaign with sends is refused", func(t *testing.T) {
		c := h.createCampaign(t, "Already sent", "Hi {{name}}!")
		h.replaceRecipients(t, c.ID, "77013334444,Dana\n77015556666,Timur")
		h.seedSentRecipient(t, c.ID)

		status, _ := h.del("/xchats/api/v1/campaigns/" + c.ID)
		if status != http.StatusConflict {
			t.Fatalf("delete sent campaign status = %d, want 409", status)
		}
		var still campaignDTO
		h.get("/xchats/api/v1/campaigns/"+c.ID, &still)
		if still.ID != c.ID {
			t.Error("a refused delete must leave the campaign in place")
		}
	})

	t.Run("running campaign must be stopped first", func(t *testing.T) {
		c := h.createCampaign(t, "Live one", "Hi {{name}}!")
		h.replaceRecipients(t, c.ID, "77017778888,Alia\n77019990000,Erlan")
		h.startCampaign(t, c.ID)

		status, _ := h.del("/xchats/api/v1/campaigns/" + c.ID)
		if status != http.StatusConflict {
			t.Fatalf("delete running campaign status = %d, want 409", status)
		}
	})
}

func TestCampaignDuplicate(t *testing.T) {
	h := newHarness(t)
	c := h.createCampaign(t, "Original", "Hi!")
	h.replaceRecipients(t, c.ID, "77011234567,Aigul")

	resp, env := h.postJSON("/xchats/api/v1/campaigns/"+c.ID+"/duplicate", nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d body=%s", resp.StatusCode, env["message"])
	}
	var dup campaignDTO
	mustPayload(t, env, &dup)
	if dup.ID == c.ID || dup.Status != "draft" {
		t.Errorf("duplicate = %+v", dup)
	}
	if dup.RecipientCounts["pending"] != 1 {
		t.Errorf("duplicate recipient_counts = %v, want 1 pending copied over", dup.RecipientCounts)
	}
}

func TestCampaignRetryFailed(t *testing.T) {
	h := newHarness(t)
	c := h.createCampaign(t, "Promo", "Hi!")
	h.replaceRecipients(t, c.ID, "77011234567,Aigul")
	h.startCampaign(t, c.ID)
	rid := h.markOneRecipientFailed(t, c.ID)

	resp, env := h.postJSON("/xchats/api/v1/campaigns/"+c.ID+"/recipients/retry-failed", map[string]any{"recipient_ids": []string{rid}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, env["message"])
	}
	var out struct {
		Retried int `json:"retried"`
	}
	mustPayload(t, env, &out)
	if out.Retried != 1 {
		t.Errorf("retried = %d, want 1", out.Retried)
	}
	counts := h.campaignCounts(t, c.ID)
	if counts["failed"] != 0 || counts["pending"] != 1 {
		t.Errorf("counts after retry = %v", counts)
	}
}

func TestAccountSendingBudgetAndLimits(t *testing.T) {
	h := newHarness(t)

	var budget struct {
		AccountID          string `json:"account_id"`
		MinIntervalSeconds int    `json:"min_interval_seconds"`
		Allowed            bool   `json:"allowed"`
	}
	h.get("/xchats/api/v1/accounts/"+h.accountID.String()+"/sending-budget", &budget)
	if budget.AccountID != h.accountID.String() || !budget.Allowed {
		t.Fatalf("budget = %+v, want allowed=true for a fresh account", budget)
	}

	resp, env := h.putJSON("/xchats/api/v1/accounts/"+h.accountID.String()+"/sending-limits", map[string]any{
		"limit_mode": "custom", "min_interval_seconds": 60, "jitter_seconds": 10, "paused": false,
		"tiers":   []map[string]any{{"window_seconds": 3600, "max_sends": 20}},
		"windows": []map[string]any{{"weekday": 1, "start_minute": 480, "end_minute": 1200}},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set limits status=%d body=%s", resp.StatusCode, env["message"])
	}
	var settings struct {
		MinIntervalSeconds int `json:"min_interval_seconds"`
		Tiers              []struct {
			WindowSeconds int `json:"window_seconds"`
			MaxSends      int `json:"max_sends"`
		} `json:"tiers"`
		Windows []struct {
			Weekday int `json:"weekday"`
		} `json:"windows"`
	}
	mustPayload(t, env, &settings)
	if settings.MinIntervalSeconds != 60 || len(settings.Tiers) != 1 || settings.Tiers[0].MaxSends != 20 || len(settings.Windows) != 1 {
		t.Fatalf("settings = %+v", settings)
	}

	var reGet struct {
		MinIntervalSeconds int `json:"min_interval_seconds"`
	}
	h.get("/xchats/api/v1/accounts/"+h.accountID.String()+"/sending-limits", &reGet)
	if reGet.MinIntervalSeconds != 60 {
		t.Errorf("re-GET min_interval_seconds = %d, want 60", reGet.MinIntervalSeconds)
	}

	// Invalid pace (jitter exceeding interval) is rejected.
	resp, _ = h.putJSON("/xchats/api/v1/accounts/"+h.accountID.String()+"/sending-limits", map[string]any{
		"limit_mode": "custom", "min_interval_seconds": 10, "jitter_seconds": 20, "paused": false,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid pace status = %d, want 400", resp.StatusCode)
	}
}

// --- shared test helpers ---------------------------------------------------

func (h *harness) replaceRecipients(t *testing.T, campaignID, text string) previewResultDTO {
	t.Helper()
	req := h.campaignRecipientsRequest(t, http.MethodPut, "/xchats/api/v1/campaigns/"+campaignID+"/recipients", text, nil)
	resp, env := h.doMultipart(t, req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("replace recipients status=%d body=%s", resp.StatusCode, env["message"])
	}
	var result previewResultDTO
	mustPayload(t, env, &result)
	return result
}

func (h *harness) startCampaign(t *testing.T, campaignID string) campaignDTO {
	t.Helper()
	resp, env := h.postJSON("/xchats/api/v1/campaigns/"+campaignID+"/start", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("start status=%d body=%s", resp.StatusCode, env["message"])
	}
	var c campaignDTO
	mustPayload(t, env, &c)
	return c
}

func (h *harness) campaignCounts(t *testing.T, campaignID string) map[string]int {
	t.Helper()
	var c campaignDTO
	h.get("/xchats/api/v1/campaigns/"+campaignID, &c)
	return c.RecipientCounts
}

func (h *harness) campaignEvents(t *testing.T, campaignID string) ([]map[string]any, int) {
	t.Helper()
	var page struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	h.get("/xchats/api/v1/campaigns/"+campaignID+"/events", &page)
	return page.Items, page.Total
}

// seedSentRecipient reaches past the HTTP layer directly into the store to
// give campaignID one genuinely 'sent' recipient — the HTTP API alone can't
// drive an actual send outcome (that's internal/campaign's Runner,
// deliberately not wired into this harness; see campaigns.go's own file
// header), and TestUpdateCampaign_ContentLockedAfterFirstSend only needs
// CanEditContent's own sentCount>0 gate to trip. Leaves the campaign paused
// again afterward (running would also block the pacing edit half of that
// test, which is not what it's checking).
func (h *harness) seedSentRecipient(t *testing.T, campaignID string) {
	t.Helper()
	ctx := context.Background()
	cid, err := uuid.Parse(campaignID)
	if err != nil {
		t.Fatalf("parse campaign id: %v", err)
	}
	if err := h.store.ReplaceCampaignRecipients(ctx, cid, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77099999999", Name: "Seed"},
	}); err != nil {
		t.Fatalf("seed recipient: %v", err)
	}
	if _, _, _, err := h.store.SetCampaignAccountLimits(ctx, h.accountID,
		store.CampaignAccountSettingsInput{LimitMode: "custom", MinIntervalSeconds: 1, JitterSeconds: 0},
		[]purecampaign.Tier{{WindowSeconds: 3600, MaxSends: 1000}}, nil); err != nil {
		t.Fatalf("set account limits: %v", err)
	}
	if _, err := h.store.SetCampaignStatus(ctx, cid, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start campaign: %v", err)
	}
	claim, claimed, err := h.store.ClaimNextRecipient(ctx, h.accountID, time.Now())
	if err != nil || !claimed {
		t.Fatalf("claim: claimed=%v err=%v", claimed, err)
	}
	if err := h.store.FinalizeAttempt(ctx, store.FinalizeAttemptParams{
		LogID: claim.LogID, RecipientID: claim.RecipientID, NewStatus: purecampaign.RecipientSent,
	}); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if _, err := h.store.SetCampaignStatus(ctx, cid, purecampaign.StatusPaused, uuid.NullUUID{}, "paused", nil); err != nil {
		t.Fatalf("pause campaign: %v", err)
	}
}

// markOneRecipientFailed reaches past the HTTP layer directly into the
// store to put one of campaignID's (already running) recipients into a
// terminal 'failed' state — see seedSentRecipient's own doc comment for why.
func (h *harness) markOneRecipientFailed(t *testing.T, campaignID string) string {
	t.Helper()
	ctx := context.Background()
	cid, err := uuid.Parse(campaignID)
	if err != nil {
		t.Fatalf("parse campaign id: %v", err)
	}
	claim, claimed, err := h.store.ClaimNextRecipient(ctx, h.accountID, time.Now())
	if err != nil || !claimed {
		t.Fatalf("claim: claimed=%v err=%v", claimed, err)
	}
	if claim.CampaignID != cid {
		t.Fatalf("claimed recipient for campaign %s, want %s", claim.CampaignID, cid)
	}
	if err := h.store.FinalizeAttempt(ctx, store.FinalizeAttemptParams{
		LogID: claim.LogID, RecipientID: claim.RecipientID, NewStatus: purecampaign.RecipientFailed, FailureReason: "seeded failure",
	}); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	return claim.RecipientID.String()
}
