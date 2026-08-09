package httpapi_test

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/inboxmedia"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// seedOutboundMedia creates a channel_messages row + a ready
// channel_message_media row backed by a real blob, and returns the media
// row's own id (the exact id inboxmedia.Signer signs) plus the owning org.
func (h *metaHarness) seedOutboundMedia(acctID uuid.UUID, blobKey string, data []byte) (mediaID, orgID uuid.UUID) {
	h.t.Helper()
	ctx := context.Background()
	inbound, err := h.store.IngestChannelInbound(ctx, store.ChannelInbound{
		AccountID: acctID, ExternalContactID: "media-seed-contact", ExternalThreadID: "media-seed-contact",
		Direction: "in", SenderKind: "contact", ExternalMessageID: "media-seed-msg", MessageKind: "conversation", Body: "hi",
	})
	if err != nil {
		h.t.Fatalf("IngestChannelInbound: %v", err)
	}
	msgID, err := h.store.InsertOutbound(ctx, "whatsapp_cloud", inbound.ChatID, acctID, "user", uuid.NullUUID{}, "imageMessage", "", "photo")
	if err != nil {
		h.t.Fatalf("InsertOutbound: %v", err)
	}
	if _, err := h.blob.Put(blobKey, data, blob.Meta{MediaType: "image", Mimetype: "image/jpeg", FileName: "x.jpg", FileSize: int64(len(data))}); err != nil {
		h.t.Fatalf("blob.Put: %v", err)
	}
	if err := h.store.UpsertOutboundMedia(ctx, "whatsapp_cloud", msgID, store.MediaRef{
		MediaType: "image", Mimetype: "image/jpeg", FileName: "x.jpg", FileSize: len(data),
	}, blobKey); err != nil {
		h.t.Fatalf("UpsertOutboundMedia: %v", err)
	}
	mediaID, orgID, err = h.store.ChannelOutboundMediaForSigning(ctx, msgID)
	if err != nil {
		h.t.Fatalf("ChannelOutboundMediaForSigning: %v", err)
	}
	return mediaID, orgID
}

func TestMetaMediaReadServesValidToken(t *testing.T) {
	h := newMetaHarness(t)
	acct := h.seedWhatsAppCloudAccount("15550009001", "waba-media-1")
	mediaID, orgID := h.seedOutboundMedia(acct.ID, "media-ok", []byte("fake-jpeg-bytes"))

	signer := inboxmedia.NewSigner(h.cfg.SessionSecret)
	exp := time.Now().Add(5 * time.Minute).Unix()
	token := signer.Sign(orgID, mediaID, exp)

	resp, err := http.Get(h.srv.URL + "/meta/api/v1/media/" + mediaID.String() + "?token=" + token + "&exp=" + strconv.FormatInt(exp, 10))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "fake-jpeg-bytes" {
		t.Fatalf("body = %q", body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("Content-Type = %q", ct)
	}
}

func TestMetaMediaReadRejectsExpiredToken(t *testing.T) {
	h := newMetaHarness(t)
	acct := h.seedWhatsAppCloudAccount("15550009002", "waba-media-2")
	mediaID, orgID := h.seedOutboundMedia(acct.ID, "media-expired", []byte("bytes"))

	signer := inboxmedia.NewSigner(h.cfg.SessionSecret)
	exp := time.Now().Add(-1 * time.Minute).Unix() // already expired
	token := signer.Sign(orgID, mediaID, exp)

	resp, err := http.Get(h.srv.URL + "/meta/api/v1/media/" + mediaID.String() + "?token=" + token + "&exp=" + strconv.FormatInt(exp, 10))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

// TestMetaMediaReadRejectsCrossOrgToken is the IDOR regression test: a token
// honestly signed for a DIFFERENT organization than the one that actually
// owns mediaID must not authorize the read — see ChannelMediaByID's own doc
// comment for why the handler resolves org independently rather than
// trusting anything client-supplied.
func TestMetaMediaReadRejectsCrossOrgToken(t *testing.T) {
	h := newMetaHarness(t)
	acct := h.seedWhatsAppCloudAccount("15550009003", "waba-media-3")
	mediaID, _ := h.seedOutboundMedia(acct.ID, "media-crossorg", []byte("bytes"))

	attackerOrgID := uuid.New()
	signer := inboxmedia.NewSigner(h.cfg.SessionSecret)
	exp := time.Now().Add(5 * time.Minute).Unix()
	token := signer.Sign(attackerOrgID, mediaID, exp) // signed for the WRONG org

	resp, err := http.Get(h.srv.URL + "/meta/api/v1/media/" + mediaID.String() + "?token=" + token + "&exp=" + strconv.FormatInt(exp, 10))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (a token signed for a different org must not authorize this media)", resp.StatusCode)
	}
}

func TestMetaMediaReadUnknownMediaIs404(t *testing.T) {
	h := newMetaHarness(t)
	signer := inboxmedia.NewSigner(h.cfg.SessionSecret)
	exp := time.Now().Add(5 * time.Minute).Unix()
	randomID := uuid.New()
	token := signer.Sign(uuid.New(), randomID, exp)

	resp, err := http.Get(h.srv.URL + "/meta/api/v1/media/" + randomID.String() + "?token=" + token + "&exp=" + strconv.FormatInt(exp, 10))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}
