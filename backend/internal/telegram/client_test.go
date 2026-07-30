package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testToken = "8123456789:AAH-test-token_do-not-log"

// apiServer stands in for api.telegram.org, capturing the request path and body
// of each call and replying with a canned Bot API envelope.
type apiServer struct {
	t      *testing.T
	srv    *httptest.Server
	paths  []string
	bodies []map[string]any
	reply  func(method string) (int, string)
}

func newAPIServer(t *testing.T) *apiServer {
	t.Helper()
	a := &apiServer{t: t}
	a.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.paths = append(a.paths, r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		var m map[string]any
		if len(body) > 0 {
			_ = json.Unmarshal(body, &m)
		}
		a.bodies = append(a.bodies, m)

		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		status, payload := 200, `{"ok":true,"result":{}}`
		if a.reply != nil {
			status, payload = a.reply(method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, payload)
	}))
	t.Cleanup(a.srv.Close)
	return a
}

func (a *apiServer) client(logTo io.Writer) *HTTP {
	return NewHTTP(a.srv.URL, slog.New(slog.NewTextHandler(logTo, &slog.HandlerOptions{Level: slog.LevelDebug})))
}

func (a *apiServer) bodyFor(method string) map[string]any {
	a.t.Helper()
	for i, p := range a.paths {
		if strings.HasSuffix(p, "/"+method) {
			return a.bodies[i]
		}
	}
	a.t.Fatalf("no call recorded for %s (paths: %v)", method, a.paths)
	return nil
}

func TestSetWebhookSendsSecretAllowedUpdatesAndDropFalse(t *testing.T) {
	a := newAPIServer(t)
	c := a.client(io.Discard)

	if err := c.SetWebhook(context.Background(), testToken,
		"https://example.test/telegram/api/v1/webhook/abc", "s3cret", DefaultAllowedUpdates, false); err != nil {
		t.Fatalf("SetWebhook: %v", err)
	}

	body := a.bodyFor("setWebhook")
	if got := body["secret_token"]; got != "s3cret" {
		t.Fatalf("secret_token = %v, want s3cret", got)
	}
	if got := body["url"]; got != "https://example.test/telegram/api/v1/webhook/abc" {
		t.Fatalf("url = %v", got)
	}
	// drop_pending_updates MUST be false on anything but a deliberate first
	// connect: a retry that drops the backlog silently loses real customer
	// messages that piled up while the webhook was broken.
	if got, ok := body["drop_pending_updates"].(bool); !ok || got {
		t.Fatalf("drop_pending_updates = %v, want false", body["drop_pending_updates"])
	}
	updates, _ := body["allowed_updates"].([]any)
	if len(updates) != 1 || updates[0] != "message" {
		t.Fatalf("allowed_updates = %v, want [message]", body["allowed_updates"])
	}
}

func TestSetWebhookOmitsSecretWhenUnset(t *testing.T) {
	a := newAPIServer(t)
	if err := a.client(io.Discard).SetWebhook(context.Background(), testToken,
		"https://example.test/hook", "", nil, false); err != nil {
		t.Fatalf("SetWebhook: %v", err)
	}
	if _, present := a.bodyFor("setWebhook")["secret_token"]; present {
		t.Fatal("secret_token was sent even though none is configured")
	}
}

func TestAPIErrorCarriesCodeAndDescription(t *testing.T) {
	a := newAPIServer(t)
	a.reply = func(string) (int, string) {
		return 401, `{"ok":false,"error_code":401,"description":"Unauthorized"}`
	}
	_, err := a.client(io.Discard).GetMe(context.Background(), testToken)
	if err == nil {
		t.Fatal("GetMe with a rejected token returned no error")
	}
	if !IsUnauthorized(err) {
		t.Fatalf("IsUnauthorized(%v) = false", err)
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("error %q does not carry Telegram's description", err)
	}
	if code := APIErrorCode(err); code != 401 {
		t.Fatalf("APIErrorCode = %d, want 401", code)
	}
}

// A 200 with ok:false is still a failure — Telegram reports application errors
// inside the envelope, not (only) in the status line.
func TestOkFalseOn200IsAnError(t *testing.T) {
	a := newAPIServer(t)
	a.reply = func(string) (int, string) {
		return 200, `{"ok":false,"error_code":400,"description":"Bad Request: bad webhook: HTTPS url must be provided"}`
	}
	err := a.client(io.Discard).SetWebhook(context.Background(), testToken, "http://insecure/hook", "", nil, false)
	if err == nil {
		t.Fatal("ok:false on a 200 was treated as success")
	}
	if !strings.Contains(err.Error(), "HTTPS url must be provided") {
		t.Fatalf("error %q lost the description operators need", err)
	}
}

func TestSendMessageReturnsMessageID(t *testing.T) {
	a := newAPIServer(t)
	a.reply = func(string) (int, string) {
		return 200, `{"ok":true,"result":{"message_id":4242,"chat":{"id":-100,"type":"private"}}}`
	}
	sent, err := a.client(io.Discard).SendMessage(context.Background(), testToken, 555, "привет")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if sent.MessageID != 4242 {
		t.Fatalf("MessageID = %d, want 4242", sent.MessageID)
	}
	body := a.bodyFor("sendMessage")
	if got := body["chat_id"]; got != float64(555) {
		t.Fatalf("chat_id = %v, want 555", got)
	}
	if got := body["text"]; got != "привет" {
		t.Fatalf("text = %v", got)
	}
	// No parse_mode: draft prose is not Markdown, and an unescaped '_' would 400.
	if _, present := body["parse_mode"]; present {
		t.Fatalf("parse_mode was sent: %v", body["parse_mode"])
	}
}

// The token is the URL path segment, so it leaks through anything that quotes
// the request URL. Nothing this client logs or returns may contain it.
func TestTokenNeverAppearsInLogsOrErrors(t *testing.T) {
	a := newAPIServer(t)
	a.reply = func(string) (int, string) {
		return 401, `{"ok":false,"error_code":401,"description":"Unauthorized"}`
	}
	var logs bytes.Buffer
	c := a.client(&logs)

	if _, err := c.GetMe(context.Background(), testToken); err != nil {
		if strings.Contains(err.Error(), testToken) {
			t.Fatalf("the bot token leaked into an API error: %q", err)
		}
	}
	if strings.Contains(logs.String(), testToken) {
		t.Fatalf("the bot token leaked into the logs: %q", logs.String())
	}

	// A transport failure is the dangerous one: net/http embeds the full URL.
	logs.Reset()
	dead := NewHTTP("http://127.0.0.1:1", slog.New(slog.NewTextHandler(&logs, nil)))
	_, err := dead.GetMe(context.Background(), testToken)
	if err == nil {
		t.Fatal("expected a transport error against a dead port")
	}
	if strings.Contains(err.Error(), testToken) {
		t.Fatalf("the bot token leaked into a transport error: %q", err)
	}
	if strings.Contains(logs.String(), testToken) {
		t.Fatalf("the bot token leaked into the logs: %q", logs.String())
	}
}

func TestGetWebhookInfoDecodesURL(t *testing.T) {
	a := newAPIServer(t)
	a.reply = func(string) (int, string) {
		return 200, `{"ok":true,"result":{"url":"https://example.test/hook","pending_update_count":3,"last_error_message":"boom"}}`
	}
	info, err := a.client(io.Discard).GetWebhookInfo(context.Background(), testToken)
	if err != nil {
		t.Fatalf("GetWebhookInfo: %v", err)
	}
	if info.URL != "https://example.test/hook" || info.PendingUpdateCount != 3 || info.LastErrorMessage != "boom" {
		t.Fatalf("decoded %+v", info)
	}
}

// --- update parsing --------------------------------------------------------

func TestParseUpdateTextMessage(t *testing.T) {
	u, err := ParseUpdate([]byte(`{"update_id":10,"message":{"message_id":5,"date":1700000000,
		"from":{"id":42,"is_bot":false,"first_name":"Иван","last_name":"Петров","username":"ivan"},
		"chat":{"id":42,"type":"private"},"text":"привет"}}`))
	if err != nil {
		t.Fatalf("ParseUpdate: %v", err)
	}
	if reason, ignorable := u.Ignorable(); ignorable {
		t.Fatalf("a plain text message was ignored: %s", reason)
	}
	if u.UpdateID != 10 || u.Message.MessageID != 5 || u.Message.Text != "привет" {
		t.Fatalf("decoded %+v", u.Message)
	}
	if got := u.Message.From.DisplayName(); got != "Иван Петров" {
		t.Fatalf("DisplayName = %q", got)
	}
	if u.Message.Attachment() != nil {
		t.Fatal("a text message reported an attachment")
	}
	if u.Message.Timestamp().IsZero() {
		t.Fatal("timestamp was not decoded")
	}
}

func TestParseUpdateCaptionedPhotoPicksLargestRendition(t *testing.T) {
	u, err := ParseUpdate([]byte(`{"update_id":11,"message":{"message_id":6,"date":1700000000,
		"from":{"id":42,"is_bot":false,"first_name":"Иван"},"chat":{"id":42,"type":"private"},
		"caption":"вот фото",
		"photo":[{"file_id":"small","file_unique_id":"su","width":90,"height":90,"file_size":100},
		         {"file_id":"big","file_unique_id":"bu","width":1280,"height":1280,"file_size":9000}]}}`))
	if err != nil {
		t.Fatalf("ParseUpdate: %v", err)
	}
	if _, ignorable := u.Ignorable(); ignorable {
		t.Fatal("a captioned photo was ignored")
	}
	if u.Message.MediaKind() != "image" {
		t.Fatalf("MediaKind = %q", u.Message.MediaKind())
	}
	f := u.Message.Attachment()
	if f == nil || f.FileID != "big" || f.FileUniqueID != "bu" {
		t.Fatalf("Attachment = %+v, want the largest rendition", f)
	}
}

func TestParseUpdateCaptionlessDocumentIsStillContent(t *testing.T) {
	u, err := ParseUpdate([]byte(`{"update_id":12,"message":{"message_id":7,"date":1700000000,
		"from":{"id":42,"is_bot":false,"first_name":"Иван"},"chat":{"id":42,"type":"private"},
		"document":{"file_id":"doc1","file_unique_id":"du","file_name":"счёт.pdf","mime_type":"application/pdf","file_size":2048}}}`))
	if err != nil {
		t.Fatalf("ParseUpdate: %v", err)
	}
	if reason, ignorable := u.Ignorable(); ignorable {
		t.Fatalf("a caption-less document was ignored: %s", reason)
	}
	f := u.Message.Attachment()
	if f == nil || f.Kind != "document" || f.FileName != "счёт.pdf" || f.Mimetype != "application/pdf" || f.Size != 2048 {
		t.Fatalf("Attachment = %+v", f)
	}
}

func TestIgnorableUpdates(t *testing.T) {
	cases := map[string]string{
		"non-message update":   `{"update_id":1,"edited_message":{"message_id":1}}`,
		"group chat":           `{"update_id":2,"message":{"message_id":1,"from":{"id":1},"chat":{"id":-100,"type":"group"},"text":"hi"}}`,
		"supergroup":           `{"update_id":3,"message":{"message_id":1,"from":{"id":1},"chat":{"id":-100,"type":"supergroup"},"text":"hi"}}`,
		"channel post":         `{"update_id":4,"message":{"message_id":1,"from":{"id":1},"chat":{"id":-100,"type":"channel"},"text":"hi"}}`,
		"service join":         `{"update_id":5,"message":{"message_id":1,"from":{"id":1},"chat":{"id":1,"type":"private"},"new_chat_members":[{"id":9}]}}`,
		"pinned":               `{"update_id":6,"message":{"message_id":1,"from":{"id":1},"chat":{"id":1,"type":"private"},"pinned_message":{"message_id":2}}}`,
		"bot sender":           `{"update_id":7,"message":{"message_id":1,"from":{"id":1,"is_bot":true},"chat":{"id":1,"type":"private"},"text":"hi"}}`,
		"empty content":        `{"update_id":8,"message":{"message_id":1,"from":{"id":1},"chat":{"id":1,"type":"private"}}}`,
		"no chat at all":       `{"update_id":9,"message":{"message_id":1,"from":{"id":1},"text":"hi"}}`,
		"completely unrelated": `{"update_id":10}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			u, err := ParseUpdate([]byte(body))
			if err != nil {
				t.Fatalf("ParseUpdate: %v", err)
			}
			if reason, ignorable := u.Ignorable(); !ignorable {
				t.Fatalf("update was ingested, want ignored (reason=%q)", reason)
			}
		})
	}
}

func TestParseUpdateRejectsGarbage(t *testing.T) {
	if _, err := ParseUpdate([]byte("not json")); err == nil {
		t.Fatal("ParseUpdate accepted non-JSON")
	}
}
