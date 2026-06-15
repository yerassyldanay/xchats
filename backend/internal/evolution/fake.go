package evolution

import (
	"context"
	"sync"
)

// SentCall records one outbound Evolution call for test assertions
// (e.g. "we sent to the phone JID, not the @lid").
type SentCall struct {
	Action    string // sendText | sendMedia
	Number    string
	Text      string
	Mediatype string
	Mimetype  string
	FileName  string
	Caption   string
	Base64Len int
}

// Fake is an in-process Client used by component/e2e tests. It records every
// send and returns deterministic, monotonically-unique key ids.
type Fake struct {
	mu        sync.Mutex
	seq       int
	Calls     []SentCall
	Instances []Instance
	// FailNext, when set, makes the next send return this error.
	FailNext error
}

// NewFake returns a Fake with one connected instance.
func NewFake(instanceName, ownerJID string) *Fake {
	return &Fake{Instances: []Instance{{
		Name: instanceName, OwnerJID: ownerJID, ID: "fake-instance",
		ConnectionStatus: "open",
	}}}
}

func (f *Fake) nextID() string {
	f.seq++
	return "FAKEKEY" + pad(f.seq)
}

func (f *Fake) SendText(ctx context.Context, number, text string) (SendResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.takeFail(); err != nil {
		return SendResult{}, err
	}
	f.Calls = append(f.Calls, SentCall{Action: "sendText", Number: number, Text: text})
	return SendResult{KeyID: f.nextID(), Status: "PENDING"}, nil
}

func (f *Fake) SendMedia(ctx context.Context, number, mediatype, mimetype, base64Data, fileName, caption string) (SendResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.takeFail(); err != nil {
		return SendResult{}, err
	}
	f.Calls = append(f.Calls, SentCall{
		Action: "sendMedia", Number: number, Mediatype: mediatype, Mimetype: mimetype,
		FileName: fileName, Caption: caption, Base64Len: len(base64Data),
	})
	return SendResult{KeyID: f.nextID(), Status: "PENDING"}, nil
}

func (f *Fake) GetBase64(ctx context.Context, messageID string) (string, string, string, error) {
	// A tiny valid 1x1 PNG, base64, for the download fallback path.
	return tinyPNGBase64, "downloaded.png", "image/png", nil
}

func (f *Fake) FetchInstances(ctx context.Context) ([]Instance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Instance(nil), f.Instances...), nil
}

func (f *Fake) SetWebhook(ctx context.Context, url, tokenHeader, token string, events []string) error {
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
