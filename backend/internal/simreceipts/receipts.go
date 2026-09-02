// Package simreceipts drives a successful Simulator send's delivery_state
// forward over time — the async counterpart to internal/simulator's
// ChannelSender.Send. Deliberately a SEPARATE package from
// internal/simulator (not a subpackage of it either — that would still be
// swept by "go list -deps ./..." run from within internal/simulator):
// internal/simulator's own test structurally proves ChannelSender/
// ChannelDecoder have no networking capability by construction, by
// asserting nothing in that package's dependency closure touches net/
// net/http/etc; ReceiptSimulator legitimately depends on internal/store
// (to read and update message rows), and putting it in the same package
// would silently widen that closure and break the guarantee the other
// package's test exists to enforce, even though this package still never
// touches the network either — it only writes to SQLite.
package simreceipts

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/simulator"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// Broadcaster is the realtime surface ReceiptSimulator needs — satisfied by
// *realtime.Hub with zero adapter code, matching internal/outbound.Deliver's
// own Broadcaster (message.updated is the exact event a real delivery
// receipt webhook would also trigger).
type Broadcaster interface {
	Broadcast(name string, data any)
}

// Config is ReceiptSimulator's runtime tunables. NewReceiptSimulator fills
// in short defaults for every zero-valued field — fast enough that a
// browser e2e test watching a campaign complete doesn't have to wait
// anywhere close to how long a REAL delivery/read receipt can take (seconds
// to indefinite, or never), while still being genuinely asynchronous rather
// than instantaneous: the point is exercising the same receipt-driven
// AdvanceDeliveryState path a real channel's webhook uses, not skipping it.
type Config struct {
	// SweepEvery is how often the sweep runs.
	SweepEvery time.Duration
	// DeliveredAfter is how long a message sits at 'sent' before the sweep
	// advances it to 'delivered' — every successful send reaches this
	// state eventually (see Outcome's own doc comment: OutcomeFailed never
	// reaches 'sent' in the first place).
	DeliveredAfter time.Duration
	// ReadAfter is how long a message sits at 'delivered' before the sweep
	// advances it to 'read' — only for destinations whose fixed Outcome is
	// OutcomeRead; OutcomeDeliveredOnly destinations stay at 'delivered'
	// forever, same as a real recipient who never opens the message.
	ReadAfter time.Duration
}

func (c Config) withDefaults() Config {
	if c.SweepEvery <= 0 {
		c.SweepEvery = 500 * time.Millisecond
	}
	if c.DeliveredAfter <= 0 {
		c.DeliveredAfter = 1 * time.Second
	}
	if c.ReadAfter <= 0 {
		c.ReadAfter = 2 * time.Second
	}
	return c
}

// ReceiptSimulator drives a successful simulator send's delivery_state from
// 'sent' through 'delivered' and, for most destinations, 'read' —
// asynchronously, exactly like a real channel's own delivery-receipt
// webhook drives store.Store.AdvanceDeliveryState (see that method's own
// doc comment: "today's only callers are all WhatsApp (whatsmeow
// receipts)" — this is simulator's own such caller, not a parallel
// mechanism). Which destinations ever reach 'read' at all is fixed by
// Outcome, not decided here — this only ever ADVANCES, never re-derives,
// what Send already committed to.
type ReceiptSimulator struct {
	Store *store.Store
	Hub   Broadcaster
	Cfg   Config
	Log   *slog.Logger

	stop chan struct{}
	done chan struct{}
}

// NewReceiptSimulator builds a ReceiptSimulator ready for Start.
func NewReceiptSimulator(st *store.Store, hub Broadcaster, cfg Config, log *slog.Logger) *ReceiptSimulator {
	if log == nil {
		log = slog.Default()
	}
	return &ReceiptSimulator{Store: st, Hub: hub, Cfg: cfg.withDefaults(), Log: log, stop: make(chan struct{}), done: make(chan struct{})}
}

// Start launches the sweep loop; it stops once ctx is done or Stop is
// called. Call Stop (mirroring internal/campaign.Scheduler's own Start/Stop
// contract) to block until the loop has actually exited, before anything
// Store/Hub depend on is torn down.
func (r *ReceiptSimulator) Start(ctx context.Context) {
	go func() {
		defer close(r.done)
		ticker := time.NewTicker(r.Cfg.SweepEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-r.stop:
				return
			case <-ticker.C:
				r.sweep(ctx)
			}
		}
	}()
}

// Stop blocks until the loop Start launched has exited.
func (r *ReceiptSimulator) Stop() {
	close(r.stop)
	<-r.done
}

// sweep runs the two independent passes described on Config's own fields.
// Each candidate is advanced individually (rather than one bulk UPDATE) so
// one row's failure never blocks the rest, and so every real change still
// goes through AdvanceDeliveryState's own monotonic guard and gets its own
// message.updated broadcast — identical to how one real receipt webhook
// call handles one message at a time.
func (r *ReceiptSimulator) sweep(ctx context.Context) {
	now := time.Now()
	r.advance(ctx, now.Add(-r.Cfg.DeliveredAfter), "sent", "delivered", 2, nil)
	r.advance(ctx, now.Add(-r.Cfg.ReadAfter), "delivered", "read", 3, func(destination string) bool {
		return simulator.SimulatedOutcome(destination) == simulator.OutcomeRead
	})
}

func (r *ReceiptSimulator) advance(ctx context.Context, olderThan time.Time, fromState, toState string, toRank int, eligible func(destination string) bool) {
	candidates, err := r.Store.SimulatorMessagesAwaitingReceipt(ctx, olderThan)
	if err != nil {
		r.Log.Error("simulator: list receipt candidates failed", "err", err)
		return
	}
	for _, c := range candidates {
		if c.DeliveryState != fromState {
			continue
		}
		if eligible != nil && !eligible(c.Destination) {
			continue
		}
		msgID, chatID, err := r.Store.AdvanceDeliveryState(ctx, "simulator", c.AccountID, c.ExternalMessageID, toState, toRank)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				r.Log.Error("simulator: advance delivery state failed", "message_id", c.MessageID, "to", toState, "err", err)
			}
			continue
		}
		r.emitMessage(ctx, msgID, chatID)
	}
}

func (r *ReceiptSimulator) emitMessage(ctx context.Context, messageID, _ uuid.UUID) {
	msg, err := r.Store.MessageByID(ctx, messageID)
	if err != nil {
		r.Log.Error("simulator: load message for broadcast failed", "message_id", messageID, "err", err)
		return
	}
	r.Hub.Broadcast("message.updated", dto.MapMessage(msg))
}
