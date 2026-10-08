package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// The inbox search lowers the pattern in Go, so every column it compares must be lowered too:
// SQLite's LIKE ignores ASCII case but PostgreSQL's does not, and a column that is not lowered
// would be found on one engine and not on the other.
func TestChatSearchIgnoresCaseInEveryColumn(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)

	org, err := st.SeedOrganization(ctx, t.Name())
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	ownerJID := "searchowner@s.whatsapp.net"
	accountID := config.AccountID(ownerJID)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: accountID, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true}, DisplayName: "WhatsApp",
		ExternalAccountRef: ownerJID, ExternalHandle: "77000000000", ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	// external_contact_ref is the contact's JID, which can carry capitals; the display name and
	// phone number do not match the queries below, so only that column can find the chat.
	if _, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: "JohnSmith@s.whatsapp.net", RemoteJID: "JohnSmith@s.whatsapp.net",
		PhoneNumber: "77012223344", PushName: "Zed", Direction: "in", SenderKind: "contact",
		ExternalMessageID: "m-1", MessageKind: "conversation", Body: "hi", Preview: "hi",
		Source: "live_webhook", MessageTS: time.Now(),
	}); err != nil {
		t.Fatalf("inbound: %v", err)
	}

	for _, q := range []string{"johnsmith", "JOHNSMITH", "JohnSmith", "ohnsm"} {
		chats, total, err := st.ListChatsForOrg(ctx, store.ChatFilter{OrgID: org.ID, Query: q, Limit: 10})
		if err != nil {
			t.Fatalf("ListChatsForOrg(%q): %v", q, err)
		}
		if total != 1 || len(chats) != 1 {
			t.Errorf("searching %q found %d chats (total %d), want 1: external_contact_ref was compared without lower()", q, len(chats), total)
		}
	}
}
