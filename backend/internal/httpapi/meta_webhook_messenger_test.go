package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

func (h *metaHarness) seedMessengerAccount(pageID string) store.ChannelAccount {
	h.t.Helper()
	acct, err := h.store.ClaimChannelAccount(context.Background(), store.ChannelAccountClaim{
		ID:                config.ChannelAccountID(config.MessengerOwnerRef(pageID)),
		OrganizationID:    h.orgID,
		Channel:           "messenger",
		ExternalAccountID: pageID,
		DisplayName:       "webhook test page",
	})
	if err != nil {
		h.t.Fatalf("seed messenger account: %v", err)
	}
	return acct
}

func TestMessengerWebhookRejectsBadSignature(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	body := []byte(`{"object":"page","entry":[]}`)
	resp := h.postRaw("/meta/api/v1/webhook/messenger", "application/json", body, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("wrong-secret", body),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestMessengerWebhookUnknownAccountAcks200(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	body := []byte(`{"object":"page","entry":[{"id":"no-such-page","time":1723104000000,"messaging":[
		{"sender":{"id":"1"},"recipient":{"id":"no-such-page"},"timestamp":1723104000000,"message":{"mid":"m1","text":"hi"}}
	]}]}`)
	resp := h.postRaw("/meta/api/v1/webhook/messenger", "application/json", body, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", body),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestMessengerWebhookStoresInboundMessage(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	acct := h.seedMessengerAccount("998877001")
	h.graphHandler = func(w http.ResponseWriter, r *http.Request) {
		// Same reasoning as the Instagram twin of this test: no live account
		// token was ever saved, so the Profile lookup for a never-before-seen
		// PSID must fail without blocking ingest.
		http.Error(w, `{"error":{"message":"no permission","code":10}}`, http.StatusBadRequest)
	}

	body := []byte(`{"object":"page","entry":[{"id":"998877001","time":1723104000000,"messaging":[
		{"sender":{"id":"999"},"recipient":{"id":"998877001"},"timestamp":1723104000000,"message":{"mid":"fb-mid-1","text":"Здравствуйте"}}
	]}]}`)
	resp := h.postRaw("/meta/api/v1/webhook/messenger", "application/json", body, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", body),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	chats, _, err := h.store.ListChatsForOrg(context.Background(), store.ChatFilter{OrgID: h.orgID, Limit: 20})
	if err != nil {
		t.Fatalf("ListChatsForOrg: %v", err)
	}
	found := false
	for _, c := range chats {
		if c.AccountID == acct.ID {
			found = true
			if c.LastMessagePreview != "Здравствуйте" {
				t.Fatalf("preview = %q", c.LastMessagePreview)
			}
		}
	}
	if !found {
		t.Fatalf("no chat found for account %s among %+v", acct.ID, chats)
	}
}

func TestMessengerWebhookEchoDoesNotDuplicateTheChat(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	acct := h.seedMessengerAccount("998877002")
	h.graphHandler = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"message":"no token","code":10}}`))
	}

	inboundBody := []byte(`{"object":"page","entry":[{"id":"998877002","time":1723104000000,"messaging":[
		{"sender":{"id":"999"},"recipient":{"id":"998877002"},"timestamp":1723104000000,"message":{"mid":"fb-mid-2","text":"вопрос"}}
	]}]}`)
	resp1 := h.postRaw("/meta/api/v1/webhook/messenger", "application/json", inboundBody, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", inboundBody),
	})
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("inbound status = %d", resp1.StatusCode)
	}

	echoBody := []byte(`{"object":"page","entry":[{"id":"998877002","time":1723104060000,"messaging":[
		{"sender":{"id":"998877002"},"recipient":{"id":"999"},"timestamp":1723104060000,"message":{"mid":"fb-mid-3","text":"ответ","is_echo":true}}
	]}]}`)
	resp2 := h.postRaw("/meta/api/v1/webhook/messenger", "application/json", echoBody, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", echoBody),
	})
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("echo status = %d", resp2.StatusCode)
	}

	chats, _, err := h.store.ListChatsForOrg(context.Background(), store.ChatFilter{OrgID: h.orgID, Limit: 20})
	if err != nil {
		t.Fatalf("ListChatsForOrg: %v", err)
	}
	count := 0
	for _, c := range chats {
		if c.AccountID == acct.ID {
			count++
			if c.LastMessagePreview != "ответ" {
				t.Fatalf("preview after echo = %q, want the operator's reply", c.LastMessagePreview)
			}
		}
	}
	if count != 1 {
		t.Fatalf("chats for account %s = %d, want 1 (the echo must land in the SAME thread as the customer's own message)", acct.ID, count)
	}

	msgs, _, err := h.store.MessagesForChat(context.Background(), chats[0].ID, time.Time{}, 20)
	if err != nil {
		t.Fatalf("MessagesForChat: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2 (the inbound + the echoed reply)", len(msgs))
	}
}

func TestMessengerWebhookIgnorableEventAcks200(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	h.seedMessengerAccount("998877003")
	body := []byte(`{"object":"page","entry":[{"id":"998877003","time":1723104000000,"messaging":[
		{"sender":{"id":"999"},"recipient":{"id":"998877003"},"timestamp":1723104000000}
	]}]}`)
	resp := h.postRaw("/meta/api/v1/webhook/messenger", "application/json", body, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", body),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
