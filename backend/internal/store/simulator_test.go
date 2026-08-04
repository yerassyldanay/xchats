package store_test

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
)

func TestGetOrCreateSimulatorAccount_IdempotentPerOrg(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()

	org, err := st.SeedOrganization(ctx, "sim-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}

	a1, err := st.GetOrCreateSimulatorAccount(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSimulatorAccount (first): %v", err)
	}
	if a1.Channel != "simulator" {
		t.Fatalf("channel = %q, want simulator", a1.Channel)
	}
	if !a1.OrganizationID.Valid || a1.OrganizationID.UUID != org.ID {
		t.Fatalf("organization mismatch: %+v", a1.OrganizationID)
	}

	a2, err := st.GetOrCreateSimulatorAccount(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetOrCreateSimulatorAccount (second): %v", err)
	}
	if a2.ID != a1.ID {
		t.Fatalf("want the same simulator account on a second call: %s vs %s", a1.ID, a2.ID)
	}
}

func TestGetOrCreateSimulatorAccount_DistinctPerOrg(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()

	org1, err := st.SeedOrganization(ctx, "sim-org-1")
	if err != nil {
		t.Fatalf("seed org1: %v", err)
	}
	org2, err := st.SeedOrganization(ctx, "sim-org-2")
	if err != nil {
		t.Fatalf("seed org2: %v", err)
	}

	a1, err := st.GetOrCreateSimulatorAccount(ctx, org1.ID)
	if err != nil {
		t.Fatalf("org1: %v", err)
	}
	a2, err := st.GetOrCreateSimulatorAccount(ctx, org2.ID)
	if err != nil {
		t.Fatalf("org2: %v", err)
	}
	if a1.ID == a2.ID {
		t.Fatal("distinct organizations must get distinct simulator accounts")
	}
}
