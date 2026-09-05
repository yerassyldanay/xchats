package responsestore

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/response"
)

// historyWindowSize mirrors internal/assistant/real.go's windowSize: the
// number of recent messages loaded before splitting off the current inbound
// message.
const historyWindowSize = 15

// maxVisionImageBytes bounds how large an image's raw bytes may be before
// LoadForResponse embeds it as a base64 data URI. A multi-megabyte photo
// inflates by roughly a third once base64-encoded and rides inside the SAME
// request as the whole rendered KB prompt — this keeps one oversized
// attachment from blowing a provider's request-size limit. An image over
// the cap is simply not attached as vision input; attachmentTailSuffix's
// "[Прикреплено фото]" text note still reaches the model either way.
const maxVisionImageBytes = 8 << 20 // 8 MiB

// ConversationRepo implements response.ConversationRepository over the
// existing wa_chats/wa_messages/wa_accounts tables.
type ConversationRepo struct {
	Store *store.Store
	// Blob resolves an image attachment's bytes for a vision call (see
	// resolveAttachments). nil is safe — every image attachment is then
	// simply skipped instead of embedded, same as an unready one.
	Blob blob.Store
}

// LoadForResponse resolves conversationID -> account -> organization (never a
// global org id), the latest historyWindowSize messages, and splits off the
// latest inbound message as the current message being answered.
func (r *ConversationRepo) LoadForResponse(ctx context.Context, conversationID string) (response.ConversationContext, error) {
	chatID, err := uuid.Parse(conversationID)
	if err != nil {
		return response.ConversationContext{}, fmt.Errorf("responsestore: invalid conversation id %q: %w", conversationID, err)
	}
	chat, err := r.Store.ChatByID(ctx, chatID)
	if err != nil {
		return response.ConversationContext{}, fmt.Errorf("responsestore: load conversation: %w", err)
	}
	account, err := r.Store.AccountByID(ctx, chat.AccountID)
	if err != nil {
		return response.ConversationContext{}, fmt.Errorf("responsestore: load account: %w", err)
	}
	if !account.OrganizationID.Valid {
		return response.ConversationContext{}, fmt.Errorf("responsestore: account %s has no organization", account.ID)
	}

	msgs, _, err := r.Store.MessagesForChat(ctx, chatID, time.Time{}, historyWindowSize)
	if err != nil {
		return response.ConversationContext{}, fmt.Errorf("responsestore: load message window: %w", err)
	}
	history, current, triggerID, currentMedia := splitHistory(msgs)
	attachments := r.resolveAttachments(currentMedia, current)

	// CRM context is best-effort: a conversation with no customer (or a
	// failed lookup) leaves Customer nil, which renders no block at all and
	// therefore sends exactly the prompt this path sent before the CRM layer
	// existed. Never fail a customer reply over a missing profile.
	var customer *aiprompt.CustomerContext
	if summary, found, err := r.Store.CustomerContextForChat(ctx, chatID); err == nil && found {
		customer = &aiprompt.CustomerContext{
			Name:       summary.Name,
			Status:     summary.Status,
			Tags:       summary.Tags,
			LatestNote: summary.LatestNote,
			NextAction: summary.NextAction,
			NextDueOn:  summary.NextDueOn,
		}
	}

	return response.ConversationContext{
		OrganizationID:   account.OrganizationID.UUID.String(),
		AccountID:        account.ID.String(),
		Channel:          messaging.Channel(account.Channel),
		History:          history,
		CurrentMessage:   current,
		TriggerMessageID: triggerID,
		Customer:         customer,
		Attachments:      attachments,
	}, nil
}

// splitHistory mirrors internal/assistant/real.go's splitWindow: the latest
// inbound message is the current message being answered, excluded from
// history so it is never duplicated in the rendered transcript;
// "in" -> client, "out" -> assistant. currentMedia is the trigger message's
// own attachments (never a history turn's) — resolveAttachments turns them
// into what Engine can act on.
func splitHistory(msgs []store.Message) (history []aiprompt.HistoryTurn, current string, triggerID string, currentMedia []store.MediaRef) {
	lastIn := -1
	for i, m := range msgs {
		if m.Direction == "in" {
			lastIn = i
		}
	}
	for i, m := range msgs {
		if i == lastIn {
			continue
		}
		role := "client"
		if m.Direction == "out" {
			role = "assistant"
		}
		history = append(history, aiprompt.HistoryTurn{Role: role, Text: m.Body})
	}
	if lastIn >= 0 {
		current = msgs[lastIn].Body
		triggerID = msgs[lastIn].ID.String()
		currentMedia = msgs[lastIn].Media
	}
	return history, current, triggerID, currentMedia
}

// resolveAttachments turns the trigger message's own media rows into
// engine-ready response.IncomingAttachment values: a transcribed audio note
// becomes a Transcript attachment, and a downloaded image becomes a
// base64-encoded DataURI attachment — this is where that resolution has to
// happen, since Engine itself has no blob-storage dependency (see
// response.IncomingAttachment's own doc comment). caption is the trigger
// message's own text body, carried onto every image attachment from it (a
// customer sending one photo with one caption is the overwhelmingly common
// case; multiple images in the same message all sharing that same caption
// is a acceptable simplification over inventing a per-image caption that
// does not exist in the data).
//
// A media row that isn't ready yet, or an image whose bytes are
// unexpectedly missing or larger than maxVisionImageBytes, is simply
// skipped — Generate still runs on whatever text the customer sent.
func (r *ConversationRepo) resolveAttachments(media []store.MediaRef, caption string) []response.IncomingAttachment {
	var out []response.IncomingAttachment
	for _, m := range media {
		switch {
		case m.MediaType == "audio" && m.Transcript != "":
			out = append(out, response.IncomingAttachment{Kind: response.AttachmentAudio, Transcript: m.Transcript})
		case m.MediaType == "image" && m.DownloadStatus == "ready" && m.StorageKey != "" && r.Blob != nil:
			data, meta, err := r.Blob.Get(m.StorageKey)
			if err != nil || len(data) == 0 || len(data) > maxVisionImageBytes {
				continue
			}
			mimetype := meta.Mimetype
			if mimetype == "" {
				mimetype = m.Mimetype
			}
			if mimetype == "" {
				mimetype = "image/jpeg"
			}
			out = append(out, response.IncomingAttachment{
				Kind:    response.AttachmentImage,
				DataURI: "data:" + mimetype + ";base64," + base64.StdEncoding.EncodeToString(data),
				Caption: caption,
			})
		}
	}
	return out
}
