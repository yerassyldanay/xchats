package evolution

import (
	"context"
	"sync"
)

// SentCall records one outbound Evolution call for test assertions
// (e.g. "we sent to the phone JID, not the @lid", "on the right instance").
type SentCall struct {
	Action    string // sendText | sendMedia
	Instance  string
	Number    string
	Text      string
	Mediatype string
	Mimetype  string
	FileName  string
	Caption   string
	Base64Len int
}

// Fake is an in-process Client used by component/e2e tests. It records every send
// and lifecycle call and returns deterministic, monotonically-unique key ids.
type Fake struct {
	mu        sync.Mutex
	seq       int
	Calls     []SentCall
	Instances []Instance
	// FailNext, when set, makes the next send return this error.
	FailNext error
	// NotOnWhatsApp, when set, makes OnWhatsApp report numbers as unregistered.
	NotOnWhatsApp bool

	// --- lifecycle knobs ---
	ConnState   string   // configurable connection state ConnectionState returns (default "open")
	QR          QR       // canned QR ConnectQR returns
	NewOwnerJID string   // owner_jid a freshly created instance reports once "connected"
	Created     []string // instance names passed to CreateInstance
	Deleted     []string // instance names passed to Delete/LogoutInstance
}

// NewFake returns a Fake with one connected instance and canned lifecycle defaults.
func NewFake(instanceName, ownerJID string) *Fake {
	return &Fake{
		Instances: []Instance{{
			Name: instanceName, OwnerJID: ownerJID, ID: "fake-instance",
			ConnectionStatus: "open",
		}},
		ConnState:   "open",
		QR:          QR{Code: "2@FAKEQRCODE", Base64: "data:image/png;base64," + tinyPNGBase64, PairingCode: "FAKEPAIR"},
		NewOwnerJID: "77022222222@s.whatsapp.net",
	}
}

func (f *Fake) nextID() string {
	f.seq++
	return "FAKEKEY" + pad(f.seq)
}

func (f *Fake) SendText(ctx context.Context, instance, number, text string) (SendResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.takeFail(); err != nil {
		return SendResult{}, err
	}
	f.Calls = append(f.Calls, SentCall{Action: "sendText", Instance: instance, Number: number, Text: text})
	return SendResult{KeyID: f.nextID(), Status: "PENDING"}, nil
}

func (f *Fake) SendMedia(ctx context.Context, instance, number, mediatype, mimetype, base64Data, fileName, caption string) (SendResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.takeFail(); err != nil {
		return SendResult{}, err
	}
	f.Calls = append(f.Calls, SentCall{
		Action: "sendMedia", Instance: instance, Number: number, Mediatype: mediatype, Mimetype: mimetype,
		FileName: fileName, Caption: caption, Base64Len: len(base64Data),
	})
	return SendResult{KeyID: f.nextID(), Status: "PENDING"}, nil
}

func (f *Fake) GetBase64(ctx context.Context, instance, messageID string) (string, string, string, error) {
	// A tiny valid 1x1 PNG, base64, for the download fallback path.
	return tinyPNGBase64, "downloaded.png", "image/png", nil
}

func (f *Fake) FetchInstances(ctx context.Context) ([]Instance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Instance(nil), f.Instances...), nil
}

func (f *Fake) SetWebhook(ctx context.Context, instance, url, tokenHeader, token string, events []string) error {
	return nil
}

// OnWhatsApp reports a number as registered by default. Set NotOnWhatsApp to
// simulate a non-WhatsApp number.
func (f *Fake) OnWhatsApp(ctx context.Context, instance, number string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.NotOnWhatsApp, nil
}

// CreateInstance records the name and adds a (configurably-stated) instance so a
// subsequent ConnectionState/FetchInstances can drive the connect flow offline.
func (f *Fake) CreateInstance(ctx context.Context, instance, displayName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.takeFail(); err != nil {
		return err
	}
	f.Created = append(f.Created, instance)
	f.Instances = append(f.Instances, Instance{
		Name: instance, OwnerJID: f.NewOwnerJID, ConnectionStatus: f.ConnState, ID: "fake-" + instance,
	})
	return nil
}

func (f *Fake) ConnectQR(ctx context.Context, instance string) (QR, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.takeFail(); err != nil {
		return QR{}, err
	}
	return f.QR, nil
}

func (f *Fake) ConnectionState(ctx context.Context, instance string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ConnState, nil
}

func (f *Fake) DeleteInstance(ctx context.Context, instance string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Deleted = append(f.Deleted, instance)
	for i := range f.Instances {
		if f.Instances[i].Name == instance {
			f.Instances = append(f.Instances[:i], f.Instances[i+1:]...)
			break
		}
	}
	return nil
}

func (f *Fake) LogoutInstance(ctx context.Context, instance string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Deleted = append(f.Deleted, instance)
	return nil
}

func (f *Fake) takeFail() error {
	if f.FailNext != nil {
		err := f.FailNext
		f.FailNext = nil
		return err
	}
	return nil
}

// CallsFor returns the recorded calls for an action.
func (f *Fake) CallsFor(action string) []SentCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []SentCall
	for _, c := range f.Calls {
		if c.Action == action {
			out = append(out, c)
		}
	}
	return out
}

func pad(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0000"
	}
	buf := []byte("0000")
	i := len(buf) - 1
	for n > 0 && i >= 0 {
		buf[i] = digits[n%10]
		n /= 10
		i--
	}
	return string(buf)
}

const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
