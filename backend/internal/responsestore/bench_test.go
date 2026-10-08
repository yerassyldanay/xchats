package responsestore_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
)

// BenchmarkKnowledgeBaseRepo_Load covers KnowledgeBaseRepo.Load — the
// response engine's hot path (see kb.go's own doc comment): every customer
// reply the AI drafts loads the org's approved knowledge base through this
// exact call, so its latency directly gates reply latency.
func BenchmarkKnowledgeBaseRepo_Load(b *testing.B) {
	now := time.Now()

	ctx := context.Background()
	repo, st, db := dbtest.NewKBRepo(b)
	org, err := st.SeedOrganization(ctx, "bench-org")
	if err != nil {
		b.Fatalf("seed org: %v", err)
	}

	mustExec(b, db, `INSERT INTO ai_assistants (organization_id, persona, mission, guardrails, language_policy, reply_max_words, id, created_at, updated_at)
		VALUES ($1, 'Персона', 'Миссия', 'Правила', 'Языковая политика', 100, $2, $3, $3)`, org.ID, uuid.New(), now)
	for i, slug := range []string{"delivery", "payment", "warranty"} {
		mustExec(b, db, `INSERT INTO ai_topics (organization_id, slug, title, body_md, id, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $6)`,
			org.ID, slug, slug, fmt.Sprintf("Содержимое темы номер %d.", i), uuid.New(), now)
	}
	for i := 0; i < 20; i++ {
		mustExec(b, db, `INSERT INTO ai_products (organization_id, ref, name, price, description, category, availability_status, id, created_at, updated_at)
			VALUES ($1, $2, $3, '1 000 ₸', 'Описание', '', 'in_stock', $4, $5, $5)`,
			org.ID, fmt.Sprintf("product-%d", i), fmt.Sprintf("Товар %d", i), uuid.New(), now)
	}
	mustExec(b, db, `INSERT INTO ai_contacts (organization_id, phone, working_hours, id, created_at, updated_at) VALUES ($1, '+7 700 000 00 00', '9:00-18:00', $2, $3, $3)`, org.ID, uuid.New(), now)
	mustExec(b, db, `INSERT INTO ai_policies (organization_id, delivery_cost, delivery_in_days, outside_zones_note, id, created_at, updated_at) VALUES ($1, '1 000 ₸', '1-2', '', $2, $3, $3)`, org.ID, uuid.New(), now)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := repo.Load(ctx, org.ID.String()); err != nil {
			b.Fatalf("Load: %v", err)
		}
	}
}
