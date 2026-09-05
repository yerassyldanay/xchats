package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
)

func TestSeedDemoWorkspace_PopulatesScreenshotsAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	st, db := dbtest.Open(t)
	org, err := st.SeedOrganization(ctx, "xchats")
	if err != nil {
		t.Fatalf("SeedOrganization: %v", err)
	}
	adminID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	if err := st.SeedDemoCRM(ctx, org.ID, adminID); err != nil {
		t.Fatalf("SeedDemoCRM: %v", err)
	}
	if inserted, err := st.SeedDemoWorkspace(ctx, org.ID, adminID); err != nil || !inserted {
		t.Fatalf("first SeedDemoWorkspace: inserted=%v err=%v", inserted, err)
	}
	if inserted, err := st.SeedDemoWorkspace(ctx, org.ID, adminID); err != nil || !inserted {
		t.Fatalf("second SeedDemoWorkspace: inserted=%v err=%v", inserted, err)
	}

	assertCount := func(label, query string, want int) {
		t.Helper()
		var got int
		if err := db.QueryRow(ctx, query, org.ID).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", label, err)
		}
		if got != want {
			t.Errorf("%s count = %d, want %d", label, got, want)
		}
	}
	assertCount("accounts", `SELECT count(*) FROM inbox_accounts_v WHERE organization_id = $1`, 6)
	assertCount("inbox chats", `SELECT count(*) FROM inbox_chats_v WHERE organization_id = $1`, 5)
	assertCount("customers", `SELECT count(*) FROM crm_customers WHERE organization_id = $1`, 5)
	assertCount("assistant conversations", `SELECT count(*) FROM chat_conversations WHERE organization_id = $1`, 3)

	var drafts int
	if err := db.QueryRow(ctx, `
		SELECT count(*) FROM ai_drafts d
		JOIN inbox_chats_v c ON c.id = d.chat_id AND c.channel = d.channel
		WHERE c.organization_id = $1 AND d.draft_state = 'suggested'`, org.ID).Scan(&drafts); err != nil {
		t.Fatalf("count AI drafts: %v", err)
	}
	if drafts != 5 {
		t.Errorf("AI draft count = %d, want 5", drafts)
	}
}
