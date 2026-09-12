package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeServer is a minimal stand-in for the real xchats backend, implementing
// exactly the routes this tool calls, with the same envelope shape
// ({"payload":...,"errcode":"OK"}) and the same fresh-install
// password-rotation flow.
type fakeServer struct {
	mu                 sync.Mutex
	mustChangePassword bool
	loginCalls         int64
	chatSeq            int64

	// failRoute, when non-empty, makes every call to that route respond
	// with failStatus instead of succeeding — for error-handling tests.
	failRoute  string
	failStatus int

	seedChats []chatItem
}

func envelopeOK(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"payload": payload, "errcode": "OK", "message": ""})
}

func (f *fakeServer) maybeFail(w http.ResponseWriter, route string) bool {
	f.mu.Lock()
	fail := f.failRoute == route
	status := f.failStatus
	f.mu.Unlock()
	if fail {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"errcode":"INTERNAL","message":"injected failure"}`))
		return true
	}
	return false
}

func newFakeServer() *fakeServer {
	return &fakeServer{
		mustChangePassword: true,
		seedChats: []chatItem{
			{ID: "chat-wa-1", Channel: "whatsapp"},
			{ID: "chat-tg-1", Channel: "telegram"},
		},
	}
}

func (f *fakeServer) mux() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /xchats/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&f.loginCalls, 1)
		f.mu.Lock()
		mustChange := f.mustChangePassword
		f.mu.Unlock()
		envelopeOK(w, map[string]any{"user": map[string]any{"must_change_password": mustChange}})
	})

	mux.HandleFunc("POST /xchats/api/v1/auth/password", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.mustChangePassword = false
		f.mu.Unlock()
		envelopeOK(w, map[string]any{"user": map[string]any{"must_change_password": false}})
	})

	mux.HandleFunc("GET /xchats/api/v1/chats", func(w http.ResponseWriter, r *http.Request) {
		if f.maybeFail(w, routeListChats) {
			return
		}
		f.mu.Lock()
		items := append([]chatItem(nil), f.seedChats...)
		f.mu.Unlock()
		envelopeOK(w, map[string]any{"items": items, "page": 1, "page_size": 50, "total": len(items)})
	})

	mux.HandleFunc("GET /xchats/api/v1/chats/{id}/messages", func(w http.ResponseWriter, r *http.Request) {
		if f.maybeFail(w, routeListMessages) {
			return
		}
		envelopeOK(w, map[string]any{"items": []any{}, "next_before": nil})
	})

	mux.HandleFunc("GET /xchats/api/v1/kb", func(w http.ResponseWriter, r *http.Request) {
		if f.maybeFail(w, routeKB) {
			return
		}
		envelopeOK(w, map[string]any{"config": map[string]any{}, "topics": []any{}})
	})

	mux.HandleFunc("POST /xchats/api/v1/debug/wa-event", func(w http.ResponseWriter, r *http.Request) {
		if f.maybeFail(w, routeDebugWAEvent) {
			return
		}
		id := atomic.AddInt64(&f.chatSeq, 1)
		envelopeOK(w, map[string]any{
			"ok": true, "message_id": fmt.Sprintf("msg-%d", id), "chat_id": fmt.Sprintf("chat-debug-%d", id),
		})
	})

	mux.HandleFunc("POST /xchats/api/v1/simulator/messages", func(w http.ResponseWriter, r *http.Request) {
		if f.maybeFail(w, routeSimulator) {
			return
		}
		id := atomic.AddInt64(&f.chatSeq, 1)
		envelopeOK(w, map[string]any{
			"conversation_id": fmt.Sprintf("chat-sim-%d", id), "message_id": fmt.Sprintf("msg-%d", id),
			"draft": map[string]any{"id": "draft-1", "text": "ok", "reply_language": "ru", "escalate": false},
		})
	})

	mux.HandleFunc("POST /xchats/api/v1/chats/{id}/messages", func(w http.ResponseWriter, r *http.Request) {
		if f.maybeFail(w, routeSendMessage) {
			return
		}
		envelopeOK(w, map[string]any{"items": []any{}})
	})

	return mux
}

func newTestRunner(baseURL string, seed int64) (*runner, error) {
	client, err := newAPIClient(baseURL)
	if err != nil {
		return nil, err
	}
	return &runner{
		client:    client,
		chats:     newReservoir(seed, reservoirCap),
		byChannel: newChannelReservoirs(seed),
		contacts:  newContactPool(20),
	}, nil
}

func TestAuthentication_BootstrapWithRotation(t *testing.T) {
	fs := newFakeServer()
	srv := httptest.NewServer(fs.mux())
	defer srv.Close()

	client, err := newAPIClient(srv.URL)
	if err != nil {
		t.Fatalf("newAPIClient: %v", err)
	}
	if err := client.EnsureAuthenticated(context.Background(), "admin@xchat.kz", "xchat-admin-change-me", "new-pw"); err != nil {
		t.Fatalf("EnsureAuthenticated: %v", err)
	}
	fs.mu.Lock()
	changed := !fs.mustChangePassword
	fs.mu.Unlock()
	if !changed {
		t.Fatal("expected the password-rotation call to have fired")
	}

	// A second EnsureAuthenticated (fresh client, same server — the account
	// no longer needs rotation) must NOT call /auth/password again.
	client2, _ := newAPIClient(srv.URL)
	before := atomic.LoadInt64(&fs.loginCalls)
	if err := client2.EnsureAuthenticated(context.Background(), "admin@xchat.kz", "new-pw", "unused"); err != nil {
		t.Fatalf("second EnsureAuthenticated: %v", err)
	}
	if atomic.LoadInt64(&fs.loginCalls) != before+1 {
		t.Fatalf("expected exactly one more login call, login count now %d", fs.loginCalls)
	}
}

func TestAuthentication_LoginFailurePropagates(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /xchats/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errcode":"UNAUTHORIZED","message":"no session"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client, _ := newAPIClient(srv.URL)
	if err := client.EnsureAuthenticated(context.Background(), "x", "y", "z"); err == nil {
		t.Fatal("expected an error from a rejected login")
	}
}

func TestPreflight_DiscoversChatsAndChannels(t *testing.T) {
	fs := newFakeServer()
	fs.mustChangePassword = false
	srv := httptest.NewServer(fs.mux())
	defer srv.Close()

	rnr, err := newTestRunner(srv.URL, 1)
	if err != nil {
		t.Fatalf("newTestRunner: %v", err)
	}
	if err := rnr.Preflight(context.Background(), rand.New(rand.NewSource(1))); err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if rnr.chats.Len() == 0 {
		t.Fatal("Preflight should have discovered at least one chat id")
	}
	channels := rnr.byChannel.Channels()
	if len(channels) == 0 {
		t.Fatal("Preflight should have discovered at least one channel")
	}
}

func TestPreflight_FailsFastWithNoChats(t *testing.T) {
	fs := newFakeServer()
	fs.mustChangePassword = false
	fs.seedChats = nil // an empty install — nothing to discover
	srv := httptest.NewServer(fs.mux())
	defer srv.Close()

	rnr, err := newTestRunner(srv.URL, 1)
	if err != nil {
		t.Fatalf("newTestRunner: %v", err)
	}
	err = rnr.Preflight(context.Background(), rand.New(rand.NewSource(1)))
	if err == nil {
		t.Fatal("expected Preflight to fail fast when GET /chats returns nothing")
	}
}

func TestRunLoad_ExercisesEveryWeightedRouteAndReportsPercentiles(t *testing.T) {
	fs := newFakeServer()
	fs.mustChangePassword = false
	srv := httptest.NewServer(fs.mux())
	defer srv.Close()

	rnr, err := newTestRunner(srv.URL, 7)
	if err != nil {
		t.Fatalf("newTestRunner: %v", err)
	}
	if err := rnr.Preflight(context.Background(), rand.New(rand.NewSource(7))); err != nil {
		t.Fatalf("Preflight: %v", err)
	}

	wr, err := newWeightedRoutes(defaultWeights)
	if err != nil {
		t.Fatalf("newWeightedRoutes: %v", err)
	}
	stats := newStatsCollector()
	runLoad(context.Background(), runConfig{
		Concurrency: 8, Warmup: 50 * time.Millisecond, Duration: 500 * time.Millisecond, Seed: 7,
	}, rnr, wr, stats)

	if got := stats.TotalErrors(); got != 0 {
		t.Fatalf("TotalErrors() = %d, want 0 against a fully healthy fake server", got)
	}
	names := stats.RouteNames()
	for route := range defaultWeights {
		found := false
		for _, n := range names {
			if n == route {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("route %q was never exercised over the run (names seen: %v)", route, names)
		}
	}

	count, _, _, _, p50, p95, p99 := stats.Overall().hist.Snapshot()
	if count == 0 {
		t.Fatal("overall histogram recorded zero observations")
	}
	if !(p50 <= p95 && p95 <= p99) {
		t.Errorf("percentiles not monotonic: p50=%v p95=%v p99=%v", p50, p95, p99)
	}
}

func TestRunLoad_RecordsErrorsAndStatusesOnFailingRoute(t *testing.T) {
	fs := newFakeServer()
	fs.mustChangePassword = false
	fs.failRoute = routeKB
	fs.failStatus = http.StatusInternalServerError
	srv := httptest.NewServer(fs.mux())
	defer srv.Close()

	rnr, err := newTestRunner(srv.URL, 3)
	if err != nil {
		t.Fatalf("newTestRunner: %v", err)
	}
	rnr.chats.Add("seed-chat") // avoid failing preflight on the broken route alone
	rnr.byChannel.Add("whatsapp", "seed-chat")

	wr, err := newWeightedRoutes(map[string]float64{routeKB: 100}) // force every iteration onto the failing route
	if err != nil {
		t.Fatalf("newWeightedRoutes: %v", err)
	}
	stats := newStatsCollector()
	runLoad(context.Background(), runConfig{
		Concurrency: 4, Warmup: 0, Duration: 200 * time.Millisecond, Seed: 3,
	}, rnr, wr, stats)

	if got := stats.TotalErrors(); got == 0 {
		t.Fatal("expected recorded errors from the injected failure")
	}
	kb := stats.Route(routeKB)
	if kb == nil {
		t.Fatal("expected kb route stats to exist")
	}
	kb.mu.Lock()
	status500 := kb.statuses[http.StatusInternalServerError]
	kb.mu.Unlock()
	if status500 == 0 {
		t.Error("expected at least one recorded 500 status")
	}
}

func TestRunLoad_StopsPromptlyOnCancellation(t *testing.T) {
	fs := newFakeServer()
	fs.mustChangePassword = false
	srv := httptest.NewServer(fs.mux())
	defer srv.Close()

	rnr, err := newTestRunner(srv.URL, 5)
	if err != nil {
		t.Fatalf("newTestRunner: %v", err)
	}
	rnr.chats.Add("seed-chat")
	rnr.byChannel.Add("whatsapp", "seed-chat")
	wr, _ := newWeightedRoutes(defaultWeights)
	stats := newStatsCollector()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	// A long configured duration that cancellation must cut short.
	runLoad(ctx, runConfig{Concurrency: 8, Warmup: 0, Duration: 30 * time.Second, Seed: 5}, rnr, wr, stats)
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("runLoad took %v to stop after cancellation, want it to return promptly", elapsed)
	}
}

func TestWeightedRoutes_DeterministicForAFixedSeed(t *testing.T) {
	wr, err := newWeightedRoutes(defaultWeights)
	if err != nil {
		t.Fatalf("newWeightedRoutes: %v", err)
	}
	rngA := rand.New(rand.NewSource(99))
	rngB := rand.New(rand.NewSource(99))
	for i := 0; i < 200; i++ {
		a, b := wr.Pick(rngA), wr.Pick(rngB)
		if a != b {
			t.Fatalf("pick %d diverged for the same seed: %q vs %q", i, a, b)
		}
	}
}

func TestWeightedRoutes_RejectsAllZeroWeights(t *testing.T) {
	if _, err := newWeightedRoutes(map[string]float64{"a": 0, "b": 0}); err == nil {
		t.Fatal("expected an error when every weight is zero")
	}
}

func TestWeightedRoutes_RejectsNegativeWeight(t *testing.T) {
	if _, err := newWeightedRoutes(map[string]float64{"a": -1}); err == nil {
		t.Fatal("expected an error for a negative weight")
	}
}
