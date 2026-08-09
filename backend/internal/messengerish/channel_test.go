package messengerish

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/messaging"
)

type fakeAccountSource struct {
	externalAccountID string
	token             string
	err               error
}

func (f *fakeAccountSource) ChannelExternalAccountID(ctx context.Context, accountID uuid.UUID) (string, error) {
	return f.externalAccountID, f.err
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
	var gotAuth, gotRecipient string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		recipient, _ := body["recipient"].(map[string]any)
		gotRecipient, _ = recipient["id"].(string)
		_, _ = w.Write([]byte(`{"recipient_id":"1234567890123456","message_id":"m1"}`))
	}, nil)
	accounts := &fakeAccountSource{externalAccountID: "17841400000000001", token: "resolved-token"}
	sender := NewChannelSender(c, accounts, &fakeMediaSource{}, &fakeSigner{}, true, "https://xchats.example/meta/api/v1/media")

	accountID := uuid.New()
	res, err := sender.Send(context.Background(), messaging.OutboundMessage{
		MessageID: uuid.New().String(), AccountID: accountID.String(), Channel: messaging.ChannelInstagram,
		Text: "hello", To: "1234567890123456",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotAuth != "Bearer resolved-token" {
		t.Fatalf("Authorization = %q — the token must be resolved at send time via AccountSource, never carried on OutboundMessage", gotAuth)
	}
	if gotRecipient != "1234567890123456" {
		t.Fatalf("recipient = %q", gotRecipient)
	}
	if res.ExternalID != "m1" || !res.Delivered {
		t.Fatalf("result = %+v", res)
	}
}

func TestChannelSenderSendMediaSignsAnOrgScopedLink(t *testing.T) {
	var gotURL string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		message, _ := body["message"].(map[string]any)
		att, _ := message["attachment"].(map[string]any)
		payload, _ := att["payload"].(map[string]any)
		gotURL, _ = payload["url"].(string)
		_, _ = w.Write([]byte(`{"recipient_id":"1234567890123456","message_id":"m2"}`))
	}, nil)
	accounts := &fakeAccountSource{externalAccountID: "17841400000000001", token: "tok"}
	org := uuid.New()
	media := uuid.New()
	signer := &fakeSigner{}
	sender := NewChannelSender(c, accounts, &fakeMediaSource{mediaID: media, orgID: org}, signer, true, "https://xchats.example/meta/api/v1/media")

	messageID := uuid.New()
	_, err := sender.Send(context.Background(), messaging.OutboundMessage{
		MessageID: messageID.String(), AccountID: uuid.New().String(), Channel: messaging.ChannelInstagram,
		To:    "1234567890123456",
		Media: &messaging.OutboundMedia{BlobID: "blob-key-not-a-media-row-id", Kind: "image"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if signer.lastOrgID != org || signer.lastMediaID != media {
		t.Fatalf("signer was called with (org=%s, media=%s), want (%s, %s)", signer.lastOrgID, signer.lastMediaID, org, media)
	}
	wantPrefix := "https://xchats.example/meta/api/v1/media/" + media.String() + "?token=signed-token&exp="
	if len(gotURL) <= len(wantPrefix) || gotURL[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("url = %q, want prefix %q", gotURL, wantPrefix)
	}
}

func TestChannelSenderUnrecognizedMediaKindFallsBackToFile(t *testing.T) {
	var gotType string
	c := testClient(t, nil, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		message, _ := body["message"].(map[string]any)
		att, _ := message["attachment"].(map[string]any)
		gotType, _ = att["type"].(string)
		_, _ = w.Write([]byte(`{"recipient_id":"psid-1","message_id":"m3"}`))
	})
	sender := NewChannelSender(c, &fakeAccountSource{externalAccountID: "page-1", token: "tok"},
		&fakeMediaSource{mediaID: uuid.New(), orgID: uuid.New()}, &fakeSigner{}, false, "https://xchats.example/meta/api/v1/media")
	_, err := sender.Send(context.Background(), messaging.OutboundMessage{
		MessageID: uuid.New().String(), AccountID: uuid.New().String(), To: "psid-1",
		Media: &messaging.OutboundMedia{BlobID: "b", Kind: "some-future-kind"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotType != "file" {
		t.Fatalf("type = %q, want file (the safe fallback)", gotType)
	}
}

func TestChannelSenderInvalidAccountIDIsRejected(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no HTTP call should happen when the account id itself is invalid")
	}, nil)
	sender := NewChannelSender(c, &fakeAccountSource{}, &fakeMediaSource{}, &fakeSigner{}, true, "https://xchats.example/meta/api/v1/media")
	_, err := sender.Send(context.Background(), messaging.OutboundMessage{AccountID: "not-a-uuid", MessageID: uuid.New().String()})
	if err == nil {
		t.Fatal("expected an error for a malformed account id")
	}
}
