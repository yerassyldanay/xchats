package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yerassyldanay/xchats/backend/internal/meta"
	"github.com/yerassyldanay/xchats/backend/internal/metaingest"
	"github.com/yerassyldanay/xchats/backend/internal/whatsappcloud"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// handleMetaWebhookVerify is the one-time GET subscription handshake every
// Meta channel performs identically — see meta.Challenge's own doc comment
// for the protocol. It is shared across all three Meta channels (a single
// verify_token, derived once — see meta.DeriveVerifyToken) even though today
// only the WhatsApp Cloud route is registered; Instagram/Messenger's own POST
// routes (Phase 4/5) will point their GET half at this same handler.
func (s *Server) handleMetaWebhookVerify(c *gin.Context) {
	verifyToken := meta.DeriveVerifyToken(s.cfg.SessionSecret)
	challenge, matched := meta.Challenge(c.Request.URL.Query(), verifyToken)
	if !matched {
		s.log.Warn("meta webhook verify rejected", "path", c.Request.URL.Path)
		c.Status(http.StatusForbidden)
		return
	}
	// Plain text, no JSON envelope — Meta treats anything else as a failed
	// subscription attempt.
	c.String(http.StatusOK, challenge)
}

// handleWhatsAppCloudWebhook is the WhatsApp Cloud API ingress. It does NOT
// enqueue-and-ack: the normalized rows are committed BEFORE the 200 — see
// handleTelegramWebhook's identical shape and doc comment for the general
// at-least-once reasoning, and internal/metaingest.Processor for the shared
// ingest core this delegates to.
//
// Order matters (§10), each step answering the way that keeps Meta's own
// retry behavior correct: Meta redelivers a webhook until it sees a 2xx.
//   - No App Secret configured at all: 503 — an operator setup gap, not
//     something retrying will fix, but also not safe to treat as "ignore
//     forever" the way an unknown account is, so this is the one non-2xx
//     that is not "durability failure".
//   - A bad signature: 401 — never store or process an unverified body.
//   - A malformed body, or an entry naming a WABA this install does not own:
//     200 — retrying either would only ever reproduce the same outcome.
//   - An ingest/enqueue failure: 500 — Meta redelivers, and every write here
//     is idempotent on (channel, external_message_id), so a redelivered
//     entry that DID partially land is a safe no-op the second time.
func (s *Server) handleWhatsAppCloudWebhook(c *gin.Context) {
	raw, err := c.GetRawData()
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "unreadable body")
		return
	}

	creds, okCreds := s.metaCreds.MetaAppCredentials(ctx(c))
	if !okCreds {
		s.log.Error("whatsapp cloud webhook rejected: no Meta App Secret configured")
		fail(c, http.StatusServiceUnavailable, ErrMetaAppNotConfigured, "Meta App Secret not configured")
		return
	}
	if !meta.VerifySignature(creds.AppSecret, raw, c.GetHeader("X-Hub-Signature-256")) {
		s.log.Warn("whatsapp cloud webhook auth rejected", "reason", "bad signature")
		fail(c, http.StatusUnauthorized, ErrWebhookUnauthorized, "bad webhook signature")
		return
	}

	var payload whatsappcloud.WebhookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		s.log.Warn("whatsapp cloud webhook unparsable body", "err", err)
		c.Status(http.StatusOK)
		return
	}

	stored, duplicate, ignored, statuses := 0, 0, 0, 0
	for _, entry := range payload.Entry {
		// entry.ID is the WABA id (see Entry's own doc comment) — the
		// account this delivery is FOR is named one level deeper, by
		// change.Value.Metadata.PhoneNumberID (channel_accounts.
		// external_account_id), since one WABA can have several registered
		// numbers each with its own channel_accounts row and a batched
		// delivery can in principle cover more than one of them.
		for _, change := range entry.Changes {
			phoneNumberID := change.Value.Metadata.PhoneNumberID
			if phoneNumberID == "" {
				continue
			}
			acct, err := s.store.ChannelAccountByExternalID(ctx(c), string(messaging.ChannelWhatsAppCloud), phoneNumberID)
			if err != nil {
				s.log.Info("whatsapp cloud webhook discarded", "phone_number_id", phoneNumberID, "result", "unknown_account")
				continue
			}
			for _, m := range change.Value.Messages {
				if reason, ignorable := m.Ignorable(); ignorable {
					ignored++
					s.log.Info("whatsapp cloud message dropped", "account_id", acct.ID, "result", reason)
					continue
				}
				outcome, perr := s.metaProc.Process(ctx(c), acct, whatsAppCloudNormalize(m, change.Value.Contacts))
				if perr != nil {
					s.log.Error("whatsapp cloud webhook processing failed; not acking", "account_id", acct.ID, "err", perr)
					fail(c, http.StatusInternalServerError, ErrInternal, "processing failed")
					return
				}
				if outcome.Status == "duplicate" {
					duplicate++
				} else {
					stored++
				}
			}
			for _, st := range change.Value.Statuses {
				if _, aerr := s.metaProc.ApplyStatus(ctx(c), metaingest.StatusUpdate{
					AccountID: acct.ID, Channel: string(messaging.ChannelWhatsAppCloud),
					ExternalMessageID: st.ID, Status: st.Status, Rank: whatsappcloud.DeliveryRank(st.Status),
				}); aerr != nil {
					s.log.Error("whatsapp cloud status update failed; not acking", "account_id", acct.ID, "err", aerr)
					fail(c, http.StatusInternalServerError, ErrInternal, "processing failed")
					return
				}
				statuses++
			}
		}
	}
	s.log.Info("whatsapp cloud webhook processed", "stored", stored, "duplicate", duplicate, "ignored", ignored, "statuses", statuses)
	c.Status(http.StatusOK)
}

// whatsAppCloudNormalize maps one inbound WhatsApp Cloud message (plus its
// delivery's contact list) into metaingest's channel-neutral shape.
// WhatsApp Cloud never echoes our own sends back through the webhook (§10),
// so Direction/SenderKind are always the inbound-customer-message values.
func whatsAppCloudNormalize(m whatsappcloud.Message, contacts []whatsappcloud.Contact) metaingest.NormalizedMessage {
	contactName := ""
	for _, ct := range contacts {
		if ct.WAID == m.From {
			contactName = ct.Profile.Name
			break
		}
	}
	norm := metaingest.NormalizedMessage{
		ExternalContactID:  m.From,
		ContactHandle:      m.From,
		ContactDisplayName: contactName,
		ExternalThreadID:   m.From,
		Direction:          "in",
		SenderKind:         "contact",
		ExternalMessageID:  m.ID,
		MessageKind:        m.MessageKind(),
		Body:               m.Body(),
		Preview:            m.Preview(),
		MessageTS:          whatsappcloud.ParseTimestamp(m.Timestamp),
	}
	if raw, err := json.Marshal(m); err == nil {
		norm.Raw = raw
	}
	if att := m.Attachment(); att != nil {
		norm.Attachment = &metaingest.NormalizedAttachment{
			// WhatsApp Cloud's own Type vocabulary (image|video|audio|
			// document|sticker) already matches this codebase's MediaType
			// convention one-for-one (see telegram.Message.MediaKind's
			// identical vocabulary) — no translation needed.
			MediaType: m.Type, Mimetype: att.MimeType, FileName: att.Filename, ProviderRef: att.ID,
		}
	}
	return norm
}
