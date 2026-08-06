package telegram

import (
	"context"
	"net/http"
	"strings"
	"sync"
)

// Call records one Bot API call for test assertions. Token is recorded so a
// test can prove the RIGHT account's token was used — never so it can be
// logged; the fake is test-only.
type Call struct {
	Method         string
	Token          string
	URL            string
	Secret         string
	AllowedUpdates []string
	DropPending    bool
	ChatID         int64
	Text           string
	FileID         string
	FilePath       string
	Upload         Upload

	// Offset/LimitN/TimeoutSeconds are getUpdates' own parameters (LimitN
	// rather than Limit — Call already has no field collision risk, but the
	// name pairs visibly with GetUpdatesRequest.Limit's test assertions).
	Offset         int64
	LimitN         int
	TimeoutSeconds int
}

// Fake is an in-process Client for component tests. It records every call and
// returns deterministic ids.
type Fake struct {
	mu   sync.Mutex
	seq  int64
	Call []Call

	// Bot is what GetMe returns for any token not in BadTokens.
	Bot User
	// BadTokens make GetMe (and every other call) fail with a 401, the way a
	// revoked or mistyped token does.
	BadTokens map[string]bool
	// FailSetWebhook, when set, makes the next SetWebhook return it.
	FailSetWebhook error
	// FailDeleteWebhook, when set, makes the next DeleteWebhook return it.
	FailDeleteWebhook error
	// FailSend, when set, makes the next SendMessage return it.
	FailSend error
	// Info is what GetWebhookInfo reports.
	Info WebhookInfo
	// Files maps a file_id to the bytes DownloadFile returns for its path.
	Files map[string][]byte
	// FailGetFile / FailDownload make the media download path fail.
	FailGetFile  error
	FailDownload error

	// GetUpdatesFn, when set, gives a test full control over GetUpdates —
	// including blocking until ctx is done, to exercise a poller's
	// cancellation handling directly. Takes priority over PendingUpdates.
	GetUpdatesFn func(ctx context.Context, token string, req GetUpdatesRequest) ([]RawUpdate, error)
	// PendingUpdates is popped one batch per GetUpdates call (FIFO, only
	// when GetUpdatesFn is unset). Once empty, GetUpdates returns an empty,
	// non-error batch immediately — as if Telegram's long-poll simply timed
	// out with nothing queued — rather than blocking; a test that wants
	// blocking behavior sets GetUpdatesFn instead.
	PendingUpdates [][]RawUpdate
}

// NewFake returns a Fake whose GetMe reports the given bot.
func NewFake(botID int64, username string) *Fake {
	return &Fake{
		Bot:       User{ID: botID, IsBot: true, Username: username, FirstName: username},
		BadTokens: map[string]bool{},
		Files:     map[string][]byte{},
	}
}

func (f *Fake) record(c Call) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Call = append(f.Call, c)
}

// CallsFor returns every recorded call to method.
func (f *Fake) CallsFor(method string) []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Call
	for _, c := range f.Call {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// Reset clears the recorded calls.
func (f *Fake) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Call = nil
}

func (f *Fake) reject(token string) error {
	if f.BadTokens[token] {
		return &APIError{Code: http.StatusUnauthorized, Description: "Unauthorized"}
	}
	return nil
}

func (f *Fake) GetMe(ctx context.Context, token string) (User, error) {
	f.record(Call{Method: "getMe", Token: token})
	if err := f.reject(token); err != nil {
		return User{}, err
	}
	return f.Bot, nil
}

func (f *Fake) SetWebhook(ctx context.Context, token, url, secret string, allowedUpdates []string, dropPending bool) error {
	f.record(Call{Method: "setWebhook", Token: token, URL: url, Secret: secret,
		AllowedUpdates: allowedUpdates, DropPending: dropPending})
	if err := f.reject(token); err != nil {
		return err
	}
	f.mu.Lock()
	err := f.FailSetWebhook
	f.FailSetWebhook = nil
	f.mu.Unlock()
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.Info = WebhookInfo{URL: url, AllowedUpdates: allowedUpdates}
	f.mu.Unlock()
	return nil
}

func (f *Fake) DeleteWebhook(ctx context.Context, token string, dropPending bool) error {
	f.record(Call{Method: "deleteWebhook", Token: token, DropPending: dropPending})
	if err := f.reject(token); err != nil {
		return err
	}
	f.mu.Lock()
	err := f.FailDeleteWebhook
	f.FailDeleteWebhook = nil
	if err == nil {
		f.Info = WebhookInfo{}
	}
	f.mu.Unlock()
	return err
}

func (f *Fake) GetWebhookInfo(ctx context.Context, token string) (WebhookInfo, error) {
	f.record(Call{Method: "getWebhookInfo", Token: token})
	if err := f.reject(token); err != nil {
		return WebhookInfo{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Info, nil
}

func (f *Fake) SendMedia(ctx context.Context, token string, chatID int64, up Upload) (SentMessage, error) {
	method, _ := MediaMethod(up.Kind)
	f.record(Call{Method: method, Token: token, ChatID: chatID, Text: up.Caption, Upload: up})
	if err := f.reject(token); err != nil {
		return SentMessage{}, err
	}
	f.mu.Lock()
	err := f.FailSend
	f.FailSend = nil
	f.seq++
	id := 9000 + f.seq
	f.mu.Unlock()
	if err != nil {
		return SentMessage{}, err
	}
	return SentMessage{MessageID: id, ChatID: chatID}, nil
}

func (f *Fake) GetFile(ctx context.Context, token, fileID string) (FileInfo, error) {
	f.record(Call{Method: "getFile", Token: token, FileID: fileID})
	if err := f.reject(token); err != nil {
		return FileInfo{}, err
	}
	f.mu.Lock()
	err := f.FailGetFile
	data, known := f.Files[fileID]
	f.mu.Unlock()
	if err != nil {
		return FileInfo{}, err
	}
	if !known {
		return FileInfo{}, &APIError{Code: 400, Description: "Bad Request: file not found"}
	}
	return FileInfo{FileID: fileID, FilePath: "documents/" + fileID, FileSize: len(data)}, nil
}

func (f *Fake) DownloadFile(ctx context.Context, token, filePath string) ([]byte, error) {
	f.record(Call{Method: "downloadFile", Token: token, FilePath: filePath})
	if err := f.reject(token); err != nil {
		return nil, err
	}
	f.mu.Lock()
	err := f.FailDownload
	fileID := strings.TrimPrefix(filePath, "documents/")
	data, known := f.Files[fileID]
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if !known {
		return nil, &APIError{Code: 404, Description: "not found"}
	}
	return data, nil
}

func (f *Fake) SendMessage(ctx context.Context, token string, chatID int64, text string) (SentMessage, error) {
	f.record(Call{Method: "sendMessage", Token: token, ChatID: chatID, Text: text})
	if err := f.reject(token); err != nil {
		return SentMessage{}, err
	}
	f.mu.Lock()
	err := f.FailSend
	f.FailSend = nil
	f.seq++
	id := 9000 + f.seq
	f.mu.Unlock()
	if err != nil {
		return SentMessage{}, err
	}
	return SentMessage{MessageID: id, ChatID: chatID}, nil
}

// GetUpdates is the long-poll fake: GetUpdatesFn (if set) gets full control,
// else the next PendingUpdates batch is popped and returned, else an empty
// batch — never a real block, so a test that wants GetUpdates to hang until
// ctx is canceled must use GetUpdatesFn explicitly.
func (f *Fake) GetUpdates(ctx context.Context, token string, req GetUpdatesRequest) ([]RawUpdate, error) {
	f.record(Call{Method: "getUpdates", Token: token,
		Offset: req.Offset, LimitN: req.Limit, TimeoutSeconds: req.TimeoutSeconds})
	if err := f.reject(token); err != nil {
		return nil, err
	}
	f.mu.Lock()
	fn := f.GetUpdatesFn
	f.mu.Unlock()
	if fn != nil {
		return fn(ctx, token, req)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.PendingUpdates) == 0 {
		return nil, nil
	}
	batch := f.PendingUpdates[0]
	f.PendingUpdates = f.PendingUpdates[1:]
	return batch, nil
}

var _ Client = (*Fake)(nil)
