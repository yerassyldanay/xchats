package store

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// ---------------------------------------------------------------------------
// Manual customer merge
// ---------------------------------------------------------------------------
// Merging is ALWAYS a deliberate operator action. Nothing in this package ever
// merges on name or phone similarity — ResolveCustomerForContact matches on the
// provider's own identity and nothing else — so "John Smith (Instagram)" and
// "John (WhatsApp)" stay two customers until someone says otherwise.
//
// The merge preserves everything the product brief lists: channel identities,
// conversations (which follow their identities automatically, since a chat is
// linked through crm_customer_identities rather than by a column of its own),
// notes, tags, timeline and follow-ups.

// MergeCustomers folds source into target, in one transaction, and returns the
// surviving customer.
//
// The source row is kept as a tombstone (merged_into_id set) rather than
// deleted: an id captured before the merge — a bookmarked URL, an in-flight
// request — still resolves to the survivor through CustomerByID, and the merge
// itself stays auditable. Every list query already filters tombstones out.
func (s *Store) MergeCustomers(ctx context.Context, orgID, targetID, sourceID uuid.UUID, actor uuid.NullUUID) (Customer, error) {
	if targetID == sourceID {
		return Customer{}, ErrMergeSameCustomer
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Customer{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	target, err := lockedCustomer(ctx, tx, orgID, targetID)
	if err != nil {
		return Customer{}, err
	}
	source, err := lockedCustomer(ctx, tx, orgID, sourceID)
	if err != nil {
		return Customer{}, err
	}

	// Re-point every child row. Identities carry the conversations with them,
	// which is why there is nothing chat-shaped in this list.
	for _, q := range []string{
		`UPDATE crm_customer_identities SET customer_id = $1,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE customer_id = $2`,
		`UPDATE crm_customer_notes SET customer_id = $1,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE customer_id = $2`,
		`UPDATE crm_followups SET customer_id = $1,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE customer_id = $2`,
		`UPDATE crm_timeline SET customer_id = $1 WHERE customer_id = $2`,
	} {
		if _, err := tx.Exec(ctx, q, targetID, sourceID); err != nil {
			return Customer{}, wrap("merge repoint", err)
		}
	}

	// Tags are a union, not a replace: a merge must never drop a label. The
	// composite primary key makes the overlap a no-op instead of a conflict.
	if _, err := tx.Exec(ctx, `
		INSERT INTO crm_customer_tags (customer_id, tag_id)
		SELECT $1, tag_id FROM crm_customer_tags WHERE customer_id = $2
		ON CONFLICT (customer_id, tag_id) DO NOTHING`, targetID, sourceID); err != nil {
		return Customer{}, wrap("merge tags", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM crm_customer_tags WHERE customer_id = $1`, sourceID); err != nil {
		return Customer{}, wrap("merge tags cleanup", err)
	}

	// Scalar fields: the target wins wherever it has a value, and the source
	// only fills blanks. Merging must not overwrite what an operator typed on
	// the customer they chose to keep.
	if _, err := tx.Exec(ctx, `
		UPDATE crm_customers SET
			display_name = CASE WHEN display_name = '' THEN $3 ELSE display_name END,
			phone = CASE WHEN phone = '' THEN $4 ELSE phone END,
			email = CASE WHEN email = '' THEN $5 ELSE email END,
			avatar_url = CASE WHEN avatar_url = '' THEN $6 ELSE avatar_url END,
			status_id = COALESCE(status_id, $7),
			assignee_user_id = COALESCE(assignee_user_id, $8),
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE organization_id = $1 AND id = $2`,
		orgID, targetID, source.DisplayName, source.Phone, source.Email, source.AvatarURL,
		source.StatusID, source.AssigneeUserID); err != nil {
		return Customer{}, wrap("merge scalars", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE crm_customers SET merged_into_id = $3,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE organization_id = $1 AND id = $2`, orgID, sourceID, targetID); err != nil {
		return Customer{}, wrap("mark merged", err)
	}

	name := source.DisplayName
	if name == "" {
		name = sourceID.String()
	}
	if err := appendTimeline(ctx, tx, orgID, targetID, timelineEvent{
		Kind: TimelineCustomersMerged, Actor: actor, Summary: name,
		Detail: jsonObject(map[string]any{
			"source_customer_id": sourceID.String(),
			"source_name":        source.DisplayName,
			"target_name":        target.DisplayName,
		}),
	}); err != nil {
		return Customer{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Customer{}, err
	}
	return s.CustomerByID(ctx, orgID, targetID)
}

// lockedCustomer reads one live customer inside the merge transaction. Every
// dbx transaction is immediate-mode (it takes the write lock upfront), so the
// read is already serialized against a concurrent merge — the name records the
// intent, not an extra lock statement.
func lockedCustomer(ctx context.Context, tx *dbx.Tx, orgID, id uuid.UUID) (Customer, error) {
	var c Customer
	err := tx.QueryRow(ctx, `SELECT `+customerColsBare+`
		FROM crm_customers WHERE organization_id = $1 AND id = $2`, orgID, id).Scan(scanCustomerDst(&c)...)
	if errors.Is(err, dbx.ErrNoRows) {
		return c, ErrNotFound
	}
	if err != nil {
		return c, wrap("load customer for merge", err)
	}
	// Merging into (or out of) a tombstone would fork the history it exists to
	// preserve; the caller must resolve to the survivor first.
	if c.MergedIntoID.Valid {
		return c, ErrNotFound
	}
	return c, nil
}
