package chatkb_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/chatkb"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// The retrieval service is the seam everything else depends on, so its one
// non-obvious property is worth pinning against the real kbstore: SearchReal
// sees only the live tables, while SearchDraft sees the EFFECTIVE draft —
// live rows with pending edits applied — so an untouched product still has a
// draft price rather than vanishing from the draft state.
func TestStoreServiceSeparatesLiveFromEffectiveDraft(t *testing.T) {
	kb, st, _ := dbtest.NewKB(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "acme")
	if err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	actor := uuid.Nil
	inStock := true

	live := kbstore.ProductInput{Ref: "vitamin-d", Name: "Vitamin D", Price: "12 000 KZT", InStock: &inStock, SalesStatus: "active"}
	if err := kb.PutLiveProduct(ctx, org.ID, actor, live); err != nil {
		t.Fatalf("seed live product: %v", err)
	}
	untouched := kbstore.ProductInput{Ref: "omega-3", Name: "Omega 3", Price: "8 000 KZT", InStock: &inStock, SalesStatus: "active"}
	if err := kb.PutLiveProduct(ctx, org.ID, actor, untouched); err != nil {
		t.Fatalf("seed untouched product: %v", err)
	}
	// Stage a price cut in the draft lane only.
	staged := live
	staged.Price = "10 800 KZT"
	if err := kb.UpsertProduct(ctx, org.ID, actor, staged); err != nil {
		t.Fatalf("stage draft product: %v", err)
	}

	svc := chatkb.NewStoreService(kb)
	real, err := svc.SearchReal(ctx, org.ID, "vitamin")
	if err != nil {
		t.Fatalf("SearchReal: %v", err)
	}
	draft, err := svc.SearchDraft(ctx, org.ID, "vitamin")
	if err != nil {
		t.Fatalf("SearchDraft: %v", err)
	}

	if real.Source != chatkb.SourceReal || draft.Source != chatkb.SourceDraft {
		t.Fatalf("sources = %s/%s, want %s/%s", real.Source, draft.Source, chatkb.SourceReal, chatkb.SourceDraft)
	}
	for _, rec := range real.Records {
		if rec.Source != chatkb.SourceReal {
			t.Errorf("real record %q carries source %s", rec.Key, rec.Source)
		}
	}
	for _, rec := range draft.Records {
		if rec.Source != chatkb.SourceDraft {
			t.Errorf("draft record %q carries source %s", rec.Key, rec.Source)
		}
	}

	if got := priceOf(t, real, "vitamin-d"); got != "12 000 KZT" {
		t.Errorf("real vitamin-d price = %q, want the live value", got)
	}
	if got := priceOf(t, draft, "vitamin-d"); got != "10 800 KZT" {
		t.Errorf("draft vitamin-d price = %q, want the staged value", got)
	}
	// The untouched product must exist in BOTH states — "what is the draft
	// price of Omega 3?" has an answer even though nobody edited it.
	if got := priceOf(t, draft, "omega-3"); got != "8 000 KZT" {
		t.Errorf("draft omega-3 price = %q, want the live value showing through", got)
	}

	diffs := chatkb.Result{Real: real, Draft: draft}.Differences()
	if len(diffs) != 1 {
		t.Fatalf("differences = %+v, want exactly the staged price change", diffs)
	}
	if diffs[0].Key != "vitamin-d" || len(diffs[0].Fields) != 1 || diffs[0].Fields[0].Key != "price" {
		t.Errorf("difference = %+v, want vitamin-d's price alone", diffs[0])
	}
}

func priceOf(t *testing.T, snap chatkb.Snapshot, key string) string {
	t.Helper()
	for _, rec := range snap.Records {
		if rec.Key != key {
			continue
		}
		for _, f := range rec.Fields {
			if f.Key == "price" {
				return f.Value
			}
		}
		t.Fatalf("record %q has no price field: %+v", key, rec.Fields)
	}
	t.Fatalf("record %q not found in the %s snapshot", key, snap.Source)
	return ""
}
