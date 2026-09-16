package mcpserver_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	campaigndiag "github.com/yerassyldanay/xchats/backend/internal/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
	"github.com/yerassyldanay/xchats/backend/internal/mcpserver"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/whatsapp"
)

// diagFixture seeds one organization/user/WhatsApp account for the campaign
// diagnostic MCP tools' tests, and provides the same claim/finalize
// primitives internal/store's own seedCampaignFixture/claimAndFinalize
// helpers use (campaign_membership_test.go) — reimplemented here because
// those are private to the internal/store test package, which
// mcpserver_test cannot import.
type diagFixture struct {
	srv       *mcpserver.Server
	principal mcpauth.Principal
	st        *store.Store
	orgID     uuid.UUID
	userID    uuid.UUID
	acctID    uuid.UUID
}

func newDiagFixture(t *testing.T, wa whatsapp.Manager) diagFixture {
	t.Helper()
	ctx := context.Background()
	st := dbtest.New(t)
	org, err := st.SeedOrganization(ctx, "diag-org-"+uuid.NewString())
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	user, err := st.SeedUser(ctx, org.ID, uuid.NewString()+"@example.com", "hash", "Tester")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	acctID := uuid.New()
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: acctID, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName: "Test WA", ExternalAccountRef: "7770000000@s.whatsapp.net", ExternalHandle: "77700000000",
		ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	srv := mcpserver.New(mcpserver.Deps{Store: st, WA: wa})
	principal := mcpauth.Principal{
		UserID: user.ID, OrganizationID: org.ID, ClientID: "test-client",
		Scopes: []string{mcpauth.ScopeCampaignsRead},
	}
	return diagFixture{srv: srv, principal: principal, st: st, orgID: org.ID, userID: user.ID, acctID: acctID}
}

func (f diagFixture) allowUnthrottledSends(t *testing.T) {
	t.Helper()
	if _, _, _, err := f.st.SetCampaignAccountLimits(context.Background(), f.acctID,
		store.CampaignAccountSettingsInput{LimitMode: "custom", MinIntervalSeconds: 1, JitterSeconds: 0},
		[]purecampaign.Tier{{WindowSeconds: 1, MaxSends: 1000}}, nil); err != nil {
		t.Fatalf("SetCampaignAccountLimits: %v", err)
	}
}

func (f diagFixture) createCampaign(t *testing.T, name, body string) store.Campaign {
	t.Helper()
	c, err := f.st.CreateCampaign(context.Background(), store.Campaign{
		OrganizationID: f.orgID, Name: name, AccountID: f.acctID, Channel: "whatsapp",
		MessageBody: body, CreatedBy: f.userID,
	})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	return c
}

func (f diagFixture) start(t *testing.T, campaignID uuid.UUID) {
	t.Helper()
	if _, err := f.st.SetCampaignStatus(context.Background(), campaignID, purecampaign.StatusRunning,
		uuid.NullUUID{UUID: f.userID, Valid: true}, "started", nil); err != nil {
		t.Fatalf("start campaign: %v", err)
	}
}

func (f diagFixture) claim(t *testing.T, now time.Time) store.Claim {
	t.Helper()
	claim, ok, err := f.st.ClaimNextRecipient(context.Background(), f.acctID, now)
	if err != nil || !ok {
		t.Fatalf("ClaimNextRecipient: %v, ok=%v", err, ok)
	}
	return claim
}

// finalizeSent completes claim as a first-try provider-accepted success,
// resolving a real chat/message exactly like internal/campaign.Runner.send
// would for a cold send.
func (f diagFixture) finalizeSent(t *testing.T, claim store.Claim) {
	t.Helper()
	ctx := context.Background()
	chatID, _, err := f.st.FindOrCreateChat(ctx, f.acctID, claim.NormalizedIdentity+"@s.whatsapp.net", claim.NormalizedIdentity)
	if err != nil {
		t.Fatalf("FindOrCreateChat: %v", err)
	}
	msgID, err := f.st.InsertCampaignOutbound(ctx, "whatsapp", chatID, f.acctID, "Hi!", "Hi!")
	if err != nil {
		t.Fatalf("InsertCampaignOutbound: %v", err)
	}
	if err := f.st.FinalizeAttempt(ctx, store.FinalizeAttemptParams{
		LogID: claim.LogID, AttemptID: claim.AttemptID, RecipientID: claim.RecipientID,
		NewStatus: purecampaign.RecipientSent,
		ChatID:    uuid.NullUUID{UUID: chatID, Valid: true}, MessageID: uuid.NullUUID{UUID: msgID, Valid: true},
		AttemptOutcome: "provider_accepted", ProviderMessageID: "WAMID-" + claim.NormalizedIdentity,
		Event: "provider_accepted",
	}); err != nil {
		t.Fatalf("FinalizeAttempt (sent): %v", err)
	}
}

// finalizeRetryScheduled completes claim as a transient, retryable failure —
// the recipient stays 'pending' with a backoff floor set.
func (f diagFixture) finalizeRetryScheduled(t *testing.T, claim store.Claim, code campaigndiag.ErrorCode, detail string) {
	t.Helper()
	next := time.Now().Add(time.Hour)
	retryable := true
	if err := f.st.FinalizeAttempt(context.Background(), store.FinalizeAttemptParams{
		LogID: claim.LogID, AttemptID: claim.AttemptID, RecipientID: claim.RecipientID,
		NewStatus: purecampaign.RecipientPending, FailureReason: detail, NextAttemptAt: &next,
		AttemptOutcome: "failed", ErrorCode: string(code), Retryable: &retryable,
		Event: "retry_scheduled",
	}); err != nil {
		t.Fatalf("FinalizeAttempt (retry scheduled): %v", err)
	}
}

// finalizeTerminalFailure completes claim as a permanent, non-retryable
// failure — the recipient reaches 'failed' with no retry ever scheduled.
func (f diagFixture) finalizeTerminalFailure(t *testing.T, claim store.Claim, code campaigndiag.ErrorCode, detail string) {
	t.Helper()
	retryable := false
	if err := f.st.FinalizeAttempt(context.Background(), store.FinalizeAttemptParams{
		LogID: claim.LogID, AttemptID: claim.AttemptID, RecipientID: claim.RecipientID,
		NewStatus: purecampaign.RecipientFailed, FailureReason: detail,
		AttemptOutcome: "failed", ErrorCode: string(code), Retryable: &retryable,
		Event: "provider_rejected",
	}); err != nil {
		t.Fatalf("FinalizeAttempt (terminal failure): %v", err)
	}
}

func callToolRaw(t *testing.T, srv *mcpserver.Server, principal mcpauth.Principal, name string, args map[string]any) map[string]any {
	t.Helper()
	return callTool(t, srv, principal, name, args)
}

func structuredContent(t *testing.T, res map[string]any) map[string]any {
	t.Helper()
	sc, ok := res["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("expected a map structuredContent, got %#v", res["structuredContent"])
	}
	return sc
}

// TestCampaignDiagnosticTools_ScopeEnforced confirms all 4 diagnostic tools
// are gated on campaigns:read specifically — a principal holding every KB
// scope but not this one must still be rejected, mirroring
// TestToolsCall_MissingScopeIsRejectedAsToolError's KB-side pattern.
func TestCampaignDiagnosticTools_ScopeEnforced(t *testing.T) {
	f := newDiagFixture(t, nil)
	f.principal.Scopes = []string{mcpauth.ScopeKBRead, mcpauth.ScopeKBDraftWrite}
	c := f.createCampaign(t, "Scope Test", "Hi!")

	for _, call := range []struct {
		tool string
		args map[string]any
	}{
		{"diagnose_campaign", map[string]any{"campaign_id": c.ID.String()}},
		{"campaign_recipients", map[string]any{"campaign_id": c.ID.String()}},
		{"campaign_events", map[string]any{"campaign_id": c.ID.String()}},
		{"whatsapp_connection_status", map[string]any{"account_id": f.acctID.String()}},
	} {
		res := callToolRaw(t, f.srv, f.principal, call.tool, call.args)
		if res["isError"] != true {
			t.Fatalf("%s: expected isError without campaigns:read scope, got %+v", call.tool, res)
		}
		meta, ok := res["_meta"].(map[string]any)
		if !ok {
			t.Fatalf("%s: expected a _meta challenge on scope failure, got %+v", call.tool, res)
		}
		challenge, ok := meta["mcp/www_authenticate"].(map[string]any)
		if !ok || challenge["scope"] != mcpauth.ScopeCampaignsRead {
			t.Fatalf("%s: expected the challenge to name %q, got %+v", call.tool, mcpauth.ScopeCampaignsRead, meta)
		}
	}
}

// TestCampaignDiagnosticTools_CrossOrgIsIndistinguishableFromNotFound is the
// authorization requirement's core regression guard: a campaign/account
// that genuinely exists, just in a DIFFERENT organization, must produce the
// exact same tool-error text as an id that does not exist at all — a caller
// must never learn "this resource exists elsewhere" by response shape.
func TestCampaignDiagnosticTools_CrossOrgIsIndistinguishableFromNotFound(t *testing.T) {
	owner := newDiagFixture(t, nil)
	intruder := newDiagFixture(t, nil)
	c := owner.createCampaign(t, "Owned By Org A", "Hi!")

	randomID := uuid.New()
	for _, tc := range []struct {
		tool      string
		fieldName string
		realID    uuid.UUID
	}{
		{"diagnose_campaign", "campaign_id", c.ID},
		{"campaign_recipients", "campaign_id", c.ID},
		{"campaign_events", "campaign_id", c.ID},
		{"whatsapp_connection_status", "account_id", owner.acctID},
	} {
		crossOrg := callToolRaw(t, intruder.srv, intruder.principal, tc.tool, map[string]any{tc.fieldName: tc.realID.String()})
		unknown := callToolRaw(t, intruder.srv, intruder.principal, tc.tool, map[string]any{tc.fieldName: randomID.String()})
		if crossOrg["isError"] != true || unknown["isError"] != true {
			t.Fatalf("%s: expected both cross-org and unknown ids to be isError, got cross-org=%+v unknown=%+v", tc.tool, crossOrg, unknown)
		}
		crossText := crossOrg["content"].([]map[string]any)[0]["text"]
		unknownText := unknown["content"].([]map[string]any)[0]["text"]
		if crossText != unknownText {
			t.Fatalf("%s: cross-org and unknown-id errors differ (%q vs %q) — this leaks cross-org existence", tc.tool, crossText, unknownText)
		}

		// The legitimate owner, same id, must succeed.
		ownResult := callToolRaw(t, owner.srv, owner.principal, tc.tool, map[string]any{tc.fieldName: tc.realID.String()})
		if ownResult["isError"] == true {
			t.Fatalf("%s: owner's own call unexpectedly failed: %+v", tc.tool, ownResult)
		}
	}
}

// TestDiagnoseCampaign_CountsRetriesAndTimeCorrelatedLikelyCause exercises
// diagnose_campaign end to end against 5 recipients covering every
// non-overlapping outcome bucket the tool documents: a first-try
// provider-accepted success, a transient network failure with a retry still
// scheduled (so it stays 'pending', NOT counted under failures_by_reason —
// that bucket is "CURRENTLY failed" only), a permanent invalid-recipient
// failure, a WhatsApp-disconnected failure whose retry ladder is exhausted
// (terminal, and so IS counted), and a crash-interrupted send whose outcome
// stays unknown (never blindly retried). A WhatsApp disconnect event
// recorded inside the campaign's own active window must be cited as a
// time-correlated likely cause for the exhausted disconnected failure
// specifically — never for the still-retrying transient one, which has not
// failed yet.
func TestDiagnoseCampaign_CountsRetriesAndTimeCorrelatedLikelyCause(t *testing.T) {
	f := newDiagFixture(t, nil)
	f.allowUnthrottledSends(t)
	ctx := context.Background()
	c := f.createCampaign(t, "Diagnostics Campaign", "Hi {{name}}!")
	if err := f.st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011111111", Name: "Sent"},
		{NormalizedIdentity: "77022222222", Name: "RetryScheduled"},
		{NormalizedIdentity: "77033333333", Name: "Invalid"},
		{NormalizedIdentity: "77055555555", Name: "DisconnectedExhausted"},
		{NormalizedIdentity: "77044444444", Name: "Crashed"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	f.start(t, c.ID)
	base := time.Now()

	f.finalizeSent(t, f.claim(t, base))
	f.finalizeRetryScheduled(t, f.claim(t, base.Add(2*time.Second)), campaigndiag.ErrorCodeNetwork, "temporary network error")
	f.finalizeTerminalFailure(t, f.claim(t, base.Add(4*time.Second)), campaigndiag.ErrorCodeInvalidRecipient, "not a valid WhatsApp number")
	f.finalizeTerminalFailure(t, f.claim(t, base.Add(6*time.Second)), campaigndiag.ErrorCodeWhatsAppDisconnected,
		"max retries exceeded: account disconnected mid-send")
	// Simulate a crash: claim but never finalize, then reconcile.
	f.claim(t, base.Add(8*time.Second))
	if _, err := f.st.ReconcileStuckSending(ctx); err != nil {
		t.Fatalf("ReconcileStuckSending: %v", err)
	}

	// A disconnect event recorded while the campaign is running falls inside
	// its active window (started_at .. now) by construction.
	if err := f.st.RecordWAConnectionEvent(ctx, f.acctID, uuid.NullUUID{UUID: f.orgID, Valid: true}, "disconnected", "stream closed"); err != nil {
		t.Fatalf("RecordWAConnectionEvent: %v", err)
	}

	res := callToolRaw(t, f.srv, f.principal, "diagnose_campaign", map[string]any{"campaign_id": c.ID.String()})
	if res["isError"] == true {
		t.Fatalf("diagnose_campaign failed: %+v", res)
	}
	sc := structuredContent(t, res)

	if sc["campaign_id"] != c.ID.String() || sc["name"] != "Diagnostics Campaign" || sc["status"] != string(purecampaign.StatusRunning) {
		t.Fatalf("unexpected campaign identity fields: %+v", sc)
	}
	if got := sc["total_recipients"]; got != 5 {
		t.Fatalf("total_recipients = %v, want 5", got)
	}

	counts := sc["counts"].(map[string]any)
	if counts["sent"] != 1 || counts["pending"] != 1 || counts["failed"] != 3 || counts["unknown_outcome"] != 1 || counts["sending"] != 0 || counts["skipped"] != 0 {
		t.Fatalf("unexpected counts: %+v", counts)
	}

	retry := sc["retry_summary"].(map[string]any)
	if retry["scheduled"] != 1 || retry["exhausted"] != 1 || retry["unresolved"] != 1 {
		t.Fatalf("unexpected retry_summary: %+v", retry)
	}

	failures := sc["failures_by_reason"].(map[string]int)
	if failures[string(campaigndiag.ErrorCodeWhatsAppDisconnected)] != 1 {
		t.Fatalf("expected exactly 1 (exhausted, terminal) whatsapp_disconnected failure, got %+v", failures)
	}
	if failures[string(campaigndiag.ErrorCodeInvalidRecipient)] != 1 {
		t.Fatalf("expected exactly 1 invalid_recipient failure, got %+v", failures)
	}
	if _, stillPending := failures[string(campaigndiag.ErrorCodeNetwork)]; stillPending {
		t.Fatalf("the still-retrying (pending) recipient must NOT appear in failures_by_reason, got %+v", failures)
	}

	connHistory, ok := sc["connection"].(map[string]any)["observed_during_campaign"].([]map[string]any)
	if !ok || len(connHistory) != 1 || connHistory[0]["event"] != "disconnected" {
		t.Fatalf("expected exactly 1 disconnected event in the campaign window, got %+v", sc["connection"])
	}
	disconnectEventID := connHistory[0]["event_id"].(string)

	likelyCauses, ok := sc["likely_causes"].([]map[string]any)
	if !ok || len(likelyCauses) == 0 {
		t.Fatalf("expected at least 1 likely cause citing the disconnect event, got %+v", sc["likely_causes"])
	}
	found := false
	for _, cause := range likelyCauses {
		ids, _ := cause["supporting_event_ids"].([]string)
		for _, id := range ids {
			if id == disconnectEventID {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("no likely cause cited the disconnect event %s: %+v", disconnectEventID, likelyCauses)
	}

	facts, _ := sc["facts"].([]string)
	joined := strings.Join(facts, " | ")
	if !strings.Contains(joined, "UNRESOLVED") {
		t.Fatalf("expected a fact naming the unresolved/unknown outcome, got %v", facts)
	}
}

// TestCampaignRecipients_StatusFilterMaskingAndRetryState reuses the same 4
// non-overlapping outcomes to check campaign_recipients' own contract:
// masked identities, the status filter (including the unknown pseudo-filter),
// and per-recipient retry_state.
func TestCampaignRecipients_StatusFilterMaskingAndRetryState(t *testing.T) {
	f := newDiagFixture(t, nil)
	f.allowUnthrottledSends(t)
	ctx := context.Background()
	c := f.createCampaign(t, "Recipients Campaign", "Hi!")
	if err := f.st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011111111", Name: "Sent"},
		{NormalizedIdentity: "77022222222", Name: "Disconnected"},
		{NormalizedIdentity: "77033333333", Name: "Invalid"},
		{NormalizedIdentity: "77044444444", Name: "Crashed"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	f.start(t, c.ID)
	base := time.Now()
	f.finalizeSent(t, f.claim(t, base))
	f.finalizeRetryScheduled(t, f.claim(t, base.Add(2*time.Second)), campaigndiag.ErrorCodeWhatsAppDisconnected, "account disconnected mid-send")
	f.finalizeTerminalFailure(t, f.claim(t, base.Add(4*time.Second)), campaigndiag.ErrorCodeInvalidRecipient, "not a valid WhatsApp number")
	f.claim(t, base.Add(6*time.Second))
	if _, err := f.st.ReconcileStuckSending(ctx); err != nil {
		t.Fatalf("ReconcileStuckSending: %v", err)
	}

	byStatus := func(status string) []map[string]any {
		args := map[string]any{"campaign_id": c.ID.String()}
		if status != "" {
			args["status"] = status
		}
		res := callToolRaw(t, f.srv, f.principal, "campaign_recipients", args)
		if res["isError"] == true {
			t.Fatalf("campaign_recipients(status=%q) failed: %+v", status, res)
		}
		sc := structuredContent(t, res)
		items, _ := sc["items"].([]map[string]any)
		return items
	}

	all := byStatus("")
	if len(all) != 4 {
		t.Fatalf("expected 4 recipients unfiltered, got %d", len(all))
	}
	for _, item := range all {
		identity := item["masked_identity"].(string)
		if !strings.HasSuffix(identity, "1111") && !strings.HasSuffix(identity, "2222") && !strings.HasSuffix(identity, "3333") && !strings.HasSuffix(identity, "4444") {
			t.Fatalf("masked_identity %q did not preserve the expected last 4 digits", identity)
		}
		if strings.Contains(identity, "7701") || strings.Contains(identity, "7702") {
			t.Fatalf("masked_identity %q leaked more than the last 4 characters", identity)
		}
	}

	sent := byStatus("sent")
	if len(sent) != 1 || sent[0]["retry_state"] != "none" {
		t.Fatalf("status=sent: expected 1 recipient with retry_state=none, got %+v", sent)
	}
	if sent[0]["provider_accepted_at"] == nil {
		t.Fatalf("status=sent: expected a non-nil provider_accepted_at, got %+v", sent[0])
	}

	pending := byStatus("pending")
	if len(pending) != 1 || pending[0]["retry_state"] != "scheduled" || pending[0]["next_retry_at"] == nil {
		t.Fatalf("status=pending: expected 1 recipient with retry_state=scheduled and a next_retry_at, got %+v", pending)
	}

	failed := byStatus("failed")
	if len(failed) != 2 {
		t.Fatalf("status=failed: expected 2 recipients (terminal + unknown-outcome), got %d: %+v", len(failed), failed)
	}

	unknown := byStatus("unknown")
	if len(unknown) != 1 {
		t.Fatalf("status=unknown: expected exactly 1 recipient, got %d: %+v", len(unknown), unknown)
	}
	if unknown[0]["latest_attempt"].(map[string]any)["outcome"] != "unknown" {
		t.Fatalf("status=unknown recipient's latest_attempt.outcome != unknown: %+v", unknown[0])
	}

	// An invalid status value is a clean tool error, not a silent no-op.
	bad := callToolRaw(t, f.srv, f.principal, "campaign_recipients", map[string]any{"campaign_id": c.ID.String(), "status": "bogus"})
	if bad["isError"] != true {
		t.Fatalf("expected isError for an unknown status filter, got %+v", bad)
	}
}

// TestCampaignRecipients_PaginationCursorCoversEveryRowExactlyOnce pins the
// opaque-decimal-cursor contract this tool shares with kb_read/kb_summary:
// paging with limit=1 must eventually cover every recipient exactly once,
// with next_cursor present on every page but the last.
func TestCampaignRecipients_PaginationCursorCoversEveryRowExactlyOnce(t *testing.T) {
	f := newDiagFixture(t, nil)
	f.allowUnthrottledSends(t)
	ctx := context.Background()
	c := f.createCampaign(t, "Pagination Campaign", "Hi!")
	if err := f.st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011111111", Name: "A"},
		{NormalizedIdentity: "77022222222", Name: "B"},
		{NormalizedIdentity: "77033333333", Name: "C"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}

	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		args := map[string]any{"campaign_id": c.ID.String(), "limit": 1}
		if cursor != "" {
			args["cursor"] = cursor
		}
		res := callToolRaw(t, f.srv, f.principal, "campaign_recipients", args)
		if res["isError"] == true {
			t.Fatalf("campaign_recipients page: %+v", res)
		}
		sc := structuredContent(t, res)
		items := sc["items"].([]map[string]any)
		if len(items) != 1 {
			t.Fatalf("expected exactly 1 item per page, got %d", len(items))
		}
		id := items[0]["recipient_id"].(string)
		if seen[id] {
			t.Fatalf("recipient %s returned on more than one page", id)
		}
		seen[id] = true
		if sc["total"] != 3 {
			t.Fatalf("total = %v, want 3", sc["total"])
		}
		pages++
		if pages > 10 {
			t.Fatalf("pagination did not terminate after %d pages", pages)
		}
		next, hasNext := sc["next_cursor"]
		if !hasNext || next == "" {
			break
		}
		cursor = next.(string)
	}
	if pages != 3 || len(seen) != 3 {
		t.Fatalf("expected exactly 3 pages covering 3 distinct recipients, got %d pages / %d distinct", pages, len(seen))
	}
}

// TestCampaignEvents_PaginationAndRelevantConnectionWindow confirms
// campaign_events paginates its own timeline consistently (no gaps/overlaps
// across pages) and separately surfaces the sending account's connection
// events that fall inside the campaign's active window.
func TestCampaignEvents_PaginationAndRelevantConnectionWindow(t *testing.T) {
	f := newDiagFixture(t, nil)
	f.allowUnthrottledSends(t)
	ctx := context.Background()
	c := f.createCampaign(t, "Events Campaign", "Hi!")
	if err := f.st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011111111", Name: "A"},
		{NormalizedIdentity: "77022222222", Name: "B"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	f.start(t, c.ID)
	base := time.Now()
	f.finalizeSent(t, f.claim(t, base))
	f.finalizeTerminalFailure(t, f.claim(t, base.Add(2*time.Second)), campaigndiag.ErrorCodeInvalidRecipient, "bad number")
	if err := f.st.RecordWAConnectionEvent(ctx, f.acctID, uuid.NullUUID{UUID: f.orgID, Valid: true}, "connected", ""); err != nil {
		t.Fatalf("RecordWAConnectionEvent: %v", err)
	}

	full := callToolRaw(t, f.srv, f.principal, "campaign_events", map[string]any{"campaign_id": c.ID.String(), "limit": 100})
	fullSC := structuredContent(t, full)
	total := fullSC["total"].(int)
	if total < 4 { // 2 send_attempt_started + provider_accepted + provider_rejected, at minimum
		t.Fatalf("expected at least 4 events, got %d", total)
	}
	connEvents, _ := fullSC["relevant_connection_events"].([]map[string]any)
	if len(connEvents) != 1 || connEvents[0]["event"] != "connected" {
		t.Fatalf("expected exactly 1 relevant connection event (connected), got %+v", connEvents)
	}

	// Re-page with limit=2 and confirm the pages cover `total` events exactly,
	// with no id repeated across pages.
	seen := map[string]bool{}
	cursor := ""
	for {
		args := map[string]any{"campaign_id": c.ID.String(), "limit": 2}
		if cursor != "" {
			args["cursor"] = cursor
		}
		res := callToolRaw(t, f.srv, f.principal, "campaign_events", args)
		sc := structuredContent(t, res)
		items := sc["items"].([]map[string]any)
		for _, it := range items {
			id := it["event_id"].(string)
			if seen[id] {
				t.Fatalf("event %s repeated across pages", id)
			}
			seen[id] = true
		}
		next, hasNext := sc["next_cursor"]
		if !hasNext || next == "" {
			break
		}
		cursor = next.(string)
	}
	if len(seen) != total {
		t.Fatalf("paginated event count = %d, want %d", len(seen), total)
	}
}

// TestWhatsAppConnectionStatus_LiveAndStaleFreshness confirms the tool
// prefers a live connection check when Deps.WA is available, and falls back
// to the last stored state (explicitly marked stale) when it is not — plus
// the account-wide pause blocker.
func TestWhatsAppConnectionStatus_LiveAndStaleFreshness(t *testing.T) {
	live := newDiagFixture(t, nil)
	fake := whatsapp.NewFake(live.st, nil, nil)
	live.srv = mcpserver.New(mcpserver.Deps{Store: live.st, WA: fake})
	if err := live.st.SetAccountState(context.Background(), live.acctID, "connected"); err != nil {
		t.Fatalf("SetAccountState: %v", err)
	}

	res := callToolRaw(t, live.srv, live.principal, "whatsapp_connection_status", map[string]any{"account_id": live.acctID.String()})
	if res["isError"] == true {
		t.Fatalf("whatsapp_connection_status failed: %+v", res)
	}
	sc := structuredContent(t, res)
	if sc["state"] != "connected" || sc["freshness"] != "live" || sc["ready_for_campaign_sending"] != "yes" {
		t.Fatalf("expected a live connected/ready status, got %+v", sc)
	}

	stale := newDiagFixture(t, nil) // Deps.WA left nil — no live check available
	if err := stale.st.SetAccountState(context.Background(), stale.acctID, "disconnected"); err != nil {
		t.Fatalf("SetAccountState: %v", err)
	}
	res = callToolRaw(t, stale.srv, stale.principal, "whatsapp_connection_status", map[string]any{"account_id": stale.acctID.String()})
	if res["isError"] == true {
		t.Fatalf("whatsapp_connection_status failed: %+v", res)
	}
	sc = structuredContent(t, res)
	if sc["state"] != "disconnected" || sc["freshness"] != "stale" || sc["ready_for_campaign_sending"] != "no" {
		t.Fatalf("expected a stale disconnected/not-ready status, got %+v", sc)
	}
	missing, _ := sc["missing_evidence"].([]string)
	if len(missing) == 0 {
		t.Fatalf("expected missing_evidence to explain the absent live check, got %+v", sc)
	}
	blockers, _ := sc["blockers"].([]string)
	if len(blockers) == 0 || !strings.Contains(strings.Join(blockers, " "), "disconnected") {
		t.Fatalf("expected a blocker naming the disconnected state, got %+v", blockers)
	}

	// The account-wide manual pause is a SEPARATE blocker even when connected.
	paused := newDiagFixture(t, nil)
	if err := paused.st.SetAccountState(context.Background(), paused.acctID, "connected"); err != nil {
		t.Fatalf("SetAccountState: %v", err)
	}
	if _, _, _, err := paused.st.SetCampaignAccountLimits(context.Background(), paused.acctID,
		store.CampaignAccountSettingsInput{LimitMode: "custom", MinIntervalSeconds: 1, JitterSeconds: 0, Paused: true},
		nil, nil); err != nil {
		t.Fatalf("SetCampaignAccountLimits (paused): %v", err)
	}
	res = callToolRaw(t, paused.srv, paused.principal, "whatsapp_connection_status", map[string]any{"account_id": paused.acctID.String()})
	sc = structuredContent(t, res)
	if sc["ready_for_campaign_sending"] != "no" {
		t.Fatalf("expected ready_for_campaign_sending=no while account-wide paused, got %+v", sc)
	}
	blockers, _ = sc["blockers"].([]string)
	if !strings.Contains(strings.Join(blockers, " "), "paused") {
		t.Fatalf("expected a blocker naming the manual pause, got %+v", blockers)
	}
}

// TestWhatsAppConnectionStatus_RejectsNonWhatsAppAccount confirms the tool
// refuses to report a connection status for a channel that never has one
// (e.g. a Meta channel-core account), rather than fabricating a value.
// ClaimChannelAccount (rather than SeedAccount, which always seeds a
// wa_accounts row and ignores any Channel field passed to it) is the store
// API that actually produces a non-WhatsApp/Simulator account, visible back
// through AccountByID's cross-channel inbox_accounts_v.
func TestWhatsAppConnectionStatus_RejectsNonWhatsAppAccount(t *testing.T) {
	f := newDiagFixture(t, nil)
	ctx := context.Background()
	metaAcctID := uuid.New()
	if _, err := f.st.ClaimChannelAccount(ctx, store.ChannelAccountClaim{
		ID: metaAcctID, OrganizationID: f.orgID, Channel: "whatsapp_cloud",
		ExternalAccountID: "15550001111", DisplayName: "Test Meta", Handle: "+15550001111",
	}); err != nil {
		t.Fatalf("claim channel account: %v", err)
	}
	res := callToolRaw(t, f.srv, f.principal, "whatsapp_connection_status", map[string]any{"account_id": metaAcctID.String()})
	if res["isError"] != true {
		t.Fatalf("expected isError for a non-WhatsApp account, got %+v", res)
	}
}
