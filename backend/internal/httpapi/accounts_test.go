package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

const (
	sales2Owner    = "77022222222@s.whatsapp.net"
	sales2Customer = "77033333333@s.whatsapp.net"
)

// seedConnectedAccount adds a second live WhatsApp account directly (bypassing
// the QR pairing flow, which internal/whatsmeow — not httpapi — now owns end
// to end; see TestPairWhatsAppAccount for the pairing HTTP contract itself).
func (h *harness) seedConnectedAccount(t *testing.T, ownerJID, displayName string) string {
	t.Helper()
	id := config.AccountID(ownerJID)
	if _, err := h.store.UpsertConnectedAccount(context.Background(), store.Account{
		ID:                 id,
		OrganizationID:     uuid.NullUUID{UUID: h.orgID, Valid: true},
		DisplayName:        displayName,
		ExternalAccountRef: config.CanonicalJID(ownerJID),
		ExternalHandle:     config.PhoneFromJID(ownerJID),
		ConnectionState:    "connected",
	}); err != nil {
		t.Fatalf("seed connected account: %v", err)
	}
	return id.String()
}

// TestPairWhatsAppAccount drives the QR pairing HTTP contract: start ->
// poll (qr_required, a rendered code) -> the phone finishes pairing
// out-of-band (CompletePair stands in for that, since a real QR scan cannot
// happen in a test) -> poll again (connected, with the account id).
func TestPairWhatsAppAccount(t *testing.T) {
	h := newHarness(t)

	resp, env := h.postJSON("/xchats/api/v1/wa-accounts/pair", map[string]any{})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("pair status = %d", resp.StatusCode)
	}
	var started struct {
		SessionID string `json:"session_id"`
		Status    string `json:"status"`
	}
	mustPayload(t, env, &started)
	if started.SessionID == "" || started.Status != "qr_required" {
		t.Fatalf("pair response = %+v", started)
	}

	var poll struct {
		Status   string `json:"status"`
		QRCode   string `json:"qr_code"`
		QRBase64 string `json:"qr_base64"`
	}
	h.get("/xchats/api/v1/wa-accounts/pair/"+started.SessionID, &poll)
	if poll.Status != "qr_required" || poll.QRCode == "" || poll.QRBase64 == "" {
		t.Fatalf("pair poll before scan = %+v", poll)
	}

	sales2ID := h.seedConnectedAccount(t, sales2Owner, "Sales 2")
	h.fake.CompletePair(sales2ID)

	var polled struct {
		Status    string `json:"status"`
		AccountID string `json:"account_id"`
	}
	h.get("/xchats/api/v1/wa-accounts/pair/"+started.SessionID, &polled)
	if polled.Status != "connected" || polled.AccountID != sales2ID {
		t.Fatalf("pair poll after scan = %+v, want connected/%s", polled, sales2ID)
	}

	// The newly paired account shows up in both the WhatsApp-only and the
	// channel-neutral listings.
	accts := h.listAccounts()
	if len(accts) != 2 {
		t.Fatalf("want 2 accounts, got %d", len(accts))
	}
	s2 := findAccountByHandle(accts, "77022222222")
	if s2 == nil || s2["connection_status"] != "connected" {
		t.Fatalf("sales2 not connected: %v", s2)
	}
}

// TestPairStatusUnknownSession asserts polling a session id nobody started
// (or one that has already expired out of the registry) is a 404, not a
// zero-value 200 that would read as "still pending forever".
func TestPairStatusUnknownSession(t *testing.T) {
	h := newHarness(t)
	resp, env := h.getRaw("/xchats/api/v1/wa-accounts/pair/" + uuid.NewString())
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if errcodeOf(env) != "NOT_FOUND" {
		t.Fatalf("errcode = %q, want NOT_FOUND", errcodeOf(env))
	}
}

// TestLogoutWhatsAppAccount disconnects an account without hiding it: the
// row stays listed (status logged_out) so the same number can be re-paired.
func TestLogoutWhatsAppAccount(t *testing.T) {
	h := newHarness(t)

	resp, _ := h.postJSON("/xchats/api/v1/wa-accounts/"+h.accountID.String()+"/logout", map[string]any{})
	if resp.StatusCode != 200 {
		t.Fatalf("logout status = %d", resp.StatusCode)
	}
	if len(h.fake.LoggedOut) != 1 || h.fake.LoggedOut[0] != h.accountID.String() {
		t.Fatalf("manager.Logout not called for the right account: %v", h.fake.LoggedOut)
	}

	var status struct {
		ConnectionState string `json:"connection_state"`
	}
	h.get("/xchats/api/v1/wa-accounts/"+h.accountID.String()+"/status", &status)
	if status.ConnectionState != "logged_out" {
		t.Fatalf("connection_state = %q, want logged_out", status.ConnectionState)
	}

	accts := h.listAccounts()
	if len(accts) != 1 {
		t.Fatalf("logged-out account should still be listed, got %d accounts", len(accts))
	}
}

// TestDeleteWhatsAppAccount is a "clean": best-effort logout + soft-delete —
// the row (and its chats) drop from the org's view, and re-adding the same
// number later revives the SAME row (uuidv5(owner_jid)) with history intact.
func TestDeleteWhatsAppAccount(t *testing.T) {
	h := newHarness(t)
	sales2ID := h.seedConnectedAccount(t, sales2Owner, "Sales 2")
	h.injectFor(sales2ID, sales2Customer, "S2MSG1", "привет от клиента", false)

	if got := len(h.listChatsFor(sales2ID)); got != 1 {
		t.Fatalf("setup: want 1 chat for sales2, got %d", got)
	}

	st, _ := h.del("/xchats/api/v1/whatsapp-accounts/" + sales2ID)
	if st != 200 {
		t.Fatalf("delete account status = %d", st)
	}
	if len(h.fake.LoggedOut) != 1 || h.fake.LoggedOut[0] != sales2ID {
		t.Fatalf("delete did not call manager.Logout for sales2: %v", h.fake.LoggedOut)
	}
	if len(h.listAccounts()) != 1 {
		t.Fatal("soft-deleted account still listed")
	}
	if got := len(h.listChatsFor(sales2ID)); got != 0 {
		t.Fatalf("soft-deleted account's chats still in the inbox: %d", got)
	}

	// Revive: re-seeding the same owner_jid lands on the SAME row id, history intact.
	revivedID := h.seedConnectedAccount(t, sales2Owner, "Sales 2 again")
	if revivedID != sales2ID {
		t.Fatalf("revive landed on a different id: %s vs %s", revivedID, sales2ID)
	}
	if got := len(h.listChatsFor(sales2ID)); got != 1 {
		t.Fatalf("revived account lost its history: %d chats", got)
	}
}

// TestComposeRequiresAccountWhenMany asserts the "from number" guard: with more
// than one connected account, composing without an account_id is a VALIDATION_ERROR.
func TestComposeRequiresAccountWhenMany(t *testing.T) {
	h := newHarness(t)
	sales2ID := h.seedConnectedAccount(t, sales2Owner, "Sales 2")

	resp, env := h.postJSON("/xchats/api/v1/chats",
		map[string]any{"phone": "77044444444", "text": "hi"})
	if resp.StatusCode != http.StatusBadRequest || errcodeOf(env) != "VALIDATION_ERROR" {
		t.Fatalf("ambiguous compose status=%d errcode=%q, want 400 VALIDATION_ERROR", resp.StatusCode, errcodeOf(env))
	}
	// With an explicit account_id it proceeds.
	resp, _ = h.postJSON("/xchats/api/v1/chats",
		map[string]any{"phone": "77044444444", "text": "hi", "account_id": sales2ID})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("explicit-account compose status = %d, want 201", resp.StatusCode)
	}
}

// --- helpers --------------------------------------------------------------

func (h *harness) listAccounts() []map[string]any {
	var out struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/whatsapp-accounts", &out)
	return out.Items
}

func (h *harness) listChatsFor(accountID string) []map[string]any {
	var out struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/chats?account_id="+accountID, &out)
	return out.Items
}

func (h *harness) del(path string) (int, map[string]json.RawMessage) {
	req, _ := http.NewRequest(http.MethodDelete, h.srv.URL+path, nil)
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("DELETE %s: %v", path, err)
	}
	var env map[string]json.RawMessage
	_ = json.NewDecoder(resp.Body).Decode(&env)
	resp.Body.Close()
	return resp.StatusCode, env
}

// TestListAccounts_AlwaysIncludesSimulator covers CAM-16: the simulator
// account must be available to the campaign wizard's account picker (which
// reads this same GET /accounts the Channels page does) from the very
// first list call, not only once some simulator traffic happens to have
// created its row already (the old behavior — GetOrCreateSimulatorAccount
// was previously only ever called lazily, from POST /simulator/messages).
func TestListAccounts_AlwaysIncludesSimulator(t *testing.T) {
	h := newHarness(t)

	var page struct {
		Items []struct {
			Channel         string `json:"channel"`
			ConnectionState string `json:"connection_state"`
		} `json:"items"`
	}
	h.get("/xchats/api/v1/accounts", &page)

	var sim *struct {
		Channel         string `json:"channel"`
		ConnectionState string `json:"connection_state"`
	}
	for i := range page.Items {
		if page.Items[i].Channel == "simulator" {
			sim = &page.Items[i]
			break
		}
	}
	if sim == nil {
		t.Fatalf("items = %+v, want a simulator account present on the very first list", page.Items)
	}
	if sim.ConnectionState != "connected" {
		t.Errorf("simulator connection_state = %q, want connected", sim.ConnectionState)
	}

	// Idempotent: listing twice must not create a second simulator row.
	var page2 struct {
		Items []struct {
			Channel string `json:"channel"`
		} `json:"items"`
	}
	h.get("/xchats/api/v1/accounts", &page2)
	simCount := 0
	for _, it := range page2.Items {
		if it.Channel == "simulator" {
			simCount++
		}
	}
	if simCount != 1 {
		t.Errorf("simulator accounts after a second list = %d, want exactly 1", simCount)
	}
}

// getRaw is like h.get but returns the raw response + envelope instead of
// decoding into a caller-supplied shape, for asserting on non-2xx status.
func (h *harness) getRaw(path string) (*http.Response, map[string]json.RawMessage) {
	resp, err := h.client.Get(h.srv.URL + path)
	if err != nil {
		h.t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	var env map[string]json.RawMessage
	_ = json.NewDecoder(resp.Body).Decode(&env)
	return resp, env
}

func findAccountByHandle(items []map[string]any, handle string) map[string]any {
	for _, it := range items {
		if it["phone_number"] == handle {
			return it
		}
	}
	return nil
}

func errcodeOf(env map[string]json.RawMessage) string {
	var s string
	_ = json.Unmarshal(env["errcode"], &s)
	return s
}
