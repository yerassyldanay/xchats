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
	mu         sync.Mutex
	handlers   []wm.EventHandler
	connected  bool
	connectErr error
	logoutErr  error
	msgSeq     int
	sent       []fakeSentMsg
	sendErr    error
	uploadResp wm.UploadResponse
	uploadErr  error
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
func (f *fakeWAClient) AddEventHandler(h wm.EventHandler) uint32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers = append(f.handlers, h)
	return uint32(len(f.handlers))
}

// fire delivers evt to every handler registered so far, synchronously — the
// same guarantee whatsmeow's own dispatch makes to a *single* handler, which
// is what lets a test assert on state immediately after calling fire.
func (f *fakeWAClient) fire(evt any) {
	f.mu.Lock()
	hs := append([]wm.EventHandler(nil), f.handlers...)
	f.mu.Unlock()
	for _, h := range hs {
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
