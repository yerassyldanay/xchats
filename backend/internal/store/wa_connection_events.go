package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// WAConnectionEvent is one durable row of a WhatsApp account's connection
// history — see migrations/sqlite/0020_campaign_diagnostics.up.sql's own
// doc comment for why this exists alongside wa_accounts.connection_state
// (which only ever holds the CURRENT transition, not history).
type WAConnectionEvent struct {
	ID             uuid.UUID
	AccountID      uuid.UUID
	OrganizationID uuid.NullUUID
	Event          string
	Reason         string
	CreatedAt      time.Time
}

// RecordWAConnectionEvent appends one connection-history row for accountID.
// organizationID is denormalized purely for a direct, join-free query — see
// the migration's own doc comment: callers must still resolve account
// ownership through the existing org-scoped account lookup before trusting
// anything read back from this table, never this column alone. Called from
// internal/whatsmeow's setConnectionState, the single choke point every
// real connect/disconnect/logged-out transition already flows through.
func (s *Store) RecordWAConnectionEvent(ctx context.Context, accountID uuid.UUID, organizationID uuid.NullUUID, event, reason string) error {
	var orgArg any
	if organizationID.Valid {
		orgArg = organizationID.UUID
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO wa_account_connection_events (account_id, organization_id, event, reason)
		VALUES ($1, $2, $3, $4)`, accountID, orgArg, event, reason)
	return wrap("record wa connection event", err)
}

// ListWAConnectionEventsForAccount returns accountID's connection history,
// newest first, capped at limit — the whatsapp_connection_status MCP tool's
// "relevant disconnect/reconnect history", and diagnose_campaign's
// "historical connection evidence" once further filtered to a time window
// by the caller. Authorization is the caller's responsibility (resolve
// account ownership first, exactly like every other account-scoped query in
// this package) — this method trusts accountID alone.
func (s *Store) ListWAConnectionEventsForAccount(ctx context.Context, accountID uuid.UUID, limit int) ([]WAConnectionEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, account_id, organization_id, event, reason, created_at
		FROM wa_account_connection_events WHERE account_id = $1
		ORDER BY created_at DESC LIMIT $2`, accountID, limit)
	if err != nil {
		return nil, wrap("list wa connection events", err)
	}
	defer func() { _ = rows.Close() }()
	var out []WAConnectionEvent
	for rows.Next() {
		var e WAConnectionEvent
		if err := rows.Scan(&e.ID, &e.AccountID, &e.OrganizationID, &e.Event, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ConnectionEventsInWindow returns accountID's connection events that fall
// within [from, to] — diagnose_campaign's "relevant connection evidence",
// scoped to the campaign's own active window rather than the account's
// entire history, so one campaign's diagnostics never surface a disconnect
// that happened months apart from anything it sent.
func (s *Store) ConnectionEventsInWindow(ctx context.Context, accountID uuid.UUID, from, to time.Time, limit int) ([]WAConnectionEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, account_id, organization_id, event, reason, created_at
		FROM wa_account_connection_events
		WHERE account_id = $1 AND created_at >= $2 AND created_at <= $3
		ORDER BY created_at ASC LIMIT $4`, accountID, from, to, limit)
	if err != nil {
		return nil, wrap("list wa connection events in window", err)
	}
	defer func() { _ = rows.Close() }()
	var out []WAConnectionEvent
	for rows.Next() {
		var e WAConnectionEvent
		if err := rows.Scan(&e.ID, &e.AccountID, &e.OrganizationID, &e.Event, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
