package store

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// ---------------------------------------------------------------------------
// Customer timeline
// ---------------------------------------------------------------------------
// crm_timeline holds CRM events ONLY. The timeline the UI renders also shows
// conversation activity ("клиент написал в WhatsApp", "ИИ ответил"), but those
// entries are read live from inbox_messages_v and merged by timestamp in
// CustomerTimeline below. Mirroring every message into crm_timeline would
// double every message write to store what wa_messages/tg_messages already
// know, and would drift the moment a message is edited or redelivered.

// Timeline event kinds. These are the crm_timeline.kind CHECK's values;
// adding one means extending that constraint in a forward migration.
const (
	TimelineCustomerCreated     = "customer_created"
	TimelineIdentityLinked      = "identity_linked"
	TimelineNoteAdded           = "note_added"
	TimelineStatusChanged       = "status_changed"
	TimelineTagAdded            = "tag_added"
	TimelineTagRemoved          = "tag_removed"
	TimelineAssigneeChanged     = "assignee_changed"
	TimelineFollowupCreated     = "followup_created"
	TimelineFollowupCompleted   = "followup_completed"
	TimelineFollowupRescheduled = "followup_rescheduled"
	TimelineFollowupCancelled   = "followup_cancelled"
	TimelineCustomersMerged     = "customers_merged"
)

// Timeline entry sources. TimelineSourceMessage rows are assembled at read
// time from the messages tables and have no crm_timeline row behind them.
const (
	TimelineSourceCRM     = "crm"
	TimelineSourceMessage = "message"
)

// timelineEvent is one CRM event as written by the store methods that cause
// it. Every mutating CRM method appends inside its own transaction, so an
// event can never survive a rolled-back change.
type timelineEvent struct {
	Kind    string
	Actor   uuid.NullUUID
	Summary string
	Detail  []byte
	// OccurredAt defaults to now when zero — only the merge path, which
	// carries events over from the merged-away customer, sets it explicitly.
	OccurredAt time.Time
}

// TimelineEntry is one row of the merged customer timeline: either a CRM event
// or a conversation message. Source says which, so the UI can render each
// without guessing from the other fields.
type TimelineEntry struct {
	ID          uuid.UUID
	Source      string
	Kind        string
	ActorUserID uuid.NullUUID
	Summary     string
	Detail      []byte
	OccurredAt  time.Time

	// Message fields, set only when Source == TimelineSourceMessage.
	Channel    string
	ChatID     uuid.NullUUID
	Direction  string
	SenderKind string
	Body       string
}

func appendTimeline(ctx context.Context, tx *dbx.Tx, orgID, customerID uuid.UUID, ev timelineEvent) error {
	detail := ev.Detail
	if len(detail) == 0 {
		detail = []byte("{}")
	}
	var occurred any
	if !ev.OccurredAt.IsZero() {
		occurred = ev.OccurredAt
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO crm_timeline (organization_id, customer_id, kind, actor_user_id, summary, detail, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE($7, strftime('%Y-%m-%d %H:%M:%f','now')))`,
		orgID, customerID, ev.Kind, ev.Actor, ev.Summary, string(detail), occurred)
	return wrap("append timeline", err)
}

// CustomerTimeline returns the customer's CRM events merged with the messages
// of every conversation their identities own, newest first, capped at limit.
//
// Both legs are fetched at `limit` and merged in Go rather than UNIONed in SQL:
// crm_timeline and inbox_messages_v have no shared shape, and a UNION would
// force every message column onto the CRM leg (and vice versa) purely to
// satisfy the compiler.
func (s *Store) CustomerTimeline(ctx context.Context, orgID, customerID uuid.UUID, limit int) ([]TimelineEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]TimelineEntry, 0, limit*2)

	rows, err := s.db.Query(ctx, `
		SELECT id, kind, actor_user_id, summary, detail, occurred_at
		FROM crm_timeline
		WHERE organization_id = $1 AND customer_id = $2
		ORDER BY occurred_at DESC LIMIT $3`, orgID, customerID, limit)
	if err != nil {
		return nil, wrap("timeline events", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		e := TimelineEntry{Source: TimelineSourceCRM}
		if err := rows.Scan(&e.ID, &e.Kind, &e.ActorUserID, &e.Summary, &e.Detail, &e.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	msgRows, err := s.db.Query(ctx, `
		SELECT m.id, m.chat_id, m.channel, m.direction, m.sender_kind, m.body,
			COALESCE(m.message_ts, m.created_at)
		FROM inbox_messages_v m
		JOIN inbox_chats_v c ON c.id = m.chat_id
		JOIN crm_customer_identities ci ON ci.channel = c.channel AND ci.contact_id = c.contact_id
		WHERE c.organization_id = $1 AND ci.customer_id = $2
		ORDER BY COALESCE(m.message_ts, m.created_at) DESC LIMIT $3`, orgID, customerID, limit)
	if err != nil {
		return nil, wrap("timeline messages", err)
	}
	defer func() { _ = msgRows.Close() }()
	for msgRows.Next() {
		e := TimelineEntry{Source: TimelineSourceMessage, Kind: "message"}
		if err := msgRows.Scan(&e.ID, &e.ChatID, &e.Channel, &e.Direction, &e.SenderKind,
			&e.Body, &e.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := msgRows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].OccurredAt.After(out[j].OccurredAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
