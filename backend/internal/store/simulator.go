package store

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// GetOrCreateSimulatorAccount returns the organization's simulator channel
// account, creating one (wa_accounts.channel='simulator') if none exists yet.
// One per organization — the simulator's stable owner_jid ("simulator:<org
// id>") makes this call idempotent under concurrent requests. The random id
// expression matches the one every migrations/sqlite/*.up.sql uuid PRIMARY
// KEY DEFAULT uses (wa_accounts.id itself has no DB-side default — it's
// normally the app-derived uuidv5(owner_jid) — so a fresh id is generated
// inline here exactly as the old uuid_generate_v4() call did).
func (s *Store) GetOrCreateSimulatorAccount(ctx context.Context, orgID uuid.UUID) (Account, error) {
	var a Account
	err := s.db.QueryRow(ctx, `
		INSERT INTO wa_accounts (id, organization_id, display_name, owner_jid, channel, connection_state)
		VALUES (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6))),
		        $1, 'Simulator', 'simulator:' || $1, 'simulator', 'connected')
		ON CONFLICT (owner_jid) DO UPDATE SET updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		RETURNING `+waAccountCols, orgID).Scan(scanWaAccountDst(&a)...)
	return a, err
}

// SimulatorPurgeResult reports what PurgeSimulatorData actually removed, so
// the caller (the Simulator page's own "Clear simulator data" action, KB-12)
// can confirm something real happened rather than a silent no-op.
type SimulatorPurgeResult struct {
	ConversationsDeleted int
	CustomersDeleted     int
}

// PurgeSimulatorData hard-deletes every conversation, message, draft, and
// CRM customer the organization's simulator account (GetOrCreateSimulatorAccount)
// ever produced — the Simulator page injects synthetic traffic through the
// SAME ingestion path a real message takes (see simulator.go's own doc
// comment), so every test send leaves a real wa_chats/crm_customers row
// behind; this is the "one-click cleanup action" side of KB-12.
//
// A CRM customer is only removed if EVERY identity it carries is simulator's
// own — an operator who manually merged a simulator test contact into a real
// customer (crm_merge.go) keeps that customer and its real identities
// intact; only the simulator identity (and its now-orphaned conversation)
// goes. Nothing outside this organization's own simulator account is ever
// touched: no real wa_/tg_/channel_ account is scoped by this query at all.
func (s *Store) PurgeSimulatorData(ctx context.Context, orgID uuid.UUID) (SimulatorPurgeResult, error) {
	var res SimulatorPurgeResult

	var simAccountID uuid.UUID
	err := s.db.QueryRow(ctx,
		`SELECT id FROM wa_accounts WHERE organization_id = $1 AND channel = 'simulator'`, orgID,
	).Scan(&simAccountID)
	if errors.Is(err, dbx.ErrNoRows) {
		return res, nil // never used the simulator — nothing to clean up
	}
	if err != nil {
		return res, wrap("find simulator account", err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// ai_drafts.chat_id is polymorphic (no FK — see 0003_ai_engine.up.sql's
	// file header), so this must run BEFORE wa_chats rows disappear below.
	if _, err := tx.Exec(ctx, `
		DELETE FROM ai_drafts WHERE channel = 'simulator'
			AND chat_id IN (SELECT id FROM wa_chats WHERE account_id = $1)`, simAccountID); err != nil {
		return res, wrap("purge simulator drafts", err)
	}

	// ai_kb_gap_events.chat_id has no FK either (same channel-neutral design
	// as ai_drafts above), and its draft_id is ON DELETE SET NULL rather than
	// CASCADE — so without this, every telemetry event a simulator test ever
	// produced would survive the drafts/chats/customers purge above and keep
	// permanently inflating KBGapReportFor's "real conversations" counts
	// (kbGapFilterClause's own channel != 'simulator' exclusion only affects
	// the aggregate report, not what's actually stored). organization_id
	// alone would over-match nothing else — 'simulator' is this narrow
	// account's own channel value — but scoping by both is free and explicit.
	if _, err := tx.Exec(ctx, `
		DELETE FROM ai_kb_gap_events WHERE organization_id = $1 AND channel = 'simulator'`, orgID); err != nil {
		return res, wrap("purge simulator kb-gap events", err)
	}

	// Simulator-only customers: cascades crm_customer_identities/notes/
	// followups/timeline/tags automatically (all ON DELETE CASCADE from
	// crm_customers — see 0013_crm.up.sql). A customer with a mix of
	// identities (a manual merge — crm_merge.go) keeps its real identities;
	// only its simulator identity/chat is gone as an ordinary orphaned
	// conversation once wa_chats is cleared below.
	custTag, err := tx.Exec(ctx, `
		DELETE FROM crm_customers WHERE organization_id = $1
			AND id IN (SELECT customer_id FROM crm_customer_identities WHERE channel = 'simulator' AND account_id = $2)
			AND NOT EXISTS (
				SELECT 1 FROM crm_customer_identities ci
				WHERE ci.customer_id = crm_customers.id AND NOT (ci.channel = 'simulator' AND ci.account_id = $2)
			)`, orgID, simAccountID)
	if err != nil {
		return res, wrap("purge simulator customers", err)
	}
	res.CustomersDeleted = int(custTag.RowsAffected())

	// A customer that survived the delete above (a mix of real + simulator
	// identities, from a manual merge) still keeps a dangling simulator
	// identity row pointing at a chat this function is about to delete below
	// — drop just that identity, not the customer it belongs to.
	if _, err := tx.Exec(ctx, `
		DELETE FROM crm_customer_identities WHERE channel = 'simulator' AND account_id = $1`, simAccountID); err != nil {
		return res, wrap("purge simulator identities", err)
	}

	chatTag, err := tx.Exec(ctx, `DELETE FROM wa_chats WHERE account_id = $1`, simAccountID)
	if err != nil {
		return res, wrap("purge simulator chats", err)
	}
	res.ConversationsDeleted = int(chatTag.RowsAffected())

	if _, err := tx.Exec(ctx, `DELETE FROM wa_contacts WHERE account_id = $1`, simAccountID); err != nil {
		return res, wrap("purge simulator contacts", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return res, err
	}
	return res, nil
}
