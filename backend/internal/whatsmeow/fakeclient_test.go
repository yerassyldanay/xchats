package whatsmeow

import (
	"context"
	"fmt"
	"sync"

	wm "go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// fakeWAClient is a waClient test double: no real network, every call
// recorded, event delivery driven entirely by fire() — so a test can play
// PairSuccess/Message/Receipt/... exactly as whatsmeow's own dispatch would.
type fakeWAClient struct {
	// handlersMu mirrors whatsmeow's own eventHandlersLock exactly: an
	// RWMutex guarding ONLY the handler list, read-locked for the whole
	// duration of a dispatch and write-locked to register. Keeping it
	// separate from mu is what the real client does too, so a handler can
	// freely call SendMessage/IsConnected/... while still being unable to
	// register a new handler mid-dispatch. See AddEventHandler.
	handlersMu sync.RWMutex
	handlers   []wm.EventHandler

	mu         sync.Mutex
	connected  bool
	connectErr error
	logoutErr  error
	msgSeq     int
	sent       []fakeSentMsg
	sendErr    error
	uploadResp wm.UploadResponse
	uploadErr  error

	isOnWhatsAppResp    []types.IsOnWhatsAppResponse
	isOnWhatsAppErr     error
	isOnWhatsAppQueries [][]string // every call's phones, for assertions
}

type fakeSentMsg struct {
	to  types.JID
	msg *waE2E.Message
	id  types.MessageID
}

func newFakeWAClient() *fakeWAClient { return &fakeWAClient{} }

var _ waClient = (*fakeWAClient)(nil)

func (f *fakeWAClient) Connect() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.connectErr != nil {
		return f.connectErr
	}
	f.connected = true
	return nil
}

func (f *fakeWAClient) Disconnect() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connected = false
}

func (f *fakeWAClient) IsConnected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connected
}

func (f *fakeWAClient) IsLoggedIn() bool { return f.IsConnected() }

func (f *fakeWAClient) Logout(ctx context.Context) error {
	f.Disconnect()
	return f.logoutErr
}

// AddEventHandler records the handler; fire() is what actually invokes it.
//
// Registering a handler from INSIDE a dispatch deadlocks the real client:
// whatsmeow's dispatchEvent holds eventHandlersLock.RLock() for the whole
// callback while AddEventHandler wants the write lock on that same RWMutex,
// so the dispatch goroutine wedges permanently and the account stops
// receiving everything (whatsmeow documents this hazard on
// RemoveEventHandler). This fake originally copied the handler slice and
// released its lock BEFORE dispatching, which made the illegal call legal
// here and let exactly that deadlock ship green. Reproducing whatsmeow's
// locking verbatim is what gives the pairing tests below their teeth —
// see waitFor's failure message.
func (f *fakeWAClient) AddEventHandler(h wm.EventHandler) uint32 {
	f.handlersMu.Lock()
	defer f.handlersMu.Unlock()
	f.handlers = append(f.handlers, h)
	return uint32(len(f.handlers))
}

// fire delivers evt to every handler registered so far, synchronously and
// while holding the read lock — matching whatsmeow's dispatchEvent both in
// the ordering guarantee (a test can assert on state right after fire
// returns) and in the locking that makes mid-dispatch registration illegal.
func (f *fakeWAClient) fire(evt any) {
	f.handlersMu.RLock()
	defer f.handlersMu.RUnlock()
	for _, h := range f.handlers {
		h(evt)
	}
}

func (f *fakeWAClient) GetQRChannel(ctx context.Context) (<-chan wm.QRChannelItem, error) {
	return make(chan wm.QRChannelItem, 8), nil
}

func (f *fakeWAClient) SendMessage(ctx context.Context, to types.JID, msg *waE2E.Message, extra ...wm.SendRequestExtra) (wm.SendResponse, error) {
	if f.sendErr != nil {
		return wm.SendResponse{}, f.sendErr
	}
	var id types.MessageID
	if len(extra) > 0 && extra[0].ID != "" {
		id = extra[0].ID
	}
	f.mu.Lock()
	f.sent = append(f.sent, fakeSentMsg{to: to, msg: msg, id: id})
	f.mu.Unlock()
	return wm.SendResponse{ID: id}, nil
}

func (f *fakeWAClient) GenerateMessageID() types.MessageID {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgSeq++
	return types.MessageID(fmt.Sprintf("FAKEID%d", f.msgSeq))
}

func (f *fakeWAClient) Upload(ctx context.Context, data []byte, appInfo wm.MediaType) (wm.UploadResponse, error) {
	return f.uploadResp, f.uploadErr
}

func (f *fakeWAClient) Download(ctx context.Context, msg wm.DownloadableMessage) ([]byte, error) {
	return nil, fmt.Errorf("fakeWAClient: Download not configured")
}

// IsOnWhatsApp returns whatever isOnWhatsAppResp/isOnWhatsAppErr a test
// configured, and records the queried phones for assertions.
func (f *fakeWAClient) IsOnWhatsApp(ctx context.Context, phones []string) ([]types.IsOnWhatsAppResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.isOnWhatsAppQueries = append(f.isOnWhatsAppQueries, append([]string(nil), phones...))
	return f.isOnWhatsAppResp, f.isOnWhatsAppErr
}
