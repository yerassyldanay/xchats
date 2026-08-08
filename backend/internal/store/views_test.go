package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

const viewsEncKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// seedFourChannels builds one organization with a WhatsApp chat, a simulator
// chat, a Telegram chat and a generic-channel-core chat, and returns their
// chat ids. It is the fixture for the view-parity assertions: everything the
// inbox reads now comes through a UNION over three very different transport
// schemas, and the WhatsApp leg is deliberately UNFILTERED because the
// simulator lives inside it.
//
// The fourth chat is seeded on channel "whatsapp_cloud" — one representative
// of the generic channel_* core, not one seed per Meta channel. Instagram,
// Messenger and WhatsApp Cloud all read and write through the IDENTICAL
// channel_accounts/channel_chats/channel_messages tables and store methods
// (see channels.go), so a second or third Meta-channel seed here would
// exercise exactly the same SQL this one already does — the channel value
// itself is just a string these queries never branch on.
func seedFourChannels(t *testing.T, st *store.Store) (orgID, waChat, simChat, tgChat, metaChat uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	box, err := secretbox.FromEnvValue(viewsEncKey)
	if err != nil {
		t.Fatalf("secretbox: %v", err)
	}
	st.UseCredentialsBox(box)

	org, err := st.SeedOrganization(ctx, "views-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	orgID = org.ID

	// WhatsApp.
	const ownerJID = "77011111111@s.whatsapp.net"
	waAccount := config.AccountID(ownerJID)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: waAccount, OrganizationID: uuid.NullUUID{UUID: orgID, Valid: true},
		DisplayName: "WhatsApp", ExternalAccountRef: ownerJID,
		ExternalHandle: "77011111111", ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed wa account: %v", err)
	}
	waRes, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: waAccount, PhoneJID: "77000000000@s.whatsapp.net",
		RemoteJID: "77000000000@s.whatsapp.net", PhoneNumber: "77000000000",
		PushName: "WA Клиент", Direction: "in", SenderKind: "contact",
		ExternalMessageID: "WA1", MessageKind: "conversation", Body: "привет из whatsapp",
		Preview: "привет из whatsapp", Source: "live_webhook", MessageTS: time.Now(),
	})
	if err != nil {
		t.Fatalf("wa inbound: %v", err)
	}
	waChat = waRes.ChatID

	// Simulator — a wa_accounts row with channel='simulator'.
	simAccount, err := st.GetOrCreateSimulatorAccount(ctx, orgID)
	if err != nil {
		t.Fatalf("simulator account: %v", err)
	}
	simRes, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: simAccount.ID, PhoneJID: "sim-contact", RemoteJID: "sim-conv",
		PhoneNumber: "sim-contact", PushName: "Sim Клиент", Direction: "in", SenderKind: "contact",
		ExternalMessageID: uuid.NewString(), MessageKind: "conversation", Body: "привет из симулятора",
		Preview: "привет из симулятора", Source: "simulator", MessageTS: time.Now(),
	})
	if err != nil {
		t.Fatalf("sim inbound: %v", err)
	}
	simChat = simRes.ChatID

	// Telegram.
	const botID = int64(4242)
	tgAccount, err := st.ClaimTelegramAccount(ctx, store.TelegramClaim{
		ID:             config.ChannelAccountID(config.TelegramOwnerRef(botID)),
		OrganizationID: orgID,
		DisplayName:    "Telegram",
		BotID:          botID,
		BotUsername:    "views_bot",
		BotToken:       "4242:token",
	})
	if err != nil {
		t.Fatalf("claim telegram account: %v", err)
	}
	tgRes, err := st.IngestTelegramInbound(ctx, store.TgInbound{
		AccountID: tgAccount.ID, UpdateID: 1,
		TelegramChatID: 900900, ChatType: "private",
		TelegramUserID: 900900, Username: "tguser", FirstName: "TG", LastName: "Клиент",
		DisplayName: "TG Клиент", TelegramMessageID: 5,
		MessageKind: "conversation", Body: "привет из telegram", Preview: "привет из telegram",
		MessageTS: time.Now(),
	})
	if err != nil {
		t.Fatalf("telegram inbound: %v", err)
	}
	tgChat = tgRes.ChatID

	// Generic channel core — one representative Meta channel (see the doc
	// comment above for why only one).
	const externalAccountID = "15550001111"
	metaAccount := config.ChannelAccountID("whatsapp_cloud:phone:" + externalAccountID)
	if _, err := st.ClaimChannelAccount(ctx, store.ChannelAccountClaim{
		ID: metaAccount, OrganizationID: orgID, Channel: "whatsapp_cloud",
		ExternalAccountID: externalAccountID, DisplayName: "WhatsApp Cloud", Handle: "+15550001111",
	}); err != nil {
		t.Fatalf("claim channel account: %v", err)
	}
	metaRes, err := st.IngestChannelInbound(ctx, store.ChannelInbound{
		AccountID: metaAccount, ExternalContactID: "wa_cloud_customer",
		ContactHandle: "+15550002222", ContactDisplayName: "Cloud Клиент",
		ExternalThreadID: "wa_cloud_customer", Direction: "in", SenderKind: "contact",
		ExternalMessageID: "wamid.META1", MessageKind: "conversation", Body: "привет из whatsapp cloud",
		Preview: "привет из whatsapp cloud", Source: "live_webhook", MessageTS: time.Now(),
	})
	if err != nil {
		t.Fatalf("meta inbound: %v", err)
	}
	metaChat = metaRes.ChatID

	return orgID, waChat, simChat, tgChat, metaChat
}

// TestInboxViewsCarryEveryChannel is the Phase-A gate in test form: one org's
// inbox must contain every channel's conversations, each labelled, with the
// simulator NOT lost to a stray channel filter on the WhatsApp leg.
func TestInboxViewsCarryEveryChannel(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, waChat, simChat, tgChat, metaChat := seedFourChannels(t, st)

	chats, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, Limit: 50})
	if err != nil {
		t.Fatalf("ListChatsForOrg: %v", err)
	}
	if total != 4 || len(chats) != 4 {
		t.Fatalf("chats = %d (total %d), want 4 across whatsapp+simulator+telegram+whatsapp_cloud", len(chats), total)
	}
	byID := map[uuid.UUID]store.Chat{}
	for _, c := range chats {
		byID[c.ID] = c
	}
	for _, want := range []struct {
		id      uuid.UUID
		channel string
	}{{waChat, "whatsapp"}, {simChat, "simulator"}, {tgChat, "telegram"}, {metaChat, "whatsapp_cloud"}} {
		got, present := byID[want.id]
		if !present {
			t.Fatalf("chat %s (%s) is missing from the inbox", want.id, want.channel)
		}
		if got.Channel != want.channel {
			t.Fatalf("chat %s channel = %q, want %q", want.id, got.Channel, want.channel)
		}
	}

	// The Telegram leg's neutral columns carry the provider's own identities.
	tg := byID[tgChat]
	if tg.ExternalConversationRef != "900900" {
		t.Fatalf("telegram external_conversation_ref = %q, want the chat id", tg.ExternalConversationRef)
	}
	if tg.Contact.ExternalContactRef != "900900" {
		t.Fatalf("telegram external_contact_ref = %q, want the user id", tg.Contact.ExternalContactRef)
	}
	if tg.Contact.DisplayName != "TG Клиент" {
		t.Fatalf("telegram contact display name = %q", tg.Contact.DisplayName)
	}

	// The generic channel core's neutral columns carry the provider's own
	// identities too, and (unlike wa_*/tg_*) a real last_inbound_at.
	meta := byID[metaChat]
	if meta.ExternalConversationRef != "wa_cloud_customer" {
		t.Fatalf("whatsapp_cloud external_conversation_ref = %q", meta.ExternalConversationRef)
	}
	if meta.Contact.ExternalContactRef != "wa_cloud_customer" {
		t.Fatalf("whatsapp_cloud external_contact_ref = %q", meta.Contact.ExternalContactRef)
	}
	if meta.Contact.DisplayName != "Cloud Клиент" {
		t.Fatalf("whatsapp_cloud contact display name = %q", meta.Contact.DisplayName)
	}
	if meta.LastInboundAt == nil {
		t.Fatal("whatsapp_cloud last_inbound_at is nil after an inbound message")
	}
	if tg.LastInboundAt != nil {
		t.Fatalf("telegram last_inbound_at = %v, want nil (no service window on this leg)", tg.LastInboundAt)
	}

	// The single-account filter works on any leg.
	only, _, err := st.ListChatsForOrg(ctx, store.ChatFilter{
		OrgID: orgID, AccountID: uuid.NullUUID{UUID: tg.AccountID, Valid: true}, Limit: 50,
	})
	if err != nil {
		t.Fatalf("filtered list: %v", err)
	}
	if len(only) != 1 || only[0].ID != tgChat {
		t.Fatalf("account filter returned %d chats, want just the telegram one", len(only))
	}
}

func TestChatByIDAndOrgGuardSpanChannels(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, waChat, _, tgChat, metaChat := seedFourChannels(t, st)

	for _, id := range []uuid.UUID{waChat, tgChat, metaChat} {
		if _, err := st.ChatByID(ctx, id); err != nil {
			t.Fatalf("ChatByID(%s): %v", id, err)
		}
		if _, err := st.ChatByIDForOrg(ctx, id, orgID); err != nil {
			t.Fatalf("ChatByIDForOrg(%s): %v", id, err)
		}
	}

	// A different organization must not see any of the chats.
	other, err := st.SeedOrganization(ctx, "views-other")
	if err != nil {
		t.Fatalf("seed other org: %v", err)
	}
	otherOrg := other.ID
	for _, id := range []uuid.UUID{waChat, tgChat, metaChat} {
		if _, err := st.ChatByIDForOrg(ctx, id, otherOrg); err != store.ErrNotFound {
			t.Fatalf("cross-org read of %s returned %v, want ErrNotFound", id, err)
		}
	}
	if chats, _, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: otherOrg, Limit: 50}); err != nil || len(chats) != 0 {
		t.Fatalf("a foreign org sees %d chats (err %v)", len(chats), err)
	}
}

// A soft-deleted account's chats drop out of the inbox on EVERY channel — the
// view carries the owning account's deleted_at for exactly this.
func TestSoftDeletedAccountsHideTheirChatsOnEveryChannel(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, _, _, tgChat, metaChat := seedFourChannels(t, st)

	tg, err := st.ChatByID(ctx, tgChat)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}
	if err := st.ConfirmTelegramDisconnect(ctx, tg.AccountID); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	meta, err := st.ChatByID(ctx, metaChat)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}
	if err := st.ConfirmChannelDisconnect(ctx, meta.AccountID); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	chats, _, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, Limit: 50})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, c := range chats {
		if c.ID == tgChat {
			t.Fatal("a disconnected Telegram account's chat is still in the inbox")
		}
		if c.ID == metaChat {
			t.Fatal("a disconnected channel-core account's chat is still in the inbox")
		}
	}
	if len(chats) != 2 {
		t.Fatalf("chats = %d, want the two wa_* ones", len(chats))
	}
	if _, err := st.ChatByIDForOrg(ctx, tgChat, orgID); err != store.ErrNotFound {
		t.Fatalf("the org guard still resolves a disconnected Telegram account's chat: %v", err)
	}
	if _, err := st.ChatByIDForOrg(ctx, metaChat, orgID); err != store.ErrNotFound {
		t.Fatalf("the org guard still resolves a disconnected channel-core account's chat: %v", err)
	}
}

func TestAccountListingsSplitNeutralFromWhatsAppOnly(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, _, _, tgChat, metaChat := seedFourChannels(t, st)

	all, err := st.ListAccountsForOrg(ctx, orgID)
	if err != nil {
		t.Fatalf("ListAccountsForOrg: %v", err)
	}
	channels := map[string]int{}
	for _, a := range all {
		channels[a.Channel]++
	}
	if channels["whatsapp"] != 1 || channels["simulator"] != 1 || channels["telegram"] != 1 || channels["whatsapp_cloud"] != 1 {
		t.Fatalf("neutral listing channels = %v, want one of each", channels)
	}

	// The WhatsApp-only listing covers the wa_* gateway (WhatsApp + simulator)
	// and must never surface a bot or a Meta channel account: /whatsapp-accounts
	// drives a QR lifecycle neither has.
	wa, err := st.ListWaAccountsForOrg(ctx, orgID)
	if err != nil {
		t.Fatalf("ListWaAccountsForOrg: %v", err)
	}
	if len(wa) != 2 {
		t.Fatalf("wa listing = %d accounts, want 2 (whatsapp + simulator)", len(wa))
	}
	for _, a := range wa {
		if a.Channel == "telegram" || a.Channel == "whatsapp_cloud" {
			t.Fatalf("a %s account appeared in the WhatsApp-only listing", a.Channel)
		}
	}

	// AccountByID resolves a Telegram account with its health fields.
	tg, err := st.ChatByID(ctx, tgChat)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}
	acct, err := st.AccountByID(ctx, tg.AccountID)
	if err != nil {
		t.Fatalf("AccountByID(telegram): %v", err)
	}
	if acct.Channel != "telegram" {
		t.Fatalf("channel = %q", acct.Channel)
	}
	if acct.ExternalHandle != "@views_bot" {
		t.Fatalf("external_handle = %q, want @views_bot", acct.ExternalHandle)
	}
	if acct.ExternalAccountRef != "telegram:bot:4242" {
		t.Fatalf("external_account_ref = %q", acct.ExternalAccountRef)
	}

	// AccountByID resolves a channel-core account with ITS health fields too
	// (the generic core registers a webhook, same as Telegram).
	meta, err := st.ChatByID(ctx, metaChat)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}
	metaAcct, err := st.AccountByID(ctx, meta.AccountID)
	if err != nil {
		t.Fatalf("AccountByID(whatsapp_cloud): %v", err)
	}
	if metaAcct.Channel != "whatsapp_cloud" {
		t.Fatalf("channel = %q", metaAcct.Channel)
	}
	if metaAcct.ExternalHandle != "+15550001111" {
		t.Fatalf("external_handle = %q, want +15550001111", metaAcct.ExternalHandle)
	}
	if metaAcct.ExternalAccountRef != "15550001111" {
		t.Fatalf("external_account_ref = %q, want 15550001111", metaAcct.ExternalAccountRef)
	}
}

func TestMessagesAndDraftsAreChannelAware(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, waChat, _, tgChat, metaChat := seedFourChannels(t, st)

	for _, tc := range []struct {
		chat    uuid.UUID
		channel string
		body    string
	}{
		{waChat, "whatsapp", "привет из whatsapp"},
		{tgChat, "telegram", "привет из telegram"},
		{metaChat, "whatsapp_cloud", "привет из whatsapp cloud"},
	} {
		msgs, _, err := st.MessagesForChat(ctx, tc.chat, time.Time{}, 10)
		if err != nil {
			t.Fatalf("MessagesForChat(%s): %v", tc.channel, err)
		}
		if len(msgs) != 1 || msgs[0].Body != tc.body {
			t.Fatalf("%s messages = %+v", tc.channel, msgs)
		}
		if msgs[0].Channel != tc.channel {
			t.Fatalf("message channel = %q, want %q", msgs[0].Channel, tc.channel)
		}
		if _, err := st.MessageByID(ctx, msgs[0].ID); err != nil {
			t.Fatalf("MessageByID(%s): %v", tc.channel, err)
		}

		trigger, err := st.LatestInboundMessageID(ctx, tc.chat)
		if err != nil || !trigger.Valid {
			t.Fatalf("LatestInboundMessageID(%s) = %v, %v", tc.channel, trigger, err)
		}

		// ai_drafts is channel-neutral now: the same table holds both, stamped.
		drafts, err := st.WriteDraftSet(ctx, tc.channel, tc.chat, trigger, []store.DraftOption{{
			Ordinal: 1, Text: "черновик", ReplyLanguage: "ru",
		}})
		if err != nil {
			t.Fatalf("WriteDraftSet(%s): %v", tc.channel, err)
		}
		if len(drafts) != 1 || drafts[0].Channel != tc.channel {
			t.Fatalf("draft = %+v, want channel %q", drafts, tc.channel)
		}
		has, err := st.HasDraftForTrigger(ctx, trigger.UUID)
		if err != nil || !has {
			t.Fatalf("HasDraftForTrigger(%s) = %v, %v", tc.channel, has, err)
		}
	}
}

func TestMarkChatReadDispatchesByChannel(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, waChat, _, tgChat, metaChat := seedFourChannels(t, st)

	for _, id := range []uuid.UUID{waChat, tgChat, metaChat} {
		before, err := st.ChatByID(ctx, id)
		if err != nil {
			t.Fatalf("ChatByID: %v", err)
		}
		if before.UnreadCount == 0 {
			t.Fatalf("chat %s (%s) starts read; the fixture is wrong", id, before.Channel)
		}
		after, err := st.MarkChatRead(ctx, id)
		if err != nil {
			t.Fatalf("MarkChatRead(%s): %v", before.Channel, err)
		}
		if after.UnreadCount != 0 {
			t.Fatalf("%s unread = %d after MarkChatRead", before.Channel, after.UnreadCount)
		}
		reread, err := st.ChatByID(ctx, id)
		if err != nil {
			t.Fatalf("re-read: %v", err)
		}
		if reread.UnreadCount != 0 {
			t.Fatalf("%s unread = %d in the database — the write went to the wrong table",
				before.Channel, reread.UnreadCount)
		}
	}
}

func TestAssignChatDispatchesByChannelAndCanClear(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, waChat, _, tgChat, metaChat := seedFourChannels(t, st)
	user, err := st.SeedUser(ctx, orgID, "assignee@example.com", "hash", "Assignee")
	if err != nil {
		t.Fatalf("SeedUser: %v", err)
	}

	for _, id := range []uuid.UUID{waChat, tgChat, metaChat} {
		assigned, err := st.AssignChat(ctx, id, uuid.NullUUID{UUID: user.ID, Valid: true})
		if err != nil {
			t.Fatalf("AssignChat(%s): %v", id, err)
		}
		if !assigned.AssigneeUserID.Valid || assigned.AssigneeUserID.UUID != user.ID {
			t.Fatalf("assignee = %v, want %s", assigned.AssigneeUserID, user.ID)
		}
		cleared, err := st.AssignChat(ctx, id, uuid.NullUUID{})
		if err != nil {
			t.Fatalf("clear AssignChat(%s): %v", id, err)
		}
		if cleared.AssigneeUserID.Valid {
			t.Fatalf("assignee still set after clear: %v", cleared.AssigneeUserID)
		}
	}
}

// A re-claim of the same bot by the same org must revive the row (and its
// history) rather than create a second account.
func TestClaimTelegramAccountRevivesTheSameRow(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, _, _, tgChat, _ := seedFourChannels(t, st)

	tg, err := st.ChatByID(ctx, tgChat)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}
	if err := st.ConfirmTelegramDisconnect(ctx, tg.AccountID); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if _, err := st.TelegramBotToken(ctx, tg.AccountID); err != store.ErrNotFound {
		t.Fatalf("the token survived a confirmed disconnect: %v", err)
	}

	again, err := st.ClaimTelegramAccount(ctx, store.TelegramClaim{
		ID:             config.ChannelAccountID(config.TelegramOwnerRef(4242)),
		OrganizationID: orgID,
		DisplayName:    "Telegram снова",
		BotID:          4242,
		BotUsername:    "views_bot",
		BotToken:       "4242:new-token",
	})
	if err != nil {
		t.Fatalf("re-claim: %v", err)
	}
	if again.ID != tg.AccountID {
		t.Fatalf("re-claim produced a new account %s, want %s", again.ID, tg.AccountID)
	}
	if again.DeletedAt != nil {
		t.Fatal("the revived account is still soft-deleted")
	}
	token, err := st.TelegramBotToken(ctx, again.ID)
	if err != nil || token != "4242:new-token" {
		t.Fatalf("token after re-claim = %q, %v", token, err)
	}
	chats, _, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, Limit: 50})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, c := range chats {
		if c.ID == tgChat {
			found = true
		}
	}
	if !found {
		t.Fatal("the revived account's history did not come back")
	}
}

// Without an encryption key, credential paths must fail loudly rather than
// storing a plaintext token.
func TestTelegramCredentialsRequireAnEncryptionKey(t *testing.T) {
	st, db := dbtest.Open(t)
	ctx := context.Background()

	org, err := st.SeedOrganization(ctx, "no-key-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	_, err = st.ClaimTelegramAccount(ctx, store.TelegramClaim{
		ID:             config.ChannelAccountID(config.TelegramOwnerRef(7)),
		OrganizationID: org.ID, BotID: 7, BotUsername: "b", BotToken: "7:tok",
	})
	if err != store.ErrNoCredentialsKey {
		t.Fatalf("claim without a key = %v, want ErrNoCredentialsKey", err)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM tg_accounts`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("a keyless claim wrote %d account rows", n)
	}
}

// A re-claim of the same external account by the same org must revive the
// row (and its history) rather than create a second account — the generic
// core's equivalent of TestClaimTelegramAccountRevivesTheSameRow.
func TestClaimChannelAccountRevivesTheSameRow(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, _, _, _, metaChat := seedFourChannels(t, st)

	meta, err := st.ChatByID(ctx, metaChat)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}
	if err := st.ConfirmChannelDisconnect(ctx, meta.AccountID); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if _, err := st.ChannelCredentialsSecret(ctx, meta.AccountID); err != store.ErrNotFound {
		t.Fatalf("the secret survived a confirmed disconnect: %v", err)
	}

	again, err := st.ClaimChannelAccount(ctx, store.ChannelAccountClaim{
		ID:                config.ChannelAccountID("whatsapp_cloud:phone:15550001111"),
		OrganizationID:    orgID,
		Channel:           "whatsapp_cloud",
		ExternalAccountID: "15550001111",
		DisplayName:       "WhatsApp Cloud снова",
		Handle:            "+15550001111",
	})
	if err != nil {
		t.Fatalf("re-claim: %v", err)
	}
	if again.ID != meta.AccountID {
		t.Fatalf("re-claim produced a new account %s, want %s", again.ID, meta.AccountID)
	}
	if again.DeletedAt != nil {
		t.Fatal("the revived account is still soft-deleted")
	}
	if err := st.SetChannelCredentials(ctx, store.ChannelCredentialsWrite{
		AccountID: again.ID, Secret: "new-token", TokenKind: "wa_business_token",
	}); err != nil {
		t.Fatalf("set credentials after re-claim: %v", err)
	}
	secret, err := st.ChannelCredentialsSecret(ctx, again.ID)
	if err != nil || secret != "new-token" {
		t.Fatalf("secret after re-claim = %q, %v", secret, err)
	}
	chats, _, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: orgID, Limit: 50})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, c := range chats {
		if c.ID == metaChat {
			found = true
		}
	}
	if !found {
		t.Fatal("the revived account's history did not come back")
	}

	// Claiming the SAME external account for a DIFFERENT org must fail rather
	// than silently steal its history.
	other, err := st.SeedOrganization(ctx, "views-other-claim")
	if err != nil {
		t.Fatalf("seed other org: %v", err)
	}
	if _, err := st.ClaimChannelAccount(ctx, store.ChannelAccountClaim{
		ID:                config.ChannelAccountID("whatsapp_cloud:phone:15550001111"),
		OrganizationID:    other.ID,
		Channel:           "whatsapp_cloud",
		ExternalAccountID: "15550001111",
		DisplayName:       "Steal attempt",
	}); err != store.ErrChannelAccountClaimed {
		t.Fatalf("cross-org re-claim = %v, want ErrChannelAccountClaimed", err)
	}
}

// Without an encryption key, credential paths must fail loudly rather than
// storing a plaintext secret — the generic core's equivalent of
// TestTelegramCredentialsRequireAnEncryptionKey.
func TestChannelCredentialsRequireAnEncryptionKey(t *testing.T) {
	st, db := dbtest.Open(t)
	ctx := context.Background()

	org, err := st.SeedOrganization(ctx, "no-key-org-meta")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	acct, err := st.ClaimChannelAccount(ctx, store.ChannelAccountClaim{
		ID:                config.ChannelAccountID("whatsapp_cloud:phone:70001"),
		OrganizationID:    org.ID,
		Channel:           "whatsapp_cloud",
		ExternalAccountID: "70001",
	})
	if err != nil {
		t.Fatalf("claim (account claim itself needs no key): %v", err)
	}
	if err := st.SetChannelCredentials(ctx, store.ChannelCredentialsWrite{
		AccountID: acct.ID, Secret: "tok",
	}); err != store.ErrNoCredentialsKey {
		t.Fatalf("set credentials without a key = %v, want ErrNoCredentialsKey", err)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM channel_credentials`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("a keyless credentials write wrote %d rows", n)
	}
}
