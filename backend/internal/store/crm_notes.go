package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// ---------------------------------------------------------------------------
// Internal customer notes
// ---------------------------------------------------------------------------
// Notes are internal only: nothing here is ever sent to a customer, and no
// channel adapter reads this table. They are also the ONLY notes model — there
// is deliberately no scalar notes column on crm_customers, so "the note" on
// the profile is always the latest row here rather than a second, drifting
// copy.

// CustomerNote is one authored, timestamped internal note. AuthorName is
// resolved by the read (join on users) so the UI does not need a second lookup
// to render «— Alice»; it is empty for a note whose author was since deleted.
type CustomerNote struct {
	ID           uuid.UUID
	CustomerID   uuid.UUID
	AuthorUserID uuid.NullUUID
	AuthorName   string
	Body         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

const noteCols = `n.id, n.customer_id, n.author_user_id,
	COALESCE(NULLIF(u.display_name, ''), u.email, '') AS author_name,
	n.body, n.created_at, n.updated_at`

func scanNoteDst(n *CustomerNote) []any {
	return []any{&n.ID, &n.CustomerID, &n.AuthorUserID, &n.AuthorName, &n.Body, &n.CreatedAt, &n.UpdatedAt}
}

// ListCustomerNotes returns a customer's notes, newest first.
func (s *Store) ListCustomerNotes(ctx context.Context, orgID, customerID uuid.UUID) ([]CustomerNote, error) {
	rows, err := s.db.Query(ctx, `SELECT `+noteCols+`
		FROM crm_customer_notes n LEFT JOIN users u ON u.id = n.author_user_id
		WHERE n.organization_id = $1 AND n.customer_id = $2
		ORDER BY n.created_at DESC`, orgID, customerID)
	if err != nil {
		return nil, wrap("list notes", err)
	}
	defer func() { _ = rows.Close() }()
	out := []CustomerNote{}
	for rows.Next() {
		var n CustomerNote
		if err := rows.Scan(scanNoteDst(&n)...); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// LatestCustomerNote returns the customer's most recent note, or ErrNotFound
// when they have none. This is what the sidebar's «Заметка» block renders and
// what the AI customer-context block quotes.
func (s *Store) LatestCustomerNote(ctx context.Context, orgID, customerID uuid.UUID) (CustomerNote, error) {
	var n CustomerNote
	err := s.db.QueryRow(ctx, `SELECT `+noteCols+`
		FROM crm_customer_notes n LEFT JOIN users u ON u.id = n.author_user_id
		WHERE n.organization_id = $1 AND n.customer_id = $2
		ORDER BY n.created_at DESC LIMIT 1`, orgID, customerID).Scan(scanNoteDst(&n)...)
	if errors.Is(err, dbx.ErrNoRows) {
		return n, ErrNotFound
	}
	return n, wrap("latest note", err)
}

// AddCustomerNote writes a note and its timeline event in one transaction —
// a note that is not on the timeline would be invisible in the very place the
// product asks for it.
func (s *Store) AddCustomerNote(ctx context.Context, orgID, customerID uuid.UUID, author uuid.NullUUID, body string) (CustomerNote, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return CustomerNote{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := customerExists(ctx, tx, orgID, customerID); err != nil {
		return CustomerNote{}, err
	}
	var id uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO crm_customer_notes (organization_id, customer_id, author_user_id, body)
		VALUES ($1, $2, $3, $4) RETURNING id`, orgID, customerID, author, body).Scan(&id); err != nil {
		return CustomerNote{}, wrap("add note", err)
	}
	if err := appendTimeline(ctx, tx, orgID, customerID, timelineEvent{
		Kind: TimelineNoteAdded, Actor: author, Summary: body,
		Detail: jsonObject(map[string]any{"note_id": id.String()}),
	}); err != nil {
		return CustomerNote{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CustomerNote{}, err
	}
	return s.customerNoteByID(ctx, orgID, id)
}

// UpdateCustomerNote edits a note's text. The timeline event keeps its
// original wording: the timeline is a record of what happened, not a mirror of
// current state.
func (s *Store) UpdateCustomerNote(ctx context.Context, orgID, noteID uuid.UUID, body string) (CustomerNote, error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE crm_customer_notes SET body = $3, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE organization_id = $1 AND id = $2`, orgID, noteID, body)
	if err != nil {
		return CustomerNote{}, wrap("update note", err)
	}
	if tag.RowsAffected() == 0 {
		return CustomerNote{}, ErrNotFound
	}
	return s.customerNoteByID(ctx, orgID, noteID)
}

func (s *Store) DeleteCustomerNote(ctx context.Context, orgID, noteID uuid.UUID) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM crm_customer_notes WHERE organization_id = $1 AND id = $2`, orgID, noteID)
	if err != nil {
		return wrap("delete note", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) customerNoteByID(ctx context.Context, orgID, id uuid.UUID) (CustomerNote, error) {
	var n CustomerNote
	err := s.db.QueryRow(ctx, `SELECT `+noteCols+`
		FROM crm_customer_notes n LEFT JOIN users u ON u.id = n.author_user_id
		WHERE n.organization_id = $1 AND n.id = $2`, orgID, id).Scan(scanNoteDst(&n)...)
	if errors.Is(err, dbx.ErrNoRows) {
		return n, ErrNotFound
	}
	return n, wrap("load note", err)
}

// customerExists is the org guard every customer-scoped write shares: it turns
// "this id belongs to another organization" into ErrNotFound before the write
// happens, so a cross-tenant id can never insert a child row.
func customerExists(ctx context.Context, tx *dbx.Tx, orgID, customerID uuid.UUID) error {
	var one int
	err := tx.QueryRow(ctx, `
		SELECT 1 FROM crm_customers
		WHERE organization_id = $1 AND id = $2 AND merged_into_id IS NULL`, orgID, customerID).Scan(&one)
	if errors.Is(err, dbx.ErrNoRows) {
		return ErrNotFound
	}
	return wrap("check customer", err)
}
