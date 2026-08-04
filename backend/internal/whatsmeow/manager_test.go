package whatsmeow

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"go.mau.fi/whatsmeow/proto/waAdv"
	wmstore "go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/whatsapp"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestManager returns a Manager over a fresh xchats store (dbtest) and a
// fresh, temp-dir whatsmeow device database — everything Start/Pair/ingest
// need, with no live network involved.
func newTestManager(t *testing.T) (*Manager, *store.Store) {
	t.Helper()
	st := dbtest.New(t)
	q := queue.NewInMem(64, 1, testLogger())
	t.Cleanup(q.Close)
	mgr, err := NewManager(context.Background(), ManagerConfig{
		DeviceDBPath: filepath.Join(t.TempDir(), "wa-device.db"),
		Store:        st,
		Queue:        q,
		Hub:          realtime.NewHub(),
		Log:          testLogger(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(mgr.Close)
	return mgr, st
}

// seedAccount seeds a connected account under a fresh organization and
// returns (orgID, accountID).
func seedAccount(t *testing.T, st *store.Store, ownerJID string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "test-org-"+uuid.NewString())
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	accountID := config.AccountID(ownerJID)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: accountID, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		ExternalAccountRef: config.CanonicalJID(ownerJID), ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	return org.ID, accountID
}

func assertDeliveryState(t *testing.T, st *store.Store, msgID uuid.UUID, want string) {
	t.Helper()
	msg, err := st.MessageByID(context.Background(), msgID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if msg.DeliveryState != want {
		t.Fatalf("delivery_state = %q, want %q", msg.DeliveryState, want)
	}
}

// TestIngestMessage_EchoCollapse is the production echo-collapse contract:
// the fromMe echo of a message we sent (same external id we stamped on the
// outbound row) must merge onto that row — no duplicate insert, no unread bump.
func TestIngestMessage_EchoCollapse(t *testing.T) {
	mgr, st := newTestManager(t)
	ctx := context.Background()
	_, accountID := seedAccount(t, st, "77011111111@s.whatsapp.net")

	chatID, _, err := st.FindOrCreateChat(ctx, accountID, "77000000000@s.whatsapp.net", "77000000000")
	if err != nil {
		t.Fatalf("FindOrCreateChat: %v", err)
	}
	msgID, err := st.InsertOutbound(ctx, "whatsapp", chatID, accountID, "user", uuid.NullUUID{}, "conversation", "привет", "привет")
	if err != nil {
		t.Fatalf("InsertOutbound: %v", err)
	}
	if err := st.StampOutboundSent(ctx, "whatsapp", msgID, "FAKEID1"); err != nil {
		t.Fatalf("StampOutboundSent: %v", err)
	}

	res, err := mgr.ingestMessage(ctx, whatsapp.MessageReceived{
		AccountID: accountID.String(), ExternalID: "FAKEID1", ChatJID: "77000000000@s.whatsapp.net",
		PhoneJID: "77000000000@s.whatsapp.net", FromMe: true, Kind: "conversation", Text: "привет",
	})
	if err != nil {
		t.Fatalf("ingestMessage: %v", err)
	}
	if res.MessageInserted {
		t.Fatal("echo should collapse onto the existing row, not insert a new one")
	}
	if res.MessageID != msgID {
		t.Fatalf("collapsed onto %s, want the original outbound row %s", res.MessageID, msgID)
	}

	chat, err := st.ChatByID(ctx, chatID)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}
	if chat.UnreadCount != 0 {
		t.Fatalf("unread_count = %d, want 0 (an echo must never bump unread)", chat.UnreadCount)
	}
}

// TestIngestMessage_NewInboundCreatesChatAndBumpsUnread is EchoCollapse's
// counterpart: a genuinely new inbound message DOES create a chat and bump
// the unread counter.
func TestIngestMessage_NewInboundCreatesChatAndBumpsUnread(t *testing.T) {
	mgr, st := newTestManager(t)
	ctx := context.Background()
	_, accountID := seedAccount(t, st, "77011111111@s.whatsapp.net")

	res, err := mgr.ingestMessage(ctx, whatsapp.MessageReceived{
		AccountID: accountID.String(), ExternalID: "IN1", ChatJID: "77000000000@s.whatsapp.net",
		PhoneJID: "77000000000@s.whatsapp.net", Kind: "conversation", Text: "hi",
	})
	if err != nil {
		t.Fatalf("ingestMessage: %v", err)
	}
	if !res.MessageInserted || !res.ChatCreated {
		t.Fatalf("want a new message + a new chat, got %+v", res)
	}
	chat, err := st.ChatByID(ctx, res.ChatID)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}
	if chat.UnreadCount != 1 {
		t.Fatalf("unread_count = %d, want 1", chat.UnreadCount)
	}
}

// TestApplyReceiptUpdate_Monotonic is the delivery ladder's core contract:
// forward transitions apply, backward ones are silently ignored.
func TestApplyReceiptUpdate_Monotonic(t *testing.T) {
	mgr, st := newTestManager(t)
	ctx := context.Background()
	_, accountID := seedAccount(t, st, "77011111111@s.whatsapp.net")
	chatID, _, err := st.FindOrCreateChat(ctx, accountID, "77000000000@s.whatsapp.net", "77000000000")
	if err != nil {
		t.Fatalf("FindOrCreateChat: %v", err)
	}
	msgID, err := st.InsertOutbound(ctx, "whatsapp", chatID, accountID, "user", uuid.NullUUID{}, "conversation", "hi", "hi")
	if err != nil {
		t.Fatalf("InsertOutbound: %v", err)
	}
	if err := st.StampOutboundSent(ctx, "whatsapp", msgID, "SENT1"); err != nil {
		t.Fatalf("StampOutboundSent: %v", err)
	}

	mgr.applyReceiptUpdate(ctx, whatsapp.ReceiptUpdated{AccountID: accountID.String(), ExternalIDs: []string{"SENT1"}, DeliveryState: "delivered"})
	assertDeliveryState(t, st, msgID, "delivered")

	mgr.applyReceiptUpdate(ctx, whatsapp.ReceiptUpdated{AccountID: accountID.String(), ExternalIDs: []string{"SENT1"}, DeliveryState: "read"})
	assertDeliveryState(t, st, msgID, "read")

	// Backwards (a stale delivered ack arriving after read) must be ignored.
	mgr.applyReceiptUpdate(ctx, whatsapp.ReceiptUpdated{AccountID: accountID.String(), ExternalIDs: []string{"SENT1"}, DeliveryState: "delivered"})
	assertDeliveryState(t, st, msgID, "read")
}

// TestInjectDebugEvent_MessageAndReceipt exercises the whatsapp.Manager port
// end to end through the same ingest/receipt core the tests above hit
// directly, proving the debug endpoint's handler wiring is correct too.
func TestInjectDebugEvent_MessageAndReceipt(t *testing.T) {
	mgr, st := newTestManager(t)
	ctx := context.Background()
	_, accountID := seedAccount(t, st, "77011111111@s.whatsapp.net")

	res, err := mgr.InjectDebugEvent(ctx, whatsapp.DebugEvent{
		EventType: "message", AccountID: accountID.String(), SenderJID: "77022223333", Text: "привет",
	})
	if err != nil {
		t.Fatalf("InjectDebugEvent(message): %v", err)
	}
	if res.MessageID == "" || res.ChatID == "" {
		t.Fatalf("want message/chat ids, got %+v", res)
	}

	msgUUID, err := uuid.Parse(res.MessageID)
	if err != nil {
		t.Fatalf("message id: %v", err)
	}
	msg, err := st.MessageByID(ctx, msgUUID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	extID := msg.ExternalMessageID

	if _, err := mgr.InjectDebugEvent(ctx, whatsapp.DebugEvent{
		EventType: "receipt", AccountID: accountID.String(), ExternalID: extID, ReceiptType: "read",
	}); err != nil {
		t.Fatalf("InjectDebugEvent(receipt): %v", err)
	}
	assertDeliveryState(t, st, msgUUID, "read")
}

func TestInjectDebugEvent_UnknownType(t *testing.T) {
	mgr, st := newTestManager(t)
	_, accountID := seedAccount(t, st, "77011111111@s.whatsapp.net")
	if _, err := mgr.InjectDebugEvent(context.Background(), whatsapp.DebugEvent{
		EventType: "not-a-real-type", AccountID: accountID.String(),
	}); err == nil {
		t.Fatal("want an error for an unknown event_type")
	}
}

// TestInjectDebugEvent_ConnectionState drives the "connected"/"disconnected"/
// "logged_out" debug event types through the same setConnectionState a real
// Connected/Disconnected/LoggedOut whatsmeow event uses, so a test can cover
// the connection-status side of the ingest path without a live socket.
func TestInjectDebugEvent_ConnectionState(t *testing.T) {
	mgr, st := newTestManager(t)
	ctx := context.Background()
	_, accountID := seedAccount(t, st, "77011111111@s.whatsapp.net")

	for _, state := range []string{"disconnected", "logged_out", "connected"} {
		if _, err := mgr.InjectDebugEvent(ctx, whatsapp.DebugEvent{
			EventType: state, AccountID: accountID.String(),
		}); err != nil {
			t.Fatalf("InjectDebugEvent(%s): %v", state, err)
		}
		got, err := mgr.Status(ctx, accountID.String())
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if got.State != state {
			t.Fatalf("after injecting %q, Status = %q", state, got.State)
		}
	}
}

// TestPair_Lifecycle drives the QR pairing flow with a fake client: start ->
// qr_required -> a PairSuccess event (standing in for the phone finishing
// the scan) -> connected, with the account persisted and its client
// registered for outbound sends.
func TestPair_Lifecycle(t *testing.T) {
	mgr, st := newTestManager(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "pair-test-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}

	var created []*fakeWAClient
	mgr.newClient = func(device *wmstore.Device, log waLog.Logger) waClient {
		f := newFakeWAClient()
		created = append(created, f)
		return f
	}

	sessionID, err := mgr.Pair(ctx, org.ID.String())
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}
	update, ok := mgr.PairStatus(sessionID)
	if !ok || update.Status != "qr_required" {
		t.Fatalf("pair status before scan = %+v, ok=%v", update, ok)
	}
	if len(created) != 1 {
		t.Fatalf("want 1 client constructed for pairing, got %d", len(created))
	}
	if !created[0].IsConnected() {
		t.Fatal("Pair did not Connect the new client")
	}

	ownerJID := types.JID{User: "77099999999", Server: types.DefaultUserServer, Device: 12}
	created[0].fire(&events.PairSuccess{ID: ownerJID})

	update, ok = mgr.PairStatus(sessionID)
	if !ok || update.Status != "connected" || update.AccountID == "" {
		t.Fatalf("pair status after scan = %+v, ok=%v", update, ok)
	}
	acctID, err := uuid.Parse(update.AccountID)
	if err != nil {
		t.Fatalf("account id: %v", err)
	}
	acct, err := st.AccountByID(ctx, acctID)
	if err != nil {
		t.Fatalf("AccountByID: %v", err)
	}
	if acct.ConnectionState != "connected" || !acct.OrganizationID.Valid || acct.OrganizationID.UUID != org.ID {
		t.Fatalf("paired account not set up correctly: %+v", acct)
	}
	creds, err := st.ListWaCredentials(ctx)
	if err != nil {
		t.Fatalf("ListWaCredentials: %v", err)
	}
	if len(creds) != 1 || creds[0].DeviceJID != ownerJID.String() {
		t.Fatalf("credentials not saved as the raw AD-form jid: %+v", creds)
	}
	if _, ok := mgr.clientFor(update.AccountID); !ok {
		t.Fatal("paired client not registered for outbound sends")
	}
}

func TestPair_ErrorEvent(t *testing.T) {
	mgr, st := newTestManager(t)
	org, err := st.SeedOrganization(context.Background(), "pair-error-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	var created []*fakeWAClient
	mgr.newClient = func(device *wmstore.Device, log waLog.Logger) waClient {
		f := newFakeWAClient()
		created = append(created, f)
		return f
	}
	sessionID, err := mgr.Pair(context.Background(), org.ID.String())
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}
	created[0].fire(&events.PairError{Error: errBoom})
	update, ok := mgr.PairStatus(sessionID)
	if !ok || update.Status != "error" || update.Message == "" {
		t.Fatalf("pair status after error = %+v, ok=%v", update, ok)
	}
}

// TestReconnect_UsesSavedCredentials is the boot-time restore path: a
// previously paired device (saved in wa_credentials, and present in
// whatsmeow's own device store) reconnects without any QR interaction.
func TestReconnect_UsesSavedCredentials(t *testing.T) {
	mgr, st := newTestManager(t)
	ctx := context.Background()
	_, accountID := seedAccount(t, st, "77088888888@s.whatsapp.net")

	var created []*fakeWAClient
	mgr.newClient = func(device *wmstore.Device, log waLog.Logger) waClient {
		f := newFakeWAClient()
		created = append(created, f)
		return f
	}

	jid := types.JID{User: "77088888888", Server: types.DefaultUserServer, Device: 3}
	device := mgr.container.NewDevice()
	device.ID = &jid
	// A real pairing fills Account in via the noise handshake before Save is
	// ever called; PutDevice dereferences it unconditionally and the schema
	// has NOT NULL / fixed-length CHECK constraints on these fields, so a
	// device persisted directly in a test needs correctly-sized dummy values.
	device.Account = &waAdv.ADVSignedDeviceIdentity{
		Details:             []byte{},
		AccountSignatureKey: make([]byte, 32),
		AccountSignature:    make([]byte, 64),
		DeviceSignature:     make([]byte, 64),
	}
	if err := device.Save(ctx); err != nil {
		t.Fatalf("save device: %v", err)
	}
	if err := st.SaveWaCredentials(ctx, accountID, jid.String()); err != nil {
		t.Fatalf("SaveWaCredentials: %v", err)
	}

	mgr.Start(ctx)

	if len(created) != 1 {
		t.Fatalf("want 1 client reconnected, got %d", len(created))
	}
	if !created[0].IsConnected() {
		t.Fatal("Start did not Connect the reconnected client")
	}
	if _, ok := mgr.clientFor(accountID.String()); !ok {
		t.Fatal("reconnected client not registered")
	}
}

// TestReconnect_MissingDeviceIsNotFatal: Start must not abort the whole
// batch when one saved account's device record is gone or unparsable.
func TestReconnect_MissingDeviceIsNotFatal(t *testing.T) {
	mgr, st := newTestManager(t)
	ctx := context.Background()
	_, accountID := seedAccount(t, st, "77077777777@s.whatsapp.net")
	// No matching device was ever Saved — GetDevice will return nil.
	if err := st.SaveWaCredentials(ctx, accountID, "77077777777:1@s.whatsapp.net"); err != nil {
		t.Fatalf("SaveWaCredentials: %v", err)
	}

	mgr.newClient = func(device *wmstore.Device, log waLog.Logger) waClient {
		t.Fatal("newClient should not be called for a missing device")
		return nil
	}

	mgr.Start(ctx) // must not panic
}

// TestLogout disconnects the live client, deletes the credential mapping,
// and marks the account logged out.
func TestLogout(t *testing.T) {
	mgr, st := newTestManager(t)
	ctx := context.Background()
	_, accountID := seedAccount(t, st, "77066666666@s.whatsapp.net")
	if err := st.SaveWaCredentials(ctx, accountID, "77066666666:1@s.whatsapp.net"); err != nil {
		t.Fatalf("SaveWaCredentials: %v", err)
	}
	fake := newFakeWAClient()
	mgr.registerClient(accountID.String(), fake)
	if err := fake.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if err := mgr.Logout(ctx, accountID.String()); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if fake.IsConnected() {
		t.Fatal("Logout did not disconnect the live client")
	}
	if _, ok := mgr.clientFor(accountID.String()); ok {
		t.Fatal("Logout did not remove the client from the registry")
	}
	creds, err := st.ListWaCredentials(ctx)
	if err != nil {
		t.Fatalf("ListWaCredentials: %v", err)
	}
	if len(creds) != 0 {
		t.Fatalf("credentials not deleted: %+v", creds)
	}
	status, err := mgr.Status(ctx, accountID.String())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.State != "logged_out" {
		t.Fatalf("state = %q, want logged_out", status.State)
	}
}

type simpleErr string

func (e simpleErr) Error() string { return string(e) }

const errBoom = simpleErr("boom")
