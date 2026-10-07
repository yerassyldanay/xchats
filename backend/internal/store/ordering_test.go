package store_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// Pages are cut with LIMIT/OFFSET over a timestamp, timestamps have millisecond resolution, and
// PostgreSQL returns rows that tie in no particular order: a different order from one page to
// the next repeats some rows and skips others. So every paginated list ends its ORDER BY with the
// row id, and tied rows come back in id order on either engine.
func TestPaginatedListsBreakTimestampTiesByID(t *testing.T) {
	ctx := context.Background()
	st, db := dbtest.Open(t)
	orgID, userID, acctID := seedCampaignFixture(t, st, ctx)
	tie := time.UnixMilli(1791244800000).UTC()
	const n = 9

	sortedIDs := func(ids []uuid.UUID) []string {
		out := make([]string, len(ids))
		for i, id := range ids {
			out[i] = id.String()
		}
		slices.Sort(out)
		return out
	}

	t.Run("campaigns", func(t *testing.T) {
		var created []uuid.UUID
		for i := 0; i < n; i++ {
			created = append(created, mustCreateCampaign(t, st, ctx, orgID, acctID, userID, fmt.Sprintf("C%d", i), "Hi").ID)
		}
		if _, err := db.Exec(ctx, `UPDATE campaigns SET created_at = $1 WHERE organization_id = $2`, tie, orgID); err != nil {
			t.Fatal(err)
		}
		got, total, err := st.ListCampaignsForOrg(ctx, orgID, 50, 0)
		if err != nil || total != n {
			t.Fatalf("ListCampaignsForOrg: total %d, err %v", total, err)
		}
		var ids []string
		for _, c := range got {
			ids = append(ids, c.ID.String())
		}
		if want := sortedIDs(created); !slices.Equal(ids, want) {
			t.Fatalf("campaigns tied on created_at came back as\n %v\nwant id order\n %v", ids, want)
		}
	})

	t.Run("customers", func(t *testing.T) {
		var created []uuid.UUID
		for i := 0; i < n; i++ {
			name := fmt.Sprintf("Customer %d", i)
			c, err := st.CreateCustomer(ctx, orgID, store.CustomerPatch{DisplayName: &name}, uuid.NullUUID{})
			if err != nil {
				t.Fatalf("CreateCustomer: %v", err)
			}
			created = append(created, c.ID)
		}
		if _, err := db.Exec(ctx, `UPDATE crm_customers SET updated_at = $1 WHERE organization_id = $2`, tie, orgID); err != nil {
			t.Fatal(err)
		}
		got, total, err := st.ListCustomers(ctx, store.CustomerFilter{OrgID: orgID, Limit: 50})
		if err != nil || total != n {
			t.Fatalf("ListCustomers: total %d, err %v", total, err)
		}
		var ids []string
		for _, c := range got {
			ids = append(ids, c.ID.String())
		}
		if want := sortedIDs(created); !slices.Equal(ids, want) {
			t.Fatalf("customers tied on updated_at came back as\n %v\nwant id order\n %v", ids, want)
		}
	})
}
