package httpapi_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/meta"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

func hmacSHA256Header(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func (h *metaHarness) seedWhatsAppCloudAccount(phoneNumberID, wabaID string) store.ChannelAccount {
	h.t.Helper()
	providerMeta, _ := json.Marshal(map[string]string{"waba_id": wabaID})
	acct, err := h.store.ClaimChannelAccount(context.Background(), store.ChannelAccountClaim{
		ID:                config.ChannelAccountID(config.WhatsAppCloudOwnerRef(phoneNumberID)),
		OrganizationID:    h.orgID,
		Channel:           "whatsapp_cloud",
		ExternalAccountID: phoneNumberID,
		DisplayName:       "webhook test number",
		ProviderMeta:      providerMeta,
	})
	if err != nil {
		h.t.Fatalf("seed whatsapp_cloud account: %v", err)
	}
	return acct
}

func TestMetaWebhookVerifySucceedsWithCorrectToken(t *testing.T) {
	h := newMetaHarness(t)
	token := meta.DeriveVerifyToken(h.cfg.SessionSecret)
	resp, err := h.client.Get(h.srv.URL + "/meta/api/v1/webhook/whatsapp?hub.mode=subscribe&hub.verify_token=" + token + "&hub.challenge=12345")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := make([]byte, 16)
	n, _ := resp.Body.Read(body)
	if got := string(body[:n]); got != "12345" {
		t.Fatalf("body = %q, want the echoed challenge 12345", got)
	}
}

func TestMetaWebhookVerifyRejectsWrongToken(t *testing.T) {
	h := newMetaHarness(t)
	resp, err := h.client.Get(h.srv.URL + "/meta/api/v1/webhook/whatsapp?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestWhatsAppCloudWebhookRejectsWithoutAppSecret(t *testing.T) {
	h := newMetaHarness(t)
	body := []byte(`{"object":"whatsapp_business_account","entry":[]}`)
	resp := h.postRaw("/meta/api/v1/webhook/whatsapp", "application/json", body, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("whatever", body),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (no App Secret configured)", resp.StatusCode)
	}
}

func TestWhatsAppCloudWebhookRejectsBadSignature(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	body := []byte(`{"object":"whatsapp_business_account","entry":[]}`)
	resp := h.postRaw("/meta/api/v1/webhook/whatsapp", "application/json", body, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("the-wrong-secret", body),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (bad signature)", resp.StatusCode)
	}
}

func TestWhatsAppCloudWebhookMalformedBodyAcks200(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	body := []byte(`not json at all`)
	resp := h.postRaw("/meta/api/v1/webhook/whatsapp", "application/json", body, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", body),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (retrying a malformed body would never help)", resp.StatusCode)
	}
}

func TestWhatsAppCloudWebhookUnknownAccountAcks200(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	body := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"waba-999","changes":[{"field":"messages","value":{
		"messaging_product":"whatsapp","metadata":{"display_phone_number":"1","phone_number_id":"no-such-number"},
		"contacts":[{"profile":{"name":"X"},"wa_id":"1"}],
		"messages":[{"from":"1","id":"wamid.unknown","timestamp":"1723104000","type":"text","text":{"body":"hi"}}]
	}}]}]}`)
	resp := h.postRaw("/meta/api/v1/webhook/whatsapp", "application/json", body, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", body),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (unknown account is acked, never retried)", resp.StatusCode)
	}
}

func TestWhatsAppCloudWebhookStoresInboundMessage(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	acct := h.seedWhatsAppCloudAccount("15550001111", "waba-1000")

	body := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"waba-1000","changes":[{"field":"messages","value":{
		"messaging_product":"whatsapp","metadata":{"display_phone_number":"15550001111","phone_number_id":"15550001111"},
		"contacts":[{"profile":{"name":"Клиент Тест"},"wa_id":"77011234567"}],
		"messages":[{"from":"77011234567","id":"wamid.stored1","timestamp":"1723104000","type":"text","text":{"body":"Здравствуйте"}}]
	}}]}]}`)
	resp := h.postRaw("/meta/api/v1/webhook/whatsapp", "application/json", body, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", body),
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	chats, _, err := h.store.ListChatsForOrg(context.Background(), store.ChatFilter{OrgID: h.orgID, Limit: 20})
	if err != nil {
		t.Fatalf("ListChats: %v", err)
	}
	if len(chats) != 1 || chats[0].AccountID != acct.ID {
		t.Fatalf("chats = %+v, want exactly one chat on account %s", chats, acct.ID)
	}
	if chats[0].LastMessagePreview != "Здравствуйте" {
		t.Fatalf("preview = %q", chats[0].LastMessagePreview)
	}

	// Redelivering the SAME wamid must not create a second message.
	resp2 := h.postRaw("/meta/api/v1/webhook/whatsapp", "application/json", body, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", body),
	})
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("redelivery status = %d, want 200", resp2.StatusCode)
	}
	msgs, _, err := h.store.MessagesForChat(context.Background(), chats[0].ID, time.Time{}, 20)
	if err != nil {
		t.Fatalf("MessagesForChat: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages = %d after a redelivered wamid, want 1 (deduped)", len(msgs))
	}
}

func TestWhatsAppCloudWebhookAppliesDeliveryStatus(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	acct := h.seedWhatsAppCloudAccount("15550001112", "waba-1001")

	// Seed a chat + an outbound message with a known external id — the shape
	// a real send leaves behind.
	ctx := context.Background()
	inboundBody := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"waba-1001","changes":[{"field":"messages","value":{
		"messaging_product":"whatsapp","metadata":{"display_phone_number":"15550001112","phone_number_id":"15550001112"},
		"contacts":[{"profile":{"name":"X"},"wa_id":"77011234567"}],
		"messages":[{"from":"77011234567","id":"wamid.seed","timestamp":"1723104000","type":"text","text":{"body":"hi"}}]
	}}]}]}`)
	resp := h.postRaw("/meta/api/v1/webhook/whatsapp", "application/json", inboundBody, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", inboundBody),
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("seed inbound status = %d", resp.StatusCode)
	}
	chats, _, err := h.store.ListChatsForOrg(ctx, store.ChatFilter{OrgID: h.orgID, Limit: 20})
	if err != nil || len(chats) != 1 {
		t.Fatalf("ListChatsForOrg: %v (chats=%+v)", err, chats)
	}
	msgID, err := h.store.InsertOutbound(ctx, "whatsapp_cloud", chats[0].ID, acct.ID, "user",
		uuid.NullUUID{}, "conversation", "reply", "reply")
	if err != nil {
		t.Fatalf("InsertOutbound: %v", err)
	}
	if err := h.store.StampOutboundSent(ctx, "whatsapp_cloud", msgID, "wamid.status1"); err != nil {
		t.Fatalf("StampOutboundSent: %v", err)
	}

	statusBody := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"waba-1001","changes":[{"field":"messages","value":{
		"messaging_product":"whatsapp","metadata":{"display_phone_number":"15550001112","phone_number_id":"15550001112"},
		"statuses":[{"id":"wamid.status1","status":"delivered","timestamp":"1723104100","recipient_id":"77011234567"}]
	}}]}]}`)
	resp2 := h.postRaw("/meta/api/v1/webhook/whatsapp", "application/json", statusBody, map[string]string{
		"X-Hub-Signature-256": hmacSHA256Header("app-secret-1", statusBody),
	})
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("status delivery webhook status = %d, want 200", resp2.StatusCode)
	}

	msg, err := h.store.MessageByID(ctx, msgID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if msg.DeliveryState != "delivered" {
		t.Fatalf("delivery_state = %q, want delivered", msg.DeliveryState)
	}
}
