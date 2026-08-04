package store

import (
	"context"

	"github.com/google/uuid"
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
