package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// TestListChats_ViewCampaignAndAll is the HTTP-level wiring test for the new
// view/campaign_id query params: view=campaign surfaces only a
// cold-send-created, never-replied-to chat (invisible in the default Inbox
// listing); view=all surfaces both it and a genuinely normal chat together.
func TestListChats_ViewCampaignAndAll(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	normalChatID, _ := h.inject(customerJID, "WA-VIEW-NORMAL", "hello", false)

	c, err := h.store.CreateCampaign(ctx, store.Campaign{
		OrganizationID: h.orgID, Name: "View Test Campaign", AccountID: h.accountID, Channel: "whatsapp",
		MessageBody: "Hi!", CreatedBy: mustAdminUserID(t, h),
	})
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	campaignChatID, _, err := h.store.FindOrCreateChat(ctx, h.accountID, "77099998888@s.whatsapp.net", "77099998888")
	if err != nil {
		t.Fatalf("FindOrCreateChat: %v", err)
	}
	if err := h.store.MarkChatCampaignOnly(ctx, "whatsapp", campaignChatID); err != nil {
		t.Fatalf("MarkChatCampaignOnly: %v", err)
	}
	if err := h.store.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77099998888", Name: "Nurlan"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	msgID, err := h.store.InsertCampaignOutbound(ctx, "whatsapp", campaignChatID, h.accountID, "Hi!", "Hi!")
	if err != nil {
		t.Fatalf("InsertCampaignOutbound: %v", err)
	}
	if _, _, _, err := h.store.SetCampaignAccountLimits(ctx, h.accountID,
		store.CampaignAccountSettingsInput{LimitMode: "custom", MinIntervalSeconds: 1, JitterSeconds: 0},
		[]purecampaign.Tier{{WindowSeconds: 1, MaxSends: 1000}}, nil); err != nil {
		t.Fatalf("SetCampaignAccountLimits: %v", err)
	}
	if _, err := h.store.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start campaign: %v", err)
	}
	claim, ok, err := h.store.ClaimNextRecipient(ctx, h.accountID, time.Now())
	if err != nil || !ok {
		t.Fatalf("ClaimNextRecipient: %v, ok=%v", err, ok)
	}
	if err := h.store.FinalizeAttempt(ctx, store.FinalizeAttemptParams{
		LogID: claim.LogID, AttemptID: claim.AttemptID, RecipientID: claim.RecipientID,
		NewStatus: purecampaign.RecipientSent,
		ChatID:    uuid.NullUUID{UUID: campaignChatID, Valid: true}, MessageID: uuid.NullUUID{UUID: msgID, Valid: true},
	}); err != nil {
		t.Fatalf("FinalizeAttempt: %v", err)
	}

	var inbox struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/chats", &inbox)
	if len(inbox.Items) != 1 || inbox.Items[0]["id"] != normalChatID {
		t.Fatalf("default inbox = %+v, want exactly [%s]", inbox.Items, normalChatID)
	}

	var campaignView struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/chats?view=campaign", &campaignView)
	if len(campaignView.Items) != 1 || campaignView.Items[0]["id"] != campaignChatID.String() {
		t.Fatalf("view=campaign = %+v, want exactly [%s]", campaignView.Items, campaignChatID)
	}
	if campaigns, ok := campaignView.Items[0]["campaigns"].([]any); !ok || len(campaigns) != 1 {
		t.Errorf("view=campaign chat's campaigns field = %v, want one entry", campaignView.Items[0]["campaigns"])
	}

	var allView struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/chats?view=all", &allView)
	if len(allView.Items) != 2 {
		t.Fatalf("view=all = %+v, want both chats", allView.Items)
	}

	// Filtering by this specific campaign returns the same single chat.
	var byCampaign struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/chats?view=campaign&campaign_id="+c.ID.String(), &byCampaign)
	if len(byCampaign.Items) != 1 || byCampaign.Items[0]["id"] != campaignChatID.String() {
		t.Fatalf("view=campaign&campaign_id=<c> = %+v, want exactly [%s]", byCampaign.Items, campaignChatID)
	}

	// An unrecognized view value is rejected, not silently defaulted.
	resp, err := h.client.Get(h.srv.URL + "/xchats/api/v1/chats?view=bogus")
	if err != nil {
		t.Fatalf("GET view=bogus: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("view=bogus status = %d, want 400", resp.StatusCode)
	}

	// campaign_id without view=campaign is rejected too (it would otherwise
	// silently do nothing under the default/all view).
	resp, err = h.client.Get(h.srv.URL + "/xchats/api/v1/chats?campaign_id=" + c.ID.String())
	if err != nil {
		t.Fatalf("GET campaign_id without view=campaign: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("campaign_id without view=campaign status = %d, want 400", resp.StatusCode)
	}
}

// TestListChats_CampaignIDCrossOrgIs404 confirms a campaign_id belonging to
// a different organization 404s exactly like every other cross-org resource
// lookup in this package (orgCampaign), rather than silently returning zero
// rows — which would let a caller distinguish "exists in another org" from
// "does not exist" by response shape alone.
func TestListChats_CampaignIDCrossOrgIs404(t *testing.T) {
	h := newHarness(t)
	other := newHarness(t)
	ctx := context.Background()

	c, err := other.store.CreateCampaign(ctx, store.Campaign{
		OrganizationID: other.orgID, Name: "Other Org Campaign", AccountID: other.accountID, Channel: "whatsapp",
		MessageBody: "Hi!", CreatedBy: mustAdminUserID(t, other),
	})
	if err != nil {
		t.Fatalf("CreateCampaign (other org): %v", err)
	}

	resp, err := h.client.Get(h.srv.URL + "/xchats/api/v1/chats?view=campaign&campaign_id=" + c.ID.String())
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-org campaign_id status = %d, want 404", resp.StatusCode)
	}
}

// TestSetChatStatus_ResponseIncludesCampaigns is the regression guard for a
// real bug this feature had: handleSetChatStatus (and, by the same fix,
// handleReadChat/handleAssignChat/handleCreateChat) used to map its chat
// through bare dto.MapChat for both its HTTP response and its chat.updated
// broadcast, silently omitting Campaigns — so resolving/reopening a
// campaign-linked chat erased its own badge from every connected client's
// already-rendered row (including the actor's own, via the same envelope
// this test reads). All four now route through dto.MapChatWithCampaigns
// (mapChatWithCampaigns locally); this asserts the wiring on one of them
// whose response body actually carries the mapped chat.
func TestSetChatStatus_ResponseIncludesCampaigns(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	c, err := h.store.CreateCampaign(ctx, store.Campaign{
		OrganizationID: h.orgID, Name: "Status Badge Campaign", AccountID: h.accountID, Channel: "whatsapp",
		MessageBody: "Hi!", CreatedBy: mustAdminUserID(t, h),
	})
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	chatID, _, err := h.store.FindOrCreateChat(ctx, h.accountID, "77098887777@s.whatsapp.net", "77098887777")
	if err != nil {
		t.Fatalf("FindOrCreateChat: %v", err)
	}
	if err := h.store.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77098887777", Name: "Askar"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	msgID, err := h.store.InsertCampaignOutbound(ctx, "whatsapp", chatID, h.accountID, "Hi!", "Hi!")
	if err != nil {
		t.Fatalf("InsertCampaignOutbound: %v", err)
	}
	if _, _, _, err := h.store.SetCampaignAccountLimits(ctx, h.accountID,
		store.CampaignAccountSettingsInput{LimitMode: "custom", MinIntervalSeconds: 1, JitterSeconds: 0},
		[]purecampaign.Tier{{WindowSeconds: 1, MaxSends: 1000}}, nil); err != nil {
		t.Fatalf("SetCampaignAccountLimits: %v", err)
	}
	if _, err := h.store.SetCampaignStatus(ctx, c.ID, purecampaign.StatusRunning, uuid.NullUUID{}, "started", nil); err != nil {
		t.Fatalf("start campaign: %v", err)
	}
	claim, ok, err := h.store.ClaimNextRecipient(ctx, h.accountID, time.Now())
	if err != nil || !ok {
		t.Fatalf("ClaimNextRecipient: %v, ok=%v", err, ok)
	}
	if err := h.store.FinalizeAttempt(ctx, store.FinalizeAttemptParams{
		LogID: claim.LogID, AttemptID: claim.AttemptID, RecipientID: claim.RecipientID,
		NewStatus: purecampaign.RecipientSent,
		ChatID:    uuid.NullUUID{UUID: chatID, Valid: true}, MessageID: uuid.NullUUID{UUID: msgID, Valid: true},
	}); err != nil {
		t.Fatalf("FinalizeAttempt: %v", err)
	}

	resp, env := h.patchJSON("/xchats/api/v1/chats/"+chatID.String()+"/status", map[string]any{"status": "resolved"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH status: status=%d body=%s", resp.StatusCode, env["message"])
	}
	var mapped map[string]any
	mustPayload(t, env, &mapped)
	campaigns, ok := mapped["campaigns"].([]any)
	if !ok || len(campaigns) != 1 {
		t.Fatalf("expected the status-change response to still carry campaigns, got %+v", mapped["campaigns"])
	}
	first, _ := campaigns[0].(map[string]any)
	if first["id"] != c.ID.String() || first["name"] != "Status Badge Campaign" {
		t.Fatalf("unexpected campaign ref: %+v", first)
	}
}

func mustAdminUserID(t *testing.T, h *harness) uuid.UUID {
	t.Helper()
	u, err := h.store.UserByEmail(context.Background(), adminEmail)
	if err != nil {
		t.Fatalf("UserByEmail: %v", err)
	}
	return u.ID
}
