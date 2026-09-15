package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// allowUnthrottledSends gives acctID effectively unlimited pacing, so a test
// can claim several recipients back to back without waiting out the real
// interval/tier math — mirrors internal/campaign's own test helpers.
func allowUnthrottledSends(t *testing.T, st *store.Store, ctx context.Context, acctID uuid.UUID) {
	t.Helper()
	if _, _, _, err := st.SetCampaignAccountLimits(ctx, acctID,
		store.CampaignAccountSettingsInput{LimitMode: "custom", MinIntervalSeconds: 1, JitterSeconds: 0},
		[]purecampaign.Tier{{WindowSeconds: 1, MaxSends: 1000}}, nil); err != nil {
		t.Fatalf("SetCampaignAccountLimits: %v", err)
	}
}

func startCampaign(t *testing.T, st *store.Store, ctx context.Context, campaignID, userID uuid.UUID) {
	t.Helper()
	if _, err := st.SetCampaignStatus(ctx, campaignID, purecampaign.StatusRunning, uuid.NullUUID{UUID: userID, Valid: true}, "started", nil); err != nil {
		t.Fatalf("start campaign: %v", err)
	}
}

// claimAndFinalize mirrors exactly what internal/campaign's Runner does
// (claim, then finalize with a resolved chat/message) without importing
// that package — internal/campaign imports internal/store, so the reverse
// import here is not possible; store_test exercises the same public
// ClaimNextRecipient/FinalizeAttempt API the Runner itself calls. now is an
// explicit, synthetic clock reading (rather than time.Now()) so a test can
// simulate several sends spaced past the account's own min-interval pacing
// without actually sleeping.
func claimAndFinalize(t *testing.T, st *store.Store, ctx context.Context, acctID uuid.UUID, now time.Time, status purecampaign.RecipientStatus, chatID, msgID uuid.NullUUID) store.Claim {
	t.Helper()
	claim, ok, err := st.ClaimNextRecipient(ctx, acctID, now)
	if err != nil || !ok {
		t.Fatalf("ClaimNextRecipient: %v, ok=%v", err, ok)
	}
	if err := st.FinalizeAttempt(ctx, store.FinalizeAttemptParams{
		LogID: claim.LogID, AttemptID: claim.AttemptID, RecipientID: claim.RecipientID,
		NewStatus: status, ChatID: chatID, MessageID: msgID,
	}); err != nil {
		t.Fatalf("FinalizeAttempt: %v", err)
	}
	return claim
}

// TestListChatsForOrg_CampaignViewIncludesExistingChatReusedByCampaign is the
// central regression test for this feature's core requirement: an EXISTING
// customer conversation a campaign later reuses must become discoverable
// under View="campaign" WITHOUT its chat_state ever becoming 'campaign'
// (that marker is cold-send-only) — membership comes solely from
// campaign_recipients.chat_id, stamped by the same FinalizeAttempt call the
// real Runner makes.
func TestListChatsForOrg_CampaignViewIncludesExistingChatReusedByCampaign(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)

	// An existing normal conversation, created by a genuine customer inbound
	// — never touched by MarkChatCampaignOnly.
	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: acctID, PhoneJID: "77011234567@s.whatsapp.net", RemoteJID: "77011234567@s.whatsapp.net",
		PhoneNumber: "77011234567", Direction: "in", SenderKind: "contact",
		ExternalMessageID: uuid.NewString(), MessageKind: "conversation", Body: "hi",
	})
	if err != nil {
		t.Fatalf("UpsertInbound: %v", err)
	}
	existingChatID := res.ChatID

	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Reuse Campaign", "Hi {{name}}!")
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011234567", Name: "Aigul"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	allowUnthrottledSends(t, st, ctx, acctID)
	startCampaign(t, st, ctx, c.ID, userID)

	msgID, err := st.InsertCampaignOutbound(ctx, "whatsapp", existingChatID, acctID, "Hi Aigul!", "Hi Aigul!")
	if err != nil {
		t.Fatalf("InsertCampaignOutbound: %v", err)
	}
	claimAndFinalize(t, st, ctx, acctID, time.Now(), purecampaign.RecipientSent,
		uuid.NullUUID{UUID: existingChatID, Valid: true}, uuid.NullUUID{UUID: msgID, Valid: true})

	// Still a completely normal chat — chat_state was never touched.
	chat, err := st.ChatByID(ctx, existingChatID)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}
	if chat.ChatState != "open" {
		t.Errorf("chat_state = %q, want open (campaign reuse must never touch it)", chat.ChatState)
	}

	// Inbox (default) still shows it — normal chats are never hidden.
	inbox, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg (inbox): %v", err)
	}
	if total != 1 || len(inbox) != 1 || inbox[0].ID != existingChatID {
		t.Fatalf("inbox view = %+v (total=%d), want exactly [%s]", inbox, total, existingChatID)
	}

	// Campaign view ALSO shows it — this is the requirement chat_state alone
	// cannot satisfy (this chat's state is 'open', never 'campaign').
	campaignView, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, View: "campaign", Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg (campaign): %v", err)
	}
	if total != 1 || len(campaignView) != 1 || campaignView[0].ID != existingChatID {
		t.Fatalf("campaign view = %+v (total=%d), want exactly [%s]", campaignView, total, existingChatID)
	}

	// All returns the same single chat (union of one thing with itself).
	allView, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, View: "all", Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg (all): %v", err)
	}
	if total != 1 || len(allView) != 1 || allView[0].ID != existingChatID {
		t.Fatalf("all view = %+v (total=%d), want exactly [%s]", allView, total, existingChatID)
	}

	// The campaign name is resolvable for this chat.
	refs, err := st.CampaignRefsForChats(ctx, []uuid.UUID{existingChatID})
	if err != nil {
		t.Fatalf("CampaignRefsForChats: %v", err)
	}
	if len(refs[existingChatID]) != 1 || refs[existingChatID][0].Name != "Reuse Campaign" {
		t.Fatalf("CampaignRefsForChats = %+v, want [Reuse Campaign]", refs[existingChatID])
	}
}

// TestListChatsForOrg_CampaignViewExcludesAudienceOnlyBeforeProcessing pins
// the opposite, equally important half of the same requirement: a customer
// merely LISTED as a campaign recipient (before any claim/send ever
// happened) must not make their existing conversation appear under
// View="campaign" — campaign_recipients.chat_id is still NULL, so there is
// no persisted participation yet.
func TestListChatsForOrg_CampaignViewExcludesAudienceOnlyBeforeProcessing(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)

	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: acctID, PhoneJID: "77011234567@s.whatsapp.net", RemoteJID: "77011234567@s.whatsapp.net",
		PhoneNumber: "77011234567", Direction: "in", SenderKind: "contact",
		ExternalMessageID: uuid.NewString(), MessageKind: "conversation", Body: "hi",
	})
	if err != nil {
		t.Fatalf("UpsertInbound: %v", err)
	}

	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Not Yet Processed", "Hi {{name}}!")
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011234567", Name: "Aigul"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	// Deliberately never started/claimed: the recipient stays 'pending' with
	// no chat_id at all.

	campaignView, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, View: "campaign", Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg (campaign): %v", err)
	}
	if total != 0 || len(campaignView) != 0 {
		t.Fatalf("campaign view = %+v (total=%d), want none — audience membership alone must not qualify", campaignView, total)
	}

	// The chat is still a completely ordinary inbox entry.
	inbox, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg (inbox): %v", err)
	}
	if total != 1 || len(inbox) != 1 || inbox[0].ID != res.ChatID {
		t.Fatalf("inbox view = %+v (total=%d), want exactly [%s]", inbox, total, res.ChatID)
	}
}

// TestListChatsForOrg_CampaignViewIncludesFailedSendWithResolvedChat covers
// "keep failed/unknown sends discoverable": a cold-send chat that FAILED
// after the chat was resolved but before a message was ever created (e.g.
// InsertCampaignOutbound itself failing) still has campaign_recipients.
// chat_id set, so it must still show under View="campaign" even though its
// coarse status is 'failed' and it has no message_id at all.
func TestListChatsForOrg_CampaignViewIncludesFailedSendWithResolvedChat(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)

	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Failing Campaign", "Hi!")
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77099999999", Name: "Nurlan"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	allowUnthrottledSends(t, st, ctx, acctID)
	startCampaign(t, st, ctx, c.ID, userID)

	chatID, _, err := st.FindOrCreateChat(ctx, acctID, "77099999999@s.whatsapp.net", "77099999999")
	if err != nil {
		t.Fatalf("FindOrCreateChat: %v", err)
	}
	if err := st.MarkChatCampaignOnly(ctx, "whatsapp", chatID); err != nil {
		t.Fatalf("MarkChatCampaignOnly: %v", err)
	}
	// Finalize as failed with a resolved chat but NO message (simulating
	// InsertCampaignOutbound itself failing right after chat resolution).
	claimAndFinalize(t, st, ctx, acctID, time.Now(), purecampaign.RecipientFailed,
		uuid.NullUUID{UUID: chatID, Valid: true}, uuid.NullUUID{})

	campaignView, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, View: "campaign", Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg (campaign): %v", err)
	}
	if total != 1 || len(campaignView) != 1 || campaignView[0].ID != chatID {
		t.Fatalf("campaign view = %+v (total=%d), want exactly [%s] (failed send, chat still resolved)", campaignView, total, chatID)
	}
}

// TestListChatsForOrg_CampaignFilterByCampaignIDAndDedup covers both
// "filter by a specific campaign_id" and "a chat touched by multiple
// campaigns appears only once": two campaigns share one recipient identity
// (and so, since FindOrCreateChat is idempotent per account+JID, the same
// underlying chat), plus each campaign also has its own distinct recipient.
func TestListChatsForOrg_CampaignFilterByCampaignIDAndDedup(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	allowUnthrottledSends(t, st, ctx, acctID)

	c1 := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Campaign One", "Hi!")
	c2 := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Campaign Two", "Hi!")
	if err := st.ReplaceCampaignRecipients(ctx, c1.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011111111", Name: "Shared"},
		{NormalizedIdentity: "77022222222", Name: "OnlyC1"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients c1: %v", err)
	}
	if err := st.ReplaceCampaignRecipients(ctx, c2.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77011111111", Name: "Shared"},
		{NormalizedIdentity: "77033333333", Name: "OnlyC2"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients c2: %v", err)
	}
	startCampaign(t, st, ctx, c1.ID, userID)
	startCampaign(t, st, ctx, c2.ID, userID)

	// "77011111111" is a SEPARATE campaign_recipients row per campaign (the
	// unique key is (campaign_id, normalized_identity)), so establishing its
	// membership in BOTH c1 and c2 takes two full claims of that identity —
	// FIFO-by-campaign-start-time means claiming c1's queue empty (both of
	// its rows) always precedes c2's, so 4 sequential claims (not 3) drain
	// exactly the 4 rows created above, in a fixed, known order:
	// c1/shared, c1/onlyC1, c2/shared, c2/onlyC2. claimOne resolves the
	// chat from the CLAIM's own identity (never an identity the caller
	// assumed in advance) — exactly like the real Runner, which also has no
	// way to know which recipient it is about to claim until it does.
	baseTime := time.Now()
	claimN := 0
	claimOne := func() (identity string, chatID uuid.UUID) {
		claimN++
		now := baseTime.Add(time.Duration(claimN) * 2 * time.Second)
		claim, ok, err := st.ClaimNextRecipient(ctx, acctID, now)
		if err != nil || !ok {
			t.Fatalf("ClaimNextRecipient #%d: %v, ok=%v", claimN, err, ok)
		}
		jid := claim.NormalizedIdentity + "@s.whatsapp.net"
		chatID, _, err = st.FindOrCreateChat(ctx, acctID, jid, claim.NormalizedIdentity)
		if err != nil {
			t.Fatalf("FindOrCreateChat(%s): %v", claim.NormalizedIdentity, err)
		}
		_ = st.MarkChatCampaignOnly(ctx, "whatsapp", chatID)
		msgID, err := st.InsertCampaignOutbound(ctx, "whatsapp", chatID, acctID, "Hi!", "Hi!")
		if err != nil {
			t.Fatalf("InsertCampaignOutbound(%s): %v", claim.NormalizedIdentity, err)
		}
		if err := st.FinalizeAttempt(ctx, store.FinalizeAttemptParams{
			LogID: claim.LogID, AttemptID: claim.AttemptID, RecipientID: claim.RecipientID,
			NewStatus: purecampaign.RecipientSent,
			ChatID:    uuid.NullUUID{UUID: chatID, Valid: true}, MessageID: uuid.NullUUID{UUID: msgID, Valid: true},
		}); err != nil {
			t.Fatalf("FinalizeAttempt(%s): %v", claim.NormalizedIdentity, err)
		}
		return claim.NormalizedIdentity, chatID
	}

	identity1, sharedChatIDFromC1 := claimOne()
	identity2, onlyC1ChatID := claimOne()
	identity3, sharedChatIDFromC2 := claimOne()
	identity4, onlyC2ChatID := claimOne()
	if identity1 != "77011111111" || identity2 != "77022222222" || identity3 != "77011111111" || identity4 != "77033333333" {
		t.Fatalf("claim order = [%s %s %s %s], want [77011111111 77022222222 77011111111 77033333333] (FIFO by campaign start time)",
			identity1, identity2, identity3, identity4)
	}
	sharedChatID := sharedChatIDFromC1
	if sharedChatIDFromC1 != sharedChatIDFromC2 {
		t.Fatalf("FindOrCreateChat for the same identity/account returned two different chats: %s vs %s", sharedChatIDFromC1, sharedChatIDFromC2)
	}
	if onlyC1ChatID == onlyC2ChatID || sharedChatID == onlyC1ChatID {
		t.Fatalf("distinct identities must resolve to distinct chats: shared=%s c1=%s c2=%s", sharedChatID, onlyC1ChatID, onlyC2ChatID)
	}

	// Unfiltered campaign view: 3 distinct chats, the shared one appearing
	// exactly once despite belonging to two campaigns.
	all, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, View: "campaign", Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg (campaign, unfiltered): %v", err)
	}
	if total != 3 || len(all) != 3 {
		t.Fatalf("unfiltered campaign view = %+v (total=%d), want 3 distinct chats", all, total)
	}
	seen := map[uuid.UUID]bool{}
	for _, ch := range all {
		if seen[ch.ID] {
			t.Fatalf("chat %s appeared more than once in the unfiltered campaign view", ch.ID)
		}
		seen[ch.ID] = true
	}

	// Filtered to campaign 1: the shared chat and c1's own chat, never c2's.
	filtered1, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, View: "campaign", CampaignID: uuid.NullUUID{UUID: c1.ID, Valid: true}, Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg (campaign_id=c1): %v", err)
	}
	gotIDs := map[uuid.UUID]bool{}
	for _, ch := range filtered1 {
		gotIDs[ch.ID] = true
	}
	if total != 2 || !gotIDs[sharedChatID] || !gotIDs[onlyC1ChatID] || gotIDs[onlyC2ChatID] {
		t.Fatalf("campaign_id=c1 view = %+v (total=%d), want exactly [shared, onlyC1]", filtered1, total)
	}

	// Filtered to campaign 2: the shared chat and c2's own chat, never c1's.
	filtered2, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, View: "campaign", CampaignID: uuid.NullUUID{UUID: c2.ID, Valid: true}, Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg (campaign_id=c2): %v", err)
	}
	gotIDs = map[uuid.UUID]bool{}
	for _, ch := range filtered2 {
		gotIDs[ch.ID] = true
	}
	if total != 2 || !gotIDs[sharedChatID] || !gotIDs[onlyC2ChatID] || gotIDs[onlyC1ChatID] {
		t.Fatalf("campaign_id=c2 view = %+v (total=%d), want exactly [shared, onlyC2]", filtered2, total)
	}

	// The shared chat's own campaign refs name both campaigns.
	refs, err := st.CampaignRefsForChats(ctx, []uuid.UUID{sharedChatID})
	if err != nil {
		t.Fatalf("CampaignRefsForChats: %v", err)
	}
	if len(refs[sharedChatID]) != 2 {
		t.Fatalf("CampaignRefsForChats(shared) = %+v, want 2 campaigns", refs[sharedChatID])
	}

	// Pagination stays correct: page size 2 then page 2 together cover all 3
	// distinct chats exactly once, with no overlap and no gap.
	page1, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, View: "campaign", Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("ListChatsForOrg (page1): %v", err)
	}
	page2, total2, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, View: "campaign", Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("ListChatsForOrg (page2): %v", err)
	}
	if total != 3 || total2 != 3 {
		t.Fatalf("total across pages = %d/%d, want 3/3", total, total2)
	}
	if len(page1) != 2 || len(page2) != 1 {
		t.Fatalf("page1/page2 lengths = %d/%d, want 2/1", len(page1), len(page2))
	}
	combined := map[uuid.UUID]bool{}
	for _, ch := range append(page1, page2...) {
		if combined[ch.ID] {
			t.Fatalf("chat %s appeared on more than one page", ch.ID)
		}
		combined[ch.ID] = true
	}
	if len(combined) != 3 {
		t.Fatalf("combined pages covered %d distinct chats, want 3", len(combined))
	}
}

// TestListChatsForOrg_AllViewIsUnionOfInboxAndCampaign covers the "All"
// requirement directly: one ordinary chat a campaign never touched, one
// cold-send campaign-only chat nobody has replied to yet — Inbox shows only
// the former, Campaign shows only the latter, All shows both exactly once.
func TestListChatsForOrg_AllViewIsUnionOfInboxAndCampaign(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	allowUnthrottledSends(t, st, ctx, acctID)

	normalRes, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: acctID, PhoneJID: "77044444444@s.whatsapp.net", RemoteJID: "77044444444@s.whatsapp.net",
		PhoneNumber: "77044444444", Direction: "in", SenderKind: "contact",
		ExternalMessageID: uuid.NewString(), MessageKind: "conversation", Body: "hi",
	})
	if err != nil {
		t.Fatalf("UpsertInbound: %v", err)
	}

	c := mustCreateCampaign(t, st, ctx, orgID, acctID, userID, "Cold Campaign", "Hi!")
	if err := st.ReplaceCampaignRecipients(ctx, c.ID, []store.CampaignRecipientInput{
		{NormalizedIdentity: "77055555555", Name: "ColdOnly"},
	}); err != nil {
		t.Fatalf("ReplaceCampaignRecipients: %v", err)
	}
	startCampaign(t, st, ctx, c.ID, userID)
	coldChatID, _, err := st.FindOrCreateChat(ctx, acctID, "77055555555@s.whatsapp.net", "77055555555")
	if err != nil {
		t.Fatalf("FindOrCreateChat: %v", err)
	}
	if err := st.MarkChatCampaignOnly(ctx, "whatsapp", coldChatID); err != nil {
		t.Fatalf("MarkChatCampaignOnly: %v", err)
	}
	msgID, err := st.InsertCampaignOutbound(ctx, "whatsapp", coldChatID, acctID, "Hi!", "Hi!")
	if err != nil {
		t.Fatalf("InsertCampaignOutbound: %v", err)
	}
	claimAndFinalize(t, st, ctx, acctID, time.Now(), purecampaign.RecipientSent,
		uuid.NullUUID{UUID: coldChatID, Valid: true}, uuid.NullUUID{UUID: msgID, Valid: true})

	inbox, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, Limit: 50})
	if err != nil || total != 1 || len(inbox) != 1 || inbox[0].ID != normalRes.ChatID {
		t.Fatalf("inbox view = %+v (total=%d, err=%v), want exactly [%s]", inbox, total, err, normalRes.ChatID)
	}
	campaignView, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, View: "campaign", Limit: 50})
	if err != nil || total != 1 || len(campaignView) != 1 || campaignView[0].ID != coldChatID {
		t.Fatalf("campaign view = %+v (total=%d, err=%v), want exactly [%s]", campaignView, total, err, coldChatID)
	}
	allView, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, View: "all", Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg (all): %v", err)
	}
	gotIDs := map[uuid.UUID]bool{}
	for _, ch := range allView {
		gotIDs[ch.ID] = true
	}
	if total != 2 || len(allView) != 2 || !gotIDs[normalRes.ChatID] || !gotIDs[coldChatID] {
		t.Fatalf("all view = %+v (total=%d), want exactly [%s, %s]", allView, total, normalRes.ChatID, coldChatID)
	}
}
