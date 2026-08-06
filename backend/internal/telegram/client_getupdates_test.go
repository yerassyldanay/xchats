package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

// --- HTTP.GetUpdates --------------------------------------------------------

func TestGetUpdatesDecodesArrayInAscendingOrderWithRawFidelity(t *testing.T) {
	a := newAPIServer(t)
	// Deliberately out of order — GetUpdates must sort, not trust wire order.
	a.reply = func(string) (int, string) {
		return 200, `{"ok":true,"result":[
			{"update_id":12,"message":{"message_id":2,"date":1700000000,"from":{"id":1},"chat":{"id":1,"type":"private"},"text":"b"}},
			{"update_id":10,"message":{"message_id":1,"date":1700000000,"from":{"id":1},"chat":{"id":1,"type":"private"},"text":"a"}},
			{"update_id":11,"message":{"message_id":3,"date":1700000000,"from":{"id":1},"chat":{"id":1,"type":"private"},"text":"c"}}
		]}`
	}
	got, err := a.client(io.Discard).GetUpdates(context.Background(), testToken, GetUpdatesRequest{TimeoutSeconds: 1})
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	for i, want := range []int64{10, 11, 12} {
		if got[i].UpdateID != want {
			t.Fatalf("out-of-order result: got[%d].UpdateID = %d, want %d (full: %+v)", i, got[i].UpdateID, want, got)
		}
	}
	// Raw must be the EXACT bytes for that element (feeds the store's raw
	// column) — round-tripping it must reproduce the same update_id, and it
	// must not be some other element's bytes.
	var raw struct {
		UpdateID int64 `json:"update_id"`
	}
	if err := json.Unmarshal(got[0].Raw, &raw); err != nil {
		t.Fatalf("Raw[0] not valid JSON: %v", err)
	}
	if raw.UpdateID != 10 {
		t.Fatalf("Raw[0] update_id = %d, want 10 (Raw=%s)", raw.UpdateID, got[0].Raw)
	}
	if got[0].Update == nil || got[0].Update.Message.Text != "a" {
		t.Fatalf("Update[0] = %+v, want text \"a\"", got[0].Update)
	}
}

func TestGetUpdatesEmptyBatch(t *testing.T) {
	a := newAPIServer(t)
	a.reply = func(string) (int, string) { return 200, `{"ok":true,"result":[]}` }
	got, err := a.client(io.Discard).GetUpdates(context.Background(), testToken, GetUpdatesRequest{TimeoutSeconds: 1})
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

func TestGetUpdatesMalformedElementFailsWholeBatch(t *testing.T) {
	a := newAPIServer(t)
	a.reply = func(string) (int, string) {
		// The second element is not an object at all.
		return 200, `{"ok":true,"result":[
			{"update_id":1,"message":{"message_id":1,"date":1700000000,"from":{"id":1},"chat":{"id":1,"type":"private"},"text":"ok"}},
			"not an update"
		]}`
	}
	got, err := a.client(io.Discard).GetUpdates(context.Background(), testToken, GetUpdatesRequest{TimeoutSeconds: 1})
	if err == nil {
		t.Fatalf("expected an error from a malformed element, got a %d-element batch", len(got))
	}
	if got != nil {
		t.Fatalf("a failed batch must return a nil slice, got %+v", got)
	}
}

func TestGetUpdatesSendsOffsetOnlyWhenNonZero(t *testing.T) {
	a := newAPIServer(t)
	a.reply = func(string) (int, string) { return 200, `{"ok":true,"result":[]}` }
	c := a.client(io.Discard)

	if _, err := c.GetUpdates(context.Background(), testToken, GetUpdatesRequest{Offset: 0, TimeoutSeconds: 1}); err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	// bodyFor returns the FIRST recorded call for a method; this test makes
	// two getUpdates calls, so it reads a.bodies by index directly rather
	// than through that single-call-per-method helper.
	if len(a.bodies) != 1 {
		t.Fatalf("recorded %d calls, want 1", len(a.bodies))
	}
	if _, present := a.bodies[0]["offset"]; present {
		t.Fatalf("offset was sent for a zero Offset: %v", a.bodies[0])
	}

	if _, err := c.GetUpdates(context.Background(), testToken, GetUpdatesRequest{Offset: 42, TimeoutSeconds: 1}); err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if len(a.bodies) != 2 {
		t.Fatalf("recorded %d calls, want 2", len(a.bodies))
	}
	if got := a.bodies[1]["offset"]; got != float64(42) {
		t.Fatalf("offset = %v, want 42", got)
	}
}

func TestGetUpdatesDefaultsLimitAndAllowedUpdates(t *testing.T) {
	a := newAPIServer(t)
	a.reply = func(string) (int, string) { return 200, `{"ok":true,"result":[]}` }
	if _, err := a.client(io.Discard).GetUpdates(context.Background(), testToken, GetUpdatesRequest{TimeoutSeconds: 1}); err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	body := a.bodyFor("getUpdates")
	if got := body["limit"]; got != float64(100) {
		t.Fatalf("limit = %v, want default 100", got)
	}
	updates, _ := body["allowed_updates"].([]any)
	if len(updates) != 1 || updates[0] != "message" {
		t.Fatalf("allowed_updates = %v, want [message]", body["allowed_updates"])
	}
}

// TestGetUpdatesDeadlineHasHeadroomOverTimeout proves the underlying HTTP
// call's deadline is TimeoutSeconds+10s of headroom, not just TimeoutSeconds
// — our own client must never race Telegram's own server-side long-poll
// wait. A client-side context deadline is never transmitted to the server
// (nothing in HTTP carries it), so this cannot be observed via r.Context()
// server-side; instead it proves the OBSERVABLE behavior: at
// TimeoutSeconds=0, a response delayed well past zero — but comfortably
// inside the fixed 10s headroom — must still succeed.
func TestGetUpdatesDeadlineHasHeadroomOverTimeout(t *testing.T) {
	a := newAPIServer(t)
	a.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"result":[]}`)
	})
	start := time.Now()
	if _, err := a.client(io.Discard).GetUpdates(context.Background(), testToken, GetUpdatesRequest{TimeoutSeconds: 0}); err != nil {
		t.Fatalf("GetUpdates: %v (elapsed %s) — the +10s headroom should easily cover a 300ms server delay even at TimeoutSeconds=0", err, time.Since(start))
	}
}

func TestGetUpdatesRetryAfterPresentAndAbsent(t *testing.T) {
	a := newAPIServer(t)
	a.reply = func(string) (int, string) {
		return 429, `{"ok":false,"error_code":429,"description":"Too Many Requests: retry later",
			"parameters":{"retry_after":7}}`
	}
	_, err := a.client(io.Discard).GetUpdates(context.Background(), testToken, GetUpdatesRequest{TimeoutSeconds: 1})
	d, ok := RetryAfter(err)
	if !ok || d != 7*time.Second {
		t.Fatalf("RetryAfter = %s, %v; want 7s, true (err=%v)", d, ok, err)
	}

	a.reply = func(string) (int, string) {
		return 409, `{"ok":false,"error_code":409,"description":"Conflict: terminated by other getUpdates request"}`
	}
	_, err = a.client(io.Discard).GetUpdates(context.Background(), testToken, GetUpdatesRequest{TimeoutSeconds: 1})
	if _, ok := RetryAfter(err); ok {
		t.Fatalf("RetryAfter reported present when Telegram sent no parameters block (err=%v)", err)
	}
	if code := APIErrorCode(err); code != 409 {
		t.Fatalf("APIErrorCode = %d, want 409", code)
	}
}

func TestGetUpdatesUnauthorizedMapping(t *testing.T) {
	a := newAPIServer(t)
	a.reply = func(string) (int, string) {
		return 401, `{"ok":false,"error_code":401,"description":"Unauthorized"}`
	}
	_, err := a.client(io.Discard).GetUpdates(context.Background(), testToken, GetUpdatesRequest{TimeoutSeconds: 1})
	if !IsUnauthorized(err) {
		t.Fatalf("IsUnauthorized(%v) = false, want true", err)
	}
}

// TestGetUpdatesTokenNeverAppearsInErrors covers the long-poll (lp) transport
// specifically — it is a DIFFERENT *http.Client than every other method uses
// (see HTTP.lp's doc comment), so this is not fully covered by
// TestTokenNeverAppearsInLogsOrErrors (client_test.go), which never calls
// GetUpdates.
func TestGetUpdatesTokenNeverAppearsInErrors(t *testing.T) {
	dead := NewHTTP("http://127.0.0.1:1", slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := dead.GetUpdates(context.Background(), testToken, GetUpdatesRequest{TimeoutSeconds: 1})
	if err == nil {
		t.Fatal("expected a transport error against a dead port")
	}
	if strings.Contains(err.Error(), testToken) {
		t.Fatalf("the bot token leaked into a GetUpdates transport error: %q", err)
	}
}

// TestGetUpdatesClassifiesCancellationBeforeRedaction proves a canceled
// parent context surfaces as a plain context.Canceled (errors.Is-able), not
// buried inside the redaction wrapper — the tgpoller loop depends on being
// able to check this directly rather than pattern-matching error text.
func TestGetUpdatesClassifiesCancellationBeforeRedaction(t *testing.T) {
	a := newAPIServer(t)
	block := make(chan struct{})
	a.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // never respond — the client must cancel first
	})
	t.Cleanup(func() { close(block) })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := a.client(io.Discard).GetUpdates(ctx, testToken, GetUpdatesRequest{TimeoutSeconds: 50})
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want errors.Is(err, context.Canceled)", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("GetUpdates did not return promptly after ctx cancellation")
	}
}

// --- Fake --------------------------------------------------------------

func TestFakeGetUpdatesPopsPendingBatchesFIFO(t *testing.T) {
	f := NewFake(1, "bot")
	b1 := []RawUpdate{RawFrom(Update{UpdateID: 1})}
	b2 := []RawUpdate{RawFrom(Update{UpdateID: 2}), RawFrom(Update{UpdateID: 3})}
	f.PendingUpdates = [][]RawUpdate{b1, b2}

	got1, err := f.GetUpdates(context.Background(), "t", GetUpdatesRequest{Offset: 5, Limit: 10, TimeoutSeconds: 3})
	if err != nil || len(got1) != 1 || got1[0].UpdateID != 1 {
		t.Fatalf("first batch = %+v, err=%v", got1, err)
	}
	got2, err := f.GetUpdates(context.Background(), "t", GetUpdatesRequest{})
	if err != nil || len(got2) != 2 {
		t.Fatalf("second batch = %+v, err=%v", got2, err)
	}
	got3, err := f.GetUpdates(context.Background(), "t", GetUpdatesRequest{})
	if err != nil || len(got3) != 0 {
		t.Fatalf("third (exhausted) batch = %+v, err=%v, want empty/nil", got3, err)
	}

	calls := f.CallsFor("getUpdates")
	if len(calls) != 3 {
		t.Fatalf("recorded %d getUpdates calls, want 3", len(calls))
	}
	if calls[0].Offset != 5 || calls[0].LimitN != 10 || calls[0].TimeoutSeconds != 3 {
		t.Fatalf("Call did not record getUpdates params: %+v", calls[0])
	}
}

func TestFakeGetUpdatesBadTokenIsUnauthorized(t *testing.T) {
	f := NewFake(1, "bot")
	f.BadTokens["bad"] = true
	_, err := f.GetUpdates(context.Background(), "bad", GetUpdatesRequest{})
	if !IsUnauthorized(err) {
		t.Fatalf("IsUnauthorized(%v) = false, want true", err)
	}
}

func TestFakeGetUpdatesFnTakesPriorityAndCanBlockUntilCtxDone(t *testing.T) {
	f := NewFake(1, "bot")
	f.GetUpdatesFn = func(ctx context.Context, token string, req GetUpdatesRequest) ([]RawUpdate, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := f.GetUpdates(ctx, "t", GetUpdatesRequest{})
		done <- err
	}()
	select {
	case <-done:
		t.Fatal("GetUpdatesFn returned before ctx was canceled")
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GetUpdatesFn did not unblock after cancellation")
	}
}

func TestRawFromRoundTrips(t *testing.T) {
	u := Update{UpdateID: 99, Message: &Message{MessageID: 1, Text: "hi"}}
	ru := RawFrom(u)
	if ru.UpdateID != 99 || ru.Update.Message.Text != "hi" {
		t.Fatalf("RawFrom = %+v", ru)
	}
	var back Update
	if err := json.Unmarshal(ru.Raw, &back); err != nil {
		t.Fatalf("Raw did not round-trip: %v", err)
	}
	if back.UpdateID != 99 {
		t.Fatalf("round-tripped update_id = %d, want 99", back.UpdateID)
	}
}
