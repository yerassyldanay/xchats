package responsestore

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/response"
)

// historyWindowSize mirrors internal/assistant/real.go's windowSize: the
// number of recent messages loaded before splitting off the current inbound
// message.
const historyWindowSize = 15

// ConversationRepo implements response.ConversationRepository over the
// existing wa_chats/wa_messages/wa_accounts tables.
type ConversationRepo struct {
	Store *store.Store
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
	history, current, triggerID := splitHistory(msgs)

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
	}, nil
}

// splitHistory mirrors internal/assistant/real.go's splitWindow: the latest
// inbound message is the current message being answered, excluded from
// history so it is never duplicated in the rendered transcript;
// "in" -> client, "out" -> assistant.
func splitHistory(msgs []store.Message) (history []aiprompt.HistoryTurn, current string, triggerID string) {
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
	}
	return history, current, triggerID
}
