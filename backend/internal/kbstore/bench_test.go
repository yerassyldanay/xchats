package kbstore_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/brain"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// BenchmarkLoadLive covers the brain's own read path (kbstore.go: "the
// brain reasons from" ai_assistants/ai_topics and the typed fact tables) —
// every reply the response engine drafts loads the org's live KB through
// this exact call.
func BenchmarkLoadLive(b *testing.B) {
	ctx := context.Background()
	kb, st, _ := dbtest.NewKB(b)
	org, err := st.SeedOrganization(ctx, "bench-org")
	if err != nil {
		b.Fatalf("seed org: %v", err)
	}
	if err := kb.SeedLiveIfEmpty(ctx, org.ID, brain.SeedSnapshot()); err != nil {
		b.Fatalf("seed live kb: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := kb.LoadLive(ctx, org.ID); err != nil {
			b.Fatalf("LoadLive: %v", err)
		}
	}
}

// BenchmarkDraft covers the Playground's merged working view (live rows
// overlaid by any pending draft entries) — GET /playground/draft's read
// path, and every /kb/* live editor page load.
func BenchmarkDraft(b *testing.B) {
	ctx := context.Background()
	kb, st, _ := dbtest.NewKB(b)
	org, err := st.SeedOrganization(ctx, "bench-org")
	if err != nil {
		b.Fatalf("seed org: %v", err)
	}
	if err := kb.SeedLiveIfEmpty(ctx, org.ID, brain.SeedSnapshot()); err != nil {
		b.Fatalf("seed live kb: %v", err)
	}
	if err := kb.UpsertTopic(ctx, org.ID, uuid.Nil, kbstore.TopicInput{
		Slug: "bench_pending", Title: "Pending", BodyMD: "Незавершённая правка.",
	}); err != nil {
		b.Fatalf("stage a pending draft edit: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := kb.Draft(ctx, org.ID); err != nil {
			b.Fatalf("Draft: %v", err)
		}
	}
}
