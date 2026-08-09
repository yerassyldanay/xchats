package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

func (h *metaHarness) seedInstagramAccount(igUserID string) store.ChannelAccount {
	h.t.Helper()
	acct, err := h.store.ClaimChannelAccount(context.Background(), store.ChannelAccountClaim{
		ID:                config.ChannelAccountID(config.InstagramOwnerRef(igUserID)),
		OrganizationID:    h.orgID,
		Channel:           "instagram",
		ExternalAccountID: igUserID,
		DisplayName:       "webhook test ig",
	})
	if err != nil {
		h.t.Fatalf("seed instagram account: %v", err)
	}
	return acct
}

func TestInstagramWebhookRejectsBadSignature(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	body := []byte(`{"object":"instagram","entry":[]}`)
	resp := h.postRaw("/meta/api/v1/webhook/instagram", "application/json", body, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("wrong-secret", body),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestInstagramWebhookUnknownAccountAcks200(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	body := []byte(`{"object":"instagram","entry":[{"id":"no-such-account","time":1723104000000,"messaging":[
		{"sender":{"id":"1"},"recipient":{"id":"no-such-account"},"timestamp":1723104000000,"message":{"mid":"m1","text":"hi"}}
	]}]}`)
	resp := h.postRaw("/meta/api/v1/webhook/instagram", "application/json", body, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", body),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestInstagramWebhookStoresInboundMessage(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	acct := h.seedInstagramAccount("178414000010")
	h.graphHandler = func(w http.ResponseWriter, r *http.Request) {
		// The webhook has no App-Secret-authenticated token available for a
		// never-before-seen contact's Profile lookup (only a live account
		// token would work, and none was ever saved here) — so the handler
		// must tolerate this failing rather than blocking ingest.
		http.Error(w, `{"error":{"message":"no permission","code":10}}`, http.StatusBadRequest)
	}

	body := []byte(`{"object":"instagram","entry":[{"id":"178414000010","time":1723104000000,"messaging":[
		{"sender":{"id":"999"},"recipient":{"id":"178414000010"},"timestamp":1723104000000,"message":{"mid":"ig-mid-1","text":"Здравствуйте"}}
	]}]}`)
	resp := h.postRaw("/meta/api/v1/webhook/instagram", "application/json", body, map[string]string{
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

func TestInstagramWebhookEchoDoesNotDuplicateTheChat(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	acct := h.seedInstagramAccount("178414000011")
	h.graphHandler = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"message":"no token","code":10}}`))
	}

	inboundBody := []byte(`{"object":"instagram","entry":[{"id":"178414000011","time":1723104000000,"messaging":[
		{"sender":{"id":"999"},"recipient":{"id":"178414000011"},"timestamp":1723104000000,"message":{"mid":"ig-mid-2","text":"вопрос"}}
	]}]}`)
	resp1 := h.postRaw("/meta/api/v1/webhook/instagram", "application/json", inboundBody, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", inboundBody),
	})
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("inbound status = %d", resp1.StatusCode)
	}

	echoBody := []byte(`{"object":"instagram","entry":[{"id":"178414000011","time":1723104060000,"messaging":[
		{"sender":{"id":"178414000011"},"recipient":{"id":"999"},"timestamp":1723104060000,"message":{"mid":"ig-mid-3","text":"ответ","is_echo":true}}
	]}]}`)
	resp2 := h.postRaw("/meta/api/v1/webhook/instagram", "application/json", echoBody, map[string]string{
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

func TestInstagramWebhookSkipsProfileLookupForAlreadyKnownContact(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	acct := h.seedInstagramAccount("178414000020")
	if err := h.store.SetChannelCredentials(context.Background(), store.ChannelCredentialsWrite{
		AccountID: acct.ID, Secret: "ig-live-token", TokenKind: "long_lived_user_token",
	}); err != nil {
		t.Fatalf("SetChannelCredentials: %v", err)
	}
	profileCalls := 0
	h.graphHandler = func(w http.ResponseWriter, r *http.Request) {
		profileCalls++
		_, _ = w.Write([]byte(`{"name":"Клиент Тест"}`))
	}

	firstBody := []byte(`{"object":"instagram","entry":[{"id":"178414000020","time":1723104000000,"messaging":[
		{"sender":{"id":"777"},"recipient":{"id":"178414000020"},"timestamp":1723104000000,"message":{"mid":"ig-mid-known-1","text":"привет"}}
	]}]}`)
	resp1 := h.postRaw("/meta/api/v1/webhook/instagram", "application/json", firstBody, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", firstBody),
	})
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d", resp1.StatusCode)
	}
	if profileCalls != 1 {
		t.Fatalf("profileCalls after a never-before-seen contact's message = %d, want 1 (a live lookup)", profileCalls)
	}

	// A second message from the SAME contact: the display name is now on
	// file, so this must NOT place a second live Profile call.
	secondBody := []byte(`{"object":"instagram","entry":[{"id":"178414000020","time":1723104060000,"messaging":[
		{"sender":{"id":"777"},"recipient":{"id":"178414000020"},"timestamp":1723104060000,"message":{"mid":"ig-mid-known-2","text":"снова я"}}
	]}]}`)
	resp2 := h.postRaw("/meta/api/v1/webhook/instagram", "application/json", secondBody, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", secondBody),
	})
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d", resp2.StatusCode)
	}
	if profileCalls != 1 {
		t.Fatalf("profileCalls after a second message from the same contact = %d, want still 1 (no repeat lookup)", profileCalls)
	}

	chats, _, err := h.store.ListChatsForOrg(context.Background(), store.ChatFilter{OrgID: h.orgID, Limit: 20})
	if err != nil {
		t.Fatalf("ListChatsForOrg: %v", err)
	}
	for _, c := range chats {
		if c.AccountID == acct.ID && c.LastMessagePreview != "снова я" {
			t.Fatalf("preview = %q, want the second message", c.LastMessagePreview)
		}
	}
}

func TestInstagramWebhookIgnorableEventAcks200(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	h.seedInstagramAccount("178414000012")
	body := []byte(`{"object":"instagram","entry":[{"id":"178414000012","time":1723104000000,"messaging":[
		{"sender":{"id":"999"},"recipient":{"id":"178414000012"},"timestamp":1723104000000}
	]}]}`)
	resp := h.postRaw("/meta/api/v1/webhook/instagram", "application/json", body, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", body),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
