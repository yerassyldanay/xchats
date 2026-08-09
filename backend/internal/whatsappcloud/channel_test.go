package whatsappcloud

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/messaging"
)

type fakeAccountSource struct {
	phoneNumberID string
	token         string
	err           error
}

func (f *fakeAccountSource) ChannelExternalAccountID(ctx context.Context, accountID uuid.UUID) (string, error) {
	return f.phoneNumberID, f.err
}
func (f *fakeAccountSource) ChannelCredentialsSecret(ctx context.Context, accountID uuid.UUID) (string, error) {
	return f.token, f.err
}

type fakeMediaSource struct {
	mediaID, orgID uuid.UUID
	err            error
}

func (f *fakeMediaSource) ChannelOutboundMediaForSigning(ctx context.Context, messageID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	return f.mediaID, f.orgID, f.err
}

type fakeSigner struct {
	lastOrgID, lastMediaID uuid.UUID
	lastExp                int64
}

func (f *fakeSigner) Sign(orgID, mediaID uuid.UUID, expiresAtUnix int64) string {
	f.lastOrgID, f.lastMediaID, f.lastExp = orgID, mediaID, expiresAtUnix
	return "signed-token"
}

func TestChannelSenderSendTextResolvesAccountAtSendTime(t *testing.T) {
	var gotAuth, gotTo string
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotTo, _ = body["to"].(string)
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.sent1"}]}`))
	})
	accounts := &fakeAccountSource{phoneNumberID: "pn1", token: "resolved-token"}
	sender := NewChannelSender(c, accounts, &fakeMediaSource{}, &fakeSigner{}, "https://xchats.example/meta/api/v1/media")

	accountID := uuid.New()
	res, err := sender.Send(context.Background(), messaging.OutboundMessage{
		MessageID: uuid.New().String(), AccountID: accountID.String(), Channel: messaging.ChannelWhatsAppCloud,
		Text: "hello", To: "77011234567",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotAuth != "Bearer resolved-token" {
		t.Fatalf("Authorization = %q — the token must be resolved at send time via AccountSource, never carried on OutboundMessage", gotAuth)
	}
	if gotTo != "77011234567" {
		t.Fatalf("to = %q", gotTo)
	}
	if res.ExternalID != "wamid.sent1" || !res.Delivered {
		t.Fatalf("result = %+v", res)
	}
}

func TestChannelSenderSendMediaSignsAnOrgScopedLink(t *testing.T) {
	var gotLink string
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		img, _ := body["image"].(map[string]any)
		gotLink, _ = img["link"].(string)
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.sent2"}]}`))
	})
	accounts := &fakeAccountSource{phoneNumberID: "pn1", token: "tok"}
	org := uuid.New()
	media := uuid.New()
	signer := &fakeSigner{}
	sender := NewChannelSender(c, accounts, &fakeMediaSource{mediaID: media, orgID: org}, signer, "https://xchats.example/meta/api/v1/media")

	messageID := uuid.New()
	_, err := sender.Send(context.Background(), messaging.OutboundMessage{
		MessageID: messageID.String(), AccountID: uuid.New().String(), Channel: messaging.ChannelWhatsAppCloud,
		To:    "77011234567",
		Media: &messaging.OutboundMedia{BlobID: "blob-key-not-a-media-row-id", Kind: "image", Caption: "cap"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if signer.lastOrgID != org || signer.lastMediaID != media {
		t.Fatalf("signer was called with (org=%s, media=%s), want (%s, %s)", signer.lastOrgID, signer.lastMediaID, org, media)
	}
	wantPrefix := "https://xchats.example/meta/api/v1/media/" + media.String() + "?token=signed-token&exp="
	if len(gotLink) <= len(wantPrefix) || gotLink[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("link = %q, want prefix %q", gotLink, wantPrefix)
	}
}

func TestChannelSenderUnrecognizedMediaKindFallsBackToDocument(t *testing.T) {
	var gotType string
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotType, _ = body["type"].(string)
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.sent3"}]}`))
	})
	sender := NewChannelSender(c, &fakeAccountSource{phoneNumberID: "pn1", token: "tok"},
		&fakeMediaSource{mediaID: uuid.New(), orgID: uuid.New()}, &fakeSigner{}, "https://xchats.example/meta/api/v1/media")
	_, err := sender.Send(context.Background(), messaging.OutboundMessage{
		MessageID: uuid.New().String(), AccountID: uuid.New().String(), To: "77011234567",
		Media: &messaging.OutboundMedia{BlobID: "b", Kind: "some-future-kind"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotType != "document" {
		t.Fatalf("type = %q, want document (the safe fallback)", gotType)
	}
}

func TestChannelSenderInvalidAccountIDIsRejected(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no HTTP call should happen when the account id itself is invalid")
	})
	sender := NewChannelSender(c, &fakeAccountSource{}, &fakeMediaSource{}, &fakeSigner{}, "https://xchats.example/meta/api/v1/media")
	_, err := sender.Send(context.Background(), messaging.OutboundMessage{AccountID: "not-a-uuid", MessageID: uuid.New().String()})
	if err == nil {
		t.Fatal("expected an error for a malformed account id")
	}
}
