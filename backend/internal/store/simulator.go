package store

import (
	"context"

	"github.com/google/uuid"
)

// GetOrCreateSimulatorAccount returns the organization's simulator channel
// account, creating one (wa_accounts.channel='simulator') if none exists yet.
// One per organization — the simulator's stable owner_jid ("simulator:<org
// id>") makes this call idempotent under concurrent requests.
func (s *Store) GetOrCreateSimulatorAccount(ctx context.Context, orgID uuid.UUID) (Account, error) {
	var a Account
	err := s.pool.QueryRow(ctx, `
		INSERT INTO xchats.wa_accounts (id, organization_id, display_name, owner_jid, channel, connection_state)
		VALUES (uuid_generate_v4(), $1::uuid, 'Simulator', 'simulator:' || $1::text, 'simulator', 'connected')
		ON CONFLICT (owner_jid) DO UPDATE SET updated_at = now()
		RETURNING `+waAccountCols, orgID).Scan(scanWaAccountDst(&a)...)
	return a, err
}
