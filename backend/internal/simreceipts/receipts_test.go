package simreceipts

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

type fakeHub struct {
	events []string
}

func (f *fakeHub) Broadcast(name string, _ any) { f.events = append(f.events, name) }

func seedSimulatorMessage(t *testing.T, st *store.Store, ctx context.Context, orgSuffix, destination string) (msgID uuid.UUID, externalID string, acctID uuid.UUID) {
	t.Helper()
	org, err := st.SeedOrganization(ctx, "receipts-org-"+orgSuffix)
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	acct, err := st.GetOrCreateSimulatorAccount(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSimulatorAccount: %v", err)
	}
	chatID, _, err := st.FindOrCreateChat(ctx, acct.ID, destination, destination)
	if err != nil {
		t.Fatalf("FindOrCreateChat: %v", err)
	}
	msgID, err = st.InsertCampaignOutbound(ctx, "simulator", chatID, acct.ID, "hi", "hi")
	if err != nil {
		t.Fatalf("InsertCampaignOutbound: %v", err)
	}
	externalID = "sim-" + msgID.String()
	if err := st.StampOutboundSent(ctx, "simulator", msgID, externalID); err != nil {
		t.Fatalf("StampOutboundSent: %v", err)
	}
	return msgID, externalID, acct.ID
}

func waitForDeliveryState(t *testing.T, st *store.Store, ctx context.Context, msgID uuid.UUID, want string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		msg, err := st.MessageByID(ctx, msgID)
		if err != nil {
			t.Fatalf("MessageByID: %v", err)
		}
		last = msg.DeliveryState
		if last == want {
			return last
		}
		time.Sleep(10 * time.Millisecond)
	}
	return last
}

// TestReceiptSimulator_AdvancesReadDestinationAllTheWayToRead proves the
// whole async sent -> delivered -> read progression for a destination whose
// fixed simulator.Outcome is OutcomeRead (digit 3-9, see that package's own
// doc comment) — the SAME AdvanceDeliveryState store method a real
// channel's own delivery-receipt webhook drives.
func TestReceiptSimulator_AdvancesReadDestinationAllTheWayToRead(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	msgID, _, _ := seedSimulatorMessage(t, st, ctx, "read-"+uuid.NewString(), "77011234563@s.whatsapp.net")

	hub := &fakeHub{}
	rs := NewReceiptSimulator(st, hub, Config{SweepEvery: 10 * time.Millisecond, DeliveredAfter: 20 * time.Millisecond, ReadAfter: 20 * time.Millisecond}, nil)
	rs.Start(ctx)
	defer rs.Stop()

	if got := waitForDeliveryState(t, st, ctx, msgID, "delivered", time.Second); got != "delivered" && got != "read" {
		t.Fatalf("delivery_state = %q, want at least delivered", got)
	}
	if got := waitForDeliveryState(t, st, ctx, msgID, "read", 2*time.Second); got != "read" {
		t.Fatalf("delivery_state = %q, want read", got)
	}
	if len(hub.events) == 0 {
		t.Error("want at least one message.updated broadcast as the state advanced")
	}
	for _, e := range hub.events {
		if e != "message.updated" {
			t.Errorf("broadcast event = %q, want message.updated", e)
		}
	}
}

// TestReceiptSimulator_DeliveredOnlyDestinationNeverReachesRead proves the
// OTHER fixed outcome: a destination ending in 1 or 2 reaches 'delivered'
// and stays there — never advanced to 'read' — mirroring a real recipient
// who receives a message but never opens it.
func TestReceiptSimulator_DeliveredOnlyDestinationNeverReachesRead(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	msgID, _, _ := seedSimulatorMessage(t, st, ctx, "delivonly-"+uuid.NewString(), "77011234561@s.whatsapp.net")

	hub := &fakeHub{}
	rs := NewReceiptSimulator(st, hub, Config{SweepEvery: 10 * time.Millisecond, DeliveredAfter: 20 * time.Millisecond, ReadAfter: 20 * time.Millisecond}, nil)
	rs.Start(ctx)
	defer rs.Stop()

	if got := waitForDeliveryState(t, st, ctx, msgID, "delivered", time.Second); got != "delivered" {
		t.Fatalf("delivery_state = %q, want delivered", got)
	}
	// Give the sweep several more chances to (wrongly) advance it further.
	time.Sleep(200 * time.Millisecond)
	msg, err := st.MessageByID(ctx, msgID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if msg.DeliveryState != "delivered" {
		t.Errorf("delivery_state = %q, want it to stay at delivered", msg.DeliveryState)
	}
}

// TestReceiptSimulator_IgnoresOtherChannels proves the sweep never touches a
// whatsapp-channel message sitting in the very same wa_messages table —
// SimulatorMessagesAwaitingReceipt's own channel='simulator' filter is what
// this ultimately relies on (see its own test in internal/store), asserted
// here end to end through the real sweep loop.
func TestReceiptSimulator_IgnoresOtherChannels(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	org, err := st.SeedOrganization(ctx, "receipts-other-channel-"+uuid.NewString())
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	waAcct, err := st.SeedAccount(ctx, store.Account{
		ID: uuid.New(), OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName: "Real WA", ExternalAccountRef: "7770000002@s.whatsapp.net", ExternalHandle: "77700000002",
		ConnectionState: "connected",
	})
	if err != nil {
		t.Fatalf("seed wa account: %v", err)
	}
	chatID, _, err := st.FindOrCreateChat(ctx, waAcct.ID, "77011234565@s.whatsapp.net", "77011234565")
	if err != nil {
		t.Fatalf("FindOrCreateChat: %v", err)
	}
	msgID, err := st.InsertCampaignOutbound(ctx, "whatsapp", chatID, waAcct.ID, "hi", "hi")
	if err != nil {
		t.Fatalf("InsertCampaignOutbound: %v", err)
	}
	if err := st.StampOutboundSent(ctx, "whatsapp", msgID, "wamid-"+msgID.String()); err != nil {
		t.Fatalf("StampOutboundSent: %v", err)
	}

	hub := &fakeHub{}
	rs := NewReceiptSimulator(st, hub, Config{SweepEvery: 10 * time.Millisecond, DeliveredAfter: 10 * time.Millisecond, ReadAfter: 10 * time.Millisecond}, nil)
	rs.Start(ctx)
	defer rs.Stop()
	time.Sleep(200 * time.Millisecond)

	msg, err := st.MessageByID(ctx, msgID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if msg.DeliveryState != "sent" {
		t.Errorf("delivery_state = %q, want unchanged at sent (not a simulator message)", msg.DeliveryState)
	}
	if len(hub.events) != 0 {
		t.Errorf("events = %v, want none for a non-simulator message", hub.events)
	}
}
