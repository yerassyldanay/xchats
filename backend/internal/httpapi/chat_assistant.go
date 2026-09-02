package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/chat"
	"github.com/yerassyldanay/xchats/backend/internal/chatkb"
	"github.com/yerassyldanay/xchats/backend/llm"
)

// --- /chat/* — the Knowledge Base chat assistant -----------------------------
//
// Deliberately NOT /chats/* (chats.go): that surface is the customer inbox —
// real people messaging the business over WhatsApp/Telegram/Instagram. This
// one is an operator talking to an assistant about their own knowledge base,
// and nothing written here is ever delivered to a customer. The one-letter
// path difference is unfortunate but the vocabulary is the product's, not
// this file's.
//
// Everything here is scoped to (organization, user) — see chatstore's package
// doc. A conversation is private to the operator who started it.

// maxChatMessageBytes bounds one operator message. Generous for a question,
// far below anything that would blow out a model's context by itself.
const maxChatMessageBytes = 32 << 10 // 32 KiB

// chatReady reports whether the assistant is wired up, failing the request
// with a clear 503 if not — a deployment built without the chat service
// still serves every other route.
func (s *Server) chatReady(c *gin.Context) bool {
	if s.chat == nil {
		fail(c, http.StatusServiceUnavailable, ErrAIUnavailable, "chat assistant is not configured")
		return false
	}
	return true
}

// chatScope resolves the (organization, user) pair every chat operation is
// keyed by.
func (s *Server) chatScope(c *gin.Context) (chat.Scope, bool) {
	if !s.chatReady(c) {
		return chat.Scope{}, false
	}
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return chat.Scope{}, false
	}
	return chat.Scope{OrgID: org.ID, UserID: currentUser(c).ID}, true
}

// chatConversationID parses the :id path segment.
func chatConversationID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid conversation id")
		return uuid.Nil, false
	}
	return id, true
}

// chatFail maps a service error onto the API's error vocabulary. Everything
// the model provider can do to us — no key configured, a rejected key, a
// timeout — is AI_UNAVAILABLE: from the operator's side they are one
// situation ("the assistant can't answer right now"), and the message
// carries the specifics.
func (s *Server) chatFail(c *gin.Context, err error) {
	status, code, msg := chatError(err)
	if code == ErrOK {
		// The client went away mid-request; there is nobody to answer.
		c.Abort()
		return
	}
	if code == ErrInternal {
		s.log.Error("chat request failed", "err", err)
	}
	fail(c, status, code, msg)
}

func chatError(err error) (status int, code, msg string) {
	switch {
	case errors.Is(err, chat.ErrNotFound):
		return http.StatusNotFound, ErrNotFound, "conversation not found"
	case errors.Is(err, chat.ErrEmptyMessage):
		return http.StatusBadRequest, ErrValidation, "message is empty"
	case errors.Is(err, llm.ErrProviderAuth):
		return http.StatusServiceUnavailable, ErrAIUnavailable, "the AI provider rejected the configured API key — check Settings"
	case errors.Is(err, context.Canceled):
		// The operator navigated away or hit stop. Not a failure worth an
		// error page; the stream is simply over.
		return http.StatusOK, ErrOK, ""
	default:
		return http.StatusInternalServerError, ErrInternal, err.Error()
	}
}

// --- conversation lifecycle --------------------------------------------------

type chatConversationsPayload struct {
	Conversations []chat.Conversation `json:"conversations"`
}

func (s *Server) handleChatListConversations(c *gin.Context) {
	scope, proceed := s.chatScope(c)
	if !proceed {
		return
	}
	limit := 0
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	conversations, err := s.chat.ListConversations(ctx(c), scope, limit)
	if err != nil {
		s.chatFail(c, err)
		return
	}
	ok(c, chatConversationsPayload{Conversations: conversations})
}

type chatCreateConversationRequest struct {
	Title string `json:"title"`
}

func (s *Server) handleChatCreateConversation(c *gin.Context) {
	scope, proceed := s.chatScope(c)
	if !proceed {
		return
	}
	var req chatCreateConversationRequest
	// A missing body is normal: "+ New chat" sends nothing, and the first
	// message names the thread.
	_ = c.ShouldBindJSON(&req)
	conv, err := s.chat.CreateConversation(ctx(c), scope, req.Title)
	if err != nil {
		s.chatFail(c, err)
		return
	}
	created(c, conv)
}

func (s *Server) handleChatGetConversation(c *gin.Context) {
	scope, proceed := s.chatScope(c)
	if !proceed {
		return
	}
	id, okID := chatConversationID(c)
	if !okID {
		return
	}
	detail, err := s.chat.Conversation(ctx(c), scope, id)
	if err != nil {
		s.chatFail(c, err)
		return
	}
	ok(c, detail)
}

type chatRenameConversationRequest struct {
	Title string `json:"title"`
}

func (s *Server) handleChatRenameConversation(c *gin.Context) {
	scope, proceed := s.chatScope(c)
	if !proceed {
		return
	}
	id, okID := chatConversationID(c)
	if !okID {
		return
	}
	var req chatRenameConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	conv, err := s.chat.RenameConversation(ctx(c), scope, id, req.Title)
	if err != nil {
		s.chatFail(c, err)
		return
	}
	ok(c, conv)
}

func (s *Server) handleChatDeleteConversation(c *gin.Context) {
	scope, proceed := s.chatScope(c)
	if !proceed {
		return
	}
	id, okID := chatConversationID(c)
	if !okID {
		return
	}
	if err := s.chat.DeleteConversation(ctx(c), scope, id); err != nil {
		s.chatFail(c, err)
		return
	}
	ok(c, gin.H{"deleted": true})
}

// --- the streaming turn --------------------------------------------------------

type chatSendRequest struct {
	Content string `json:"content"`
}

// chatStartedPayload is the stream's opening event: the operator's own
// message as persisted, plus the id the assistant's message will have. The
// id is settled before generation starts so the UI can key a streaming
// bubble on it rather than on a placeholder it later has to reconcile.
type chatStartedPayload struct {
	User        chat.Message `json:"user"`
	AssistantID uuid.UUID    `json:"assistant_id"`
}

type chatDeltaPayload struct {
	Text string `json:"text"`
}

type chatErrorPayload struct {
	Errcode string `json:"errcode"`
	Message string `json:"message"`
}

// handleChatSendMessage streams one assistant turn as Server-Sent Events:
//
//	message_created  {"user": Message, "assistant_id": "..."}
//	components       [{"type": "kb_comparison", "data": {...}}, ...]
//	text_delta       {"text": "..."}          (many)
//	done             Message                  (the persisted assistant turn)
//	error            {"errcode": "...", "message": "..."}
//
// Validation failures BEFORE the stream opens (bad body, unknown
// conversation) answer with the normal {payload, errcode, message} envelope
// — a client that got a 404 never had a stream to parse. Once the first
// event is written the status line is spent, so every later failure arrives
// as an `error` event instead; the two paths share chatError so the same
// failure carries the same errcode either way.
func (s *Server) handleChatSendMessage(c *gin.Context) {
	scope, proceed := s.chatScope(c)
	if !proceed {
		return
	}
	id, okID := chatConversationID(c)
	if !okID {
		return
	}
	flusher, okFlush := c.Writer.(http.Flusher)
	if !okFlush {
		fail(c, http.StatusInternalServerError, ErrInternal, "streaming unsupported")
		return
	}

	var req chatSendRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxChatMessageBytes)
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	// Reject an empty message and an unknown conversation before opening the
	// stream, so the two most common client mistakes stay ordinary HTTP
	// errors.
	if strings.TrimSpace(req.Content) == "" {
		s.chatFail(c, chat.ErrEmptyMessage)
		return
	}
	if _, err := s.chat.Conversation(ctx(c), scope, id); err != nil {
		s.chatFail(c, err)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	// nginx buffers proxied responses by default, which would hold the whole
	// answer back and defeat streaming entirely (frontend/nginx.conf proxies
	// this route).
	c.Header("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	flusher.Flush()

	send := func(event string, data any) {
		raw, err := json.Marshal(data)
		if err != nil {
			s.log.Error("chat: encode stream event", "event", event, "err", err)
			return
		}
		_, _ = c.Writer.WriteString("event: " + event + "\n")
		_, _ = c.Writer.WriteString("data: " + string(raw) + "\n\n")
		flusher.Flush()
	}

	_, err := s.chat.Send(ctx(c), scope, chat.SendInput{ConversationID: id, Text: req.Content}, chat.Sink{
		Started: func(user chat.Message, assistantID uuid.UUID) {
			send("message_created", chatStartedPayload{User: user, AssistantID: assistantID})
		},
		Components: func(components []chatkb.Component) { send("components", components) },
		Delta:      func(text string) { send("text_delta", chatDeltaPayload{Text: text}) },
		Done:       func(assistant chat.Message) { send("done", assistant) },
	})
	if err != nil {
		status, code, msg := chatError(err)
		if code == ErrOK {
			return // the client disconnected; there is nobody left to tell
		}
		if status == http.StatusInternalServerError {
			s.log.Error("chat: stream failed", "conversation_id", id, "err", err)
		}
		send("error", chatErrorPayload{Errcode: code, Message: msg})
	}
}
