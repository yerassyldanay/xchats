// Package tunnelmock is an in-memory tunnel.Tunnel for cmd/xchats'
// mock-externals mode — Start/Stop/Status update in-memory state only, with
// no real ngrok SDK session, so the /settings/tunnel/* management surface
// (and the boot-time auto-start path) stays fully exercisable while
// profiling with zero outbound network dependency.
package tunnelmock

import (
	"context"
	"sync"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/tunnel"
)

// PublicURL is the fixed fake public origin every mock tunnel reports once
// started — shaped like a real ngrok free-tier domain.
const PublicURL = "https://mock-tunnel.ngrok-free.app"

// Tunnel is an in-memory tunnel.Tunnel. Zero value is ready to use.
type Tunnel struct {
	mu     sync.Mutex
	status tunnel.Status
}

// New returns a ready-to-use mock Tunnel, initially stopped.
func New() *Tunnel { return &Tunnel{} }

var _ tunnel.Tunnel = (*Tunnel)(nil)

func (t *Tunnel) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now()
	t.mu.Lock()
	t.status = tunnel.Status{Running: true, PublicURL: PublicURL, StartedAt: &now}
	t.mu.Unlock()
	return nil
}

func (t *Tunnel) Stop(ctx context.Context) error {
	t.mu.Lock()
	// Stopping preserves the StartedAt/PublicURL of the last run in every
	// other field EXCEPT Running — mirrors tunnel.Manager's own Status
	// shape (a stopped tunnel still reports what it last was, minus
	// LastError, which only a failed Start ever sets).
	t.status.Running = false
	t.mu.Unlock()
	return nil
}

func (t *Tunnel) Status() tunnel.Status {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.status
}
