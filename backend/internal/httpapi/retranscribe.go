package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/stt"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
)

// retranscribeReq optionally overrides the configured language hint for
// this one attempt — the Inbox's "Re-transcribe as..." action, for a voice
// note whose language the model misdetected. "" (or "auto") falls back to
// LLMSettings.STTLanguage.
type retranscribeReq struct {
	Language string `json:"language"`
}

// handleRetranscribeMessage re-runs speech-to-text on one message's audio
// attachment — the manual counterpart to worker.Worker.transcribeIfAudio's
// automatic run right after download. Reuses the exact same
// stt.ResolveParams/stt.BuildPrompt/store.UpdateMediaTranscript pipeline so
// the two paths can never disagree about how a transcript gets produced.
// Always overwrites any prior transcript — an operator asking to
// re-transcribe has already judged the existing one wrong.
func (s *Server) handleRetranscribeMessage(c *gin.Context) {
	chatID, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	messageID, okMsgID := parseUUID(c, "message_id")
	if !okMsgID {
		return
	}
	chat, okChat := s.orgChat(c, chatID)
	if !okChat {
		return
	}
	var req retranscribeReq
	_ = c.ShouldBindJSON(&req) // an empty/absent body just keeps the configured language hint
	if req.Language != "" && !sttLanguages[req.Language] {
		fail(c, http.StatusBadRequest, ErrValidation, "unknown language")
		return
	}

	msg, err := s.store.MessageByID(ctx(c), messageID)
	if err != nil || msg.ChatID != chatID {
		fail(c, http.StatusNotFound, ErrNotFound, "message not found")
		return
	}
	var media *store.MediaRef
	for i := range msg.Media {
		if msg.Media[i].MediaType == "audio" {
			media = &msg.Media[i]
			break
		}
	}
	switch {
	case media == nil:
		fail(c, http.StatusBadRequest, ErrValidation, "message has no audio attachment")
		return
	case media.DownloadStatus != "ready" || media.StorageKey == "":
		fail(c, http.StatusConflict, ErrValidation, "audio is still downloading")
		return
	}
	if s.blob == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "media storage is unavailable")
		return
	}

	timeoutSeconds := 0
	if s.cfg != nil {
		timeoutSeconds = s.cfg.LLMDraftTimeoutSeconds
	}
	params := stt.ResolveParams(ctx(c), s.credentials, s.settings, timeoutSeconds)
	if params.Transcriber == nil {
		fail(c, http.StatusServiceUnavailable, ErrValidation, "no speech-to-text provider is configured")
		return
	}
	switch req.Language {
	case "":
		// keep the configured hint
	case "auto":
		params.Language = ""
	default:
		params.Language = req.Language
	}

	data, blobMeta, err := s.blob.Get(media.StorageKey)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "read audio bytes: "+err.Error())
		return
	}
	if int64(len(data)) > stt.MaxAudioBytes {
		fail(c, http.StatusRequestEntityTooLarge, ErrValidation, "audio exceeds the transcription provider's size limit")
		return
	}

	// Vocabulary priming is best-effort — a KB load failure (or no
	// organization/KnowledgeBase at all) just falls back to the operator's
	// own custom vocabulary alone, never blocks a re-transcribe.
	prompt := params.Vocabulary
	if s.response != nil && s.response.KnowledgeBase != nil {
		if account, err := s.store.AccountByID(ctx(c), chat.AccountID); err == nil && account.OrganizationID.Valid {
			if kb, err := s.response.KnowledgeBase.Load(ctx(c), account.OrganizationID.UUID.String()); err == nil {
				prompt = stt.BuildPrompt(kb, params.Vocabulary)
			}
		}
	}

	text, err := params.Transcriber.Transcribe(ctx(c), data, blobMeta.FileName, blobMeta.Mimetype, stt.TranscribeOptions{
		Language: params.Language, Prompt: prompt,
	})
	if err != nil {
		fail(c, http.StatusBadGateway, ErrAIUnavailable, "transcription failed: "+err.Error())
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		fail(c, http.StatusUnprocessableEntity, ErrValidation, "transcription returned no text")
		return
	}

	if err := s.store.UpdateMediaTranscript(ctx(c), chat.Channel, messageID, text); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	updated, err := s.store.MessageByID(ctx(c), messageID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	s.hub.Broadcast("message.updated", dto.MapMessage(updated))
	// Keep the chat list's sidebar preview in sync too — worker.transcribeIfAudio's
	// automatic run already does this; a manual re-transcribe correcting the
	// same field deserves the same UI refresh, not just the open thread.
	if err := s.store.UpdateChatPreviewIfCurrent(ctx(c), chatID, updated.MessageTS, worker.TranscriptPreview(text)); err != nil {
		s.log.Error("update chat preview after retranscribe", "chat_id", chatID, "err", err)
	} else if chat, err := s.store.ChatByID(ctx(c), chatID); err == nil {
		s.hub.Broadcast("chat.updated", dto.MapChat(chat))
	}
	// A fresh transcript deserves a fresh draft, exactly like handleSuggest's
	// own direct KindAIDraft enqueue (drafts.go) — never through automation's
	// debounce, which already ran once when the message first arrived.
	s.publishOrLog(ctx(c), queue.Message{Kind: queue.KindAIDraft, Payload: worker.AIDraftTask{ChatID: chatID}})

	ok(c, dto.MapMessage(updated))
}
