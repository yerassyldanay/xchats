package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/evolution"
)

const (
	sales2Instance = "sales2"
	sales2Owner    = "77022222222@s.whatsapp.net" // evolution.Fake.NewOwnerJID
	sales2Customer = "77033333333@s.whatsapp.net"
)

// TestAccountsManager drives the Build 1 manager loop end-to-end against the fake
// Evolution: add → QR → connected, the multi-account inbox, reconnect, clean
// (soft-delete), and revive-on-re-add with history intact.
func TestAccountsManager(t *testing.T) {
	h := newHarness(t)

	// 1. Add a second account: an instance is created on Evolution, no row yet.
	resp, _ := h.postJSON("/xchats/api/v1/whatsapp-accounts",
		map[string]any{"display_name": "Sales 2", "instance_name": sales2Instance})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add account status = %d", resp.StatusCode)
	}
	if got := join(h.fake.Created); got != sales2Instance {
		t.Fatalf("CreateInstance recorded %q, want %q", got, sales2Instance)
	}

	// 2. Poll the QR → the fake reports "open" → the row is written/connected,
	//    its id derived from the learned owner_jid.
	var qr struct {
		Status      string `json:"status"`
		WaAccountID string `json:"wa_account_id"`
	}
	h.get("/xchats/api/v1/whatsapp-accounts/qr?instance="+sales2Instance, &qr)
	if qr.Status != "connected" {
		t.Fatalf("qr poll status = %q, want connected", qr.Status)
	}
	sales2ID := config.AccountID(sales2Owner).String()
	if qr.WaAccountID != sales2ID {
		t.Fatalf("connected account id = %q, want uuidv5(owner) %q", qr.WaAccountID, sales2ID)
	}

	// 3. The accounts list now has both numbers, both connected (live status).
	accts := h.listAccounts()
	if len(accts) != 2 {
		t.Fatalf("want 2 accounts, got %d", len(accts))
	}
	s2 := findAccount(accts, sales2Instance)
	if s2 == nil || s2["connection_status"] != "connected" || s2["display_name"] != "Sales 2" {
		t.Fatalf("sales2 not connected/named: %v", s2)
	}

	// 4. Multi-account inbox: an inbound to sales2 shows up org-wide and is
	//    filterable by account.
	h.webhook(craftInbound(sales2Owner, sales2Customer, "S2MSG1", "привет от клиента"))
	all := h.listChats()
	if len(all) != 1 || all[0]["wa_account_id"] != sales2ID {
		t.Fatalf("inbound to sales2 not in org inbox: %v", all)
	}
	if got := len(h.listChatsFor(sales2ID)); got != 1 {
		t.Fatalf("filter by sales2 = %d chats, want 1", got)
	}
	if got := len(h.listChatsFor(h.accountID.String())); got != 0 {
		t.Fatalf("filter by xpayment = %d chats, want 0", got)
	}

	// 5. Reconnect a broken session → a fresh QR is returned (same account row).
	h.fake.ConnState = "close"
	var rc struct {
		Status string `json:"status"`
		QRCode string `json:"qr_code"`
	}
	resp, renv := h.postJSON("/xchats/api/v1/whatsapp-accounts/"+sales2ID+"/reconnect", map[string]any{})
	if resp.StatusCode != 200 {
		t.Fatalf("reconnect status = %d", resp.StatusCode)
	}
	mustPayload(t, renv, &rc)
	if rc.Status != "qr_required" || rc.QRCode == "" {
		t.Fatalf("reconnect did not return a QR: %+v", rc)
	}
	h.fake.ConnState = "open"

	// 6. Clean: the instance is deleted on Evolution and the row soft-deleted —
	//    it drops from the accounts list and its chats leave the inbox.
	st, _ := h.del("/xchats/api/v1/whatsapp-accounts/" + sales2ID)
	if st != 200 {
		t.Fatalf("delete account status = %d", st)
	}
	if !contains(h.fake.Deleted, sales2Instance) {
		t.Fatalf("DeleteInstance not called for %q (got %v)", sales2Instance, h.fake.Deleted)
	}
	if len(h.listAccounts()) != 1 {
		t.Fatalf("soft-deleted account still listed")
	}
	if len(h.listChats()) != 0 {
		t.Fatalf("soft-deleted account's chats still in the inbox")
	}

	// 7. Revive: re-adding the same number lands on the SAME row (uuidv5(owner)),
	//    so its history reappears intact.
	h.postJSON("/xchats/api/v1/whatsapp-accounts",
		map[string]any{"display_name": "Sales 2 again", "instance_name": sales2Instance})
	h.get("/xchats/api/v1/whatsapp-accounts/qr?instance="+sales2Instance, &qr)
	if qr.Status != "connected" || qr.WaAccountID != sales2ID {
		t.Fatalf("revive did not reconnect the same id: %+v", qr)
	}
	if got := len(h.listChatsFor(sales2ID)); got != 1 {
		t.Fatalf("revived account lost its history: %d chats", got)
	}
}

// TestComposeRequiresAccountWhenMany asserts the "from number" guard: with more
// than one connected account, composing without wa_account_id is a VALIDATION_ERROR.
func TestComposeRequiresAccountWhenMany(t *testing.T) {
	h := newHarness(t)
	h.postJSON("/xchats/api/v1/whatsapp-accounts",
		map[string]any{"display_name": "Sales 2", "instance_name": sales2Instance})
	var qr struct {
		WaAccountID string `json:"wa_account_id"`
	}
	h.get("/xchats/api/v1/whatsapp-accounts/qr?instance="+sales2Instance, &qr)

	resp, env := h.postJSON("/xchats/api/v1/chats",
		map[string]any{"phone": "77044444444", "text": "hi"})
	if resp.StatusCode != http.StatusBadRequest || errcodeOf(env) != "VALIDATION_ERROR" {
		t.Fatalf("ambiguous compose status=%d errcode=%q, want 400 VALIDATION_ERROR", resp.StatusCode, errcodeOf(env))
	}
	// With an explicit wa_account_id it proceeds (the fake says the number exists).
	resp, _ = h.postJSON("/xchats/api/v1/chats",
		map[string]any{"phone": "77044444444", "text": "hi", "wa_account_id": qr.WaAccountID})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("explicit-account compose status = %d, want 201", resp.StatusCode)
	}
}

// TestInstancesMaintenance covers managed/stale flags and the managed-delete guard.
func TestInstancesMaintenance(t *testing.T) {
	h := newHarness(t)
	// A stale stray instance Evolution still knows about (old + not connected).
	h.fake.Instances = append(h.fake.Instances, instanceWithAge("strayold", "close", 30))

	var out struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/whatsapp-instances", &out)
	xpay := findByName(out.Items, "xpayment")
	stray := findByName(out.Items, "strayold")
	if xpay == nil || xpay["managed"] != true {
		t.Fatalf("seeded xpayment should be managed: %v", xpay)
	}
	if stray == nil || stray["managed"] != false || stray["stale"] != true {
		t.Fatalf("strayold should be unmanaged + stale: %v", stray)
	}

	// Refuse to delete a managed instance (delete the account instead).
	st, env := h.del("/xchats/api/v1/whatsapp-instances/xpayment")
	if st != http.StatusConflict || errcodeOf(env) != "CONFLICT" {
		t.Fatalf("delete managed instance status=%d errcode=%q, want 409 CONFLICT", st, errcodeOf(env))
	}
	// A stray instance deletes fine.
	if st, _ := h.del("/xchats/api/v1/whatsapp-instances/strayold"); st != 200 {
		t.Fatalf("delete stray status = %d", st)
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
	h.get("/xchats/api/v1/chats?wa_account_id="+accountID, &out)
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

// craftInbound is an inbound (fromMe=false) text from a customer to the account
// whose owner_jid is `owner` (the envelope top-level sender).
func craftInbound(owner, customer, keyID, text string) []byte {
	ev := map[string]any{
		"event": "messages.upsert", "instance": "any", "sender": owner,
		"data": map[string]any{
			"key":              map[string]any{"remoteJid": customer, "remoteJidAlt": customer, "fromMe": false, "id": keyID},
			"pushName":         "Клиент",
			"message":          map[string]any{"conversation": text},
			"messageType":      "conversation",
			"messageTimestamp": 1781460000,
		},
	}
	b, _ := json.Marshal(ev)
	return b
}

func instanceWithAge(name, status string, daysOld int) evolution.Instance {
	return evolution.Instance{
		Name: name, ConnectionStatus: status,
		CreatedAt: time.Now().UTC().Add(-time.Duration(daysOld) * 24 * time.Hour),
	}
}

func findAccount(items []map[string]any, instance string) map[string]any {
	for _, it := range items {
		if it["instance_name"] == instance {
			return it
		}
	}
	return nil
}

func findByName(items []map[string]any, name string) map[string]any {
	for _, it := range items {
		if it["name"] == name {
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

func join(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	return ss[0]
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
