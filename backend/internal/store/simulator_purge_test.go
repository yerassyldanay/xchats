package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// seedSimulatorConversation creates the org's simulator account and one
// inbound simulator message — the same shape a real "send" through
// SimulatorPanel/handleSimulatorMessage produces (chat + contact + CRM
// customer + identity, all tied to the simulator account).
func seedSimulatorConversation(t *testing.T, st *store.Store, orgID uuid.UUID, ref string) store.InboundResult {
	t.Helper()
	ctx := context.Background()
	acct, err := st.GetOrCreateSimulatorAccount(ctx, orgID)
	if err != nil {
		t.Fatalf("simulator account: %v", err)
	}
	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: acct.ID, PhoneJID: ref, RemoteJID: ref,
		PhoneNumber: ref, PushName: "Sim " + ref, Direction: "in", SenderKind: "contact",
		ExternalMessageID: uuid.NewString(), MessageKind: "conversation", Body: "test message",
		Preview: "test message", Source: "simulator", MessageTS: time.Now(),
	})
	if err != nil {
		t.Fatalf("simulator inbound: %v", err)
	}
	return res
}

func TestPurgeSimulatorData_RemovesConversationAndCustomer(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "purge-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}

	res := seedSimulatorConversation(t, st, org.ID, "sim-1")

	purged, err := st.PurgeSimulatorData(ctx, org.ID)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged.ConversationsDeleted != 1 {
		t.Errorf("ConversationsDeleted = %d, want 1", purged.ConversationsDeleted)
	}
	if purged.CustomersDeleted != 1 {
		t.Errorf("CustomersDeleted = %d, want 1", purged.CustomersDeleted)
	}

	if _, err := st.ChatByID(ctx, res.ChatID); err == nil {
		t.Error("chat still exists after purge")
	}
	if _, err := st.CustomerByID(ctx, org.ID, res.CustomerID); err == nil {
		t.Error("customer still exists after purge")
	}
}

func TestPurgeSimulatorData_NeverUsedSimulator_NoOp(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "clean-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}

	purged, err := st.PurgeSimulatorData(ctx, org.ID)
	if err != nil {
		t.Fatalf("purge on an org with no simulator account: %v", err)
	}
	if purged.ConversationsDeleted != 0 || purged.CustomersDeleted != 0 {
		t.Errorf("expected a no-op, got %+v", purged)
	}
}

func TestPurgeSimulatorData_ScopedToOwnOrganization(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	org1, err := st.SeedOrganization(ctx, "purge-org-1")
	if err != nil {
		t.Fatalf("seed org1: %v", err)
	}
	org2, err := st.SeedOrganization(ctx, "purge-org-2")
	if err != nil {
		t.Fatalf("seed org2: %v", err)
	}

	seedSimulatorConversation(t, st, org1.ID, "sim-a")
	other := seedSimulatorConversation(t, st, org2.ID, "sim-b")

	if _, err := st.PurgeSimulatorData(ctx, org1.ID); err != nil {
		t.Fatalf("purge org1: %v", err)
	}

	// org2's own simulator conversation/customer must be untouched.
	if _, err := st.ChatByID(ctx, other.ChatID); err != nil {
		t.Errorf("org2's chat was deleted by org1's purge: %v", err)
	}
	if _, err := st.CustomerByID(ctx, org2.ID, other.CustomerID); err != nil {
		t.Errorf("org2's customer was deleted by org1's purge: %v", err)
	}
}

func TestPurgeSimulatorData_KeepsCustomerMergedWithRealIdentity(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "merge-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}

	// A real WhatsApp conversation/customer.
	const ownerJID = "77011234567@s.whatsapp.net"
	waAccount := config.AccountID(ownerJID)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: waAccount, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName: "Real WA", ExternalAccountRef: ownerJID,
		ExternalHandle: "77011234567", ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed wa account: %v", err)
	}
	realRes, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: waAccount, PhoneJID: "77011234567@s.whatsapp.net",
		RemoteJID: "77011234567@s.whatsapp.net", PhoneNumber: "77011234567",
		PushName: "Real Customer", Direction: "in", SenderKind: "contact",
		ExternalMessageID: uuid.NewString(), MessageKind: "conversation", Body: "hi",
		Preview: "hi", Source: "live_webhook", MessageTS: time.Now(),
	})
	if err != nil {
		t.Fatalf("real inbound: %v", err)
	}

	simRes := seedSimulatorConversation(t, st, org.ID, "sim-merge")

	// A deliberate operator merge: simulator customer folded into the real one.
	merged, err := st.MergeCustomers(ctx, org.ID, realRes.CustomerID, simRes.CustomerID, uuid.NullUUID{})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	purged, err := st.PurgeSimulatorData(ctx, org.ID)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged.CustomersDeleted != 0 {
		t.Errorf("CustomersDeleted = %d, want 0 (the surviving customer has a real identity too)", purged.CustomersDeleted)
	}
	if purged.ConversationsDeleted != 1 {
		t.Errorf("ConversationsDeleted = %d, want 1 (the simulator chat itself is still test data)", purged.ConversationsDeleted)
	}

	// The merged customer survives with its real data intact.
	survivor, err := st.CustomerByID(ctx, org.ID, merged.ID)
	if err != nil {
		t.Fatalf("surviving customer should still exist: %v", err)
	}
	if survivor.DisplayName != "Real Customer" {
		t.Errorf("survivor.DisplayName = %q, want the real customer's own name preserved", survivor.DisplayName)
	}
	// The real conversation is unaffected by a purge scoped to simulator data.
	if _, err := st.ChatByID(ctx, realRes.ChatID); err != nil {
		t.Errorf("real chat should be untouched by the purge: %v", err)
	}
}
