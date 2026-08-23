package whatsmeow

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	wm "go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// waClient is the seam this package depends on instead of *whatsmeow.Client
// directly, so unit tests can substitute a fake with no real network or
// device store. *whatsmeow.Client satisfies this interface as-is — no
// wrapper needed, since every method below has an identical signature to the
// real client's.
type waClient interface {
	Connect() error
	Disconnect()
	IsConnected() bool
	IsLoggedIn() bool
	Logout(ctx context.Context) error
	AddEventHandler(handler wm.EventHandler) uint32
	GetQRChannel(ctx context.Context) (<-chan wm.QRChannelItem, error)
	SendMessage(ctx context.Context, to types.JID, message *waE2E.Message, extra ...wm.SendRequestExtra) (wm.SendResponse, error)
	GenerateMessageID() types.MessageID
	Upload(ctx context.Context, data []byte, appInfo wm.MediaType) (wm.UploadResponse, error)
	Download(ctx context.Context, msg wm.DownloadableMessage) ([]byte, error)
	// IsOnWhatsApp batch-checks phone registration — Campaigns' preview-time
	// reachability check (an "invalid: not registered on WhatsApp" bucket)
	// and its send-time permanent-failure classification both go through
	// Manager.IsOnWhatsApp, which calls this.
	IsOnWhatsApp(ctx context.Context, phones []string) ([]types.IsOnWhatsAppResponse, error)
}

var _ waClient = (*wm.Client)(nil)

// eventBacklog bounds a managed client's own inbound-event queue. whatsmeow
// calls the registered handler synchronously on its own dispatch goroutine;
// a handler that blocks there would stall receipts and pings for that same
// connection, so enqueue() only ever does a non-blocking send and the
// managedClient's own goroutine does the real (store/blob/queue) work.
const eventBacklog = 256

// managedClient owns one account's live whatsmeow session: the client
// itself, and the backpressure queue + goroutine that decouples whatsmeow's
// own dispatch thread from ingest work. accountID is mutable only in the
// narrow pairing handoff window (see manager.go's completePairing) and is
// otherwise read-only for the lifetime of the client.
type managedClient struct {
	accountID string
	client    waClient
	log       *slog.Logger

	events   chan any
	done     chan struct{}
	stopOnce sync.Once
}

// newManagedClient wires cli's event handler to a bounded queue, starts the
// consumer goroutine (which calls mgr.handleEvent for every dequeued event),
// and returns the wrapper. It does not call Connect — callers do that once
// they are ready to receive events.
func newManagedClient(accountID string, cli waClient, mgr *Manager, log *slog.Logger) *managedClient {
	mc := &managedClient{
		accountID: accountID,
		client:    cli,
		log:       log,
		events:    make(chan any, eventBacklog),
		done:      make(chan struct{}),
	}
	cli.AddEventHandler(mc.enqueue)
	go mc.loop(mgr)
	return mc
}

// enqueue is the func(any) whatsmeow itself calls. It must never block.
func (mc *managedClient) enqueue(evt any) {
	select {
	case mc.events <- evt:
	default:
		mc.log.Error("whatsmeow event dropped: backpressure queue full",
			"account_id", mc.accountID, "event", fmt.Sprintf("%T", evt))
	}
}

// loop drains the backpressure queue on its own goroutine, so a slow store
// write or media download never blocks whatsmeow's dispatch thread.
func (mc *managedClient) loop(mgr *Manager) {
	for {
		select {
		case evt := <-mc.events:
			mgr.handleEvent(mc.accountID, evt)
		case <-mc.done:
			return
		}
	}
}

// stop ends the consumer goroutine. It does not disconnect the underlying
// client — callers that also want that call client.Disconnect() themselves.
//
// Idempotent and safe to call concurrently: Manager.Logout, Manager.Close,
// registerClient's replacement of a stale entry, and an external LoggedOut
// event can all reach the same managedClient, and a plain close() would
// panic on the second one ("close of closed channel") — taking the whole
// process down over a shutdown race.
func (mc *managedClient) stop() {
	mc.stopOnce.Do(func() { close(mc.done) })
}
