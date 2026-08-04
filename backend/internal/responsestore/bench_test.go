package responsestore_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
)

// BenchmarkKnowledgeBaseRepo_Load covers KnowledgeBaseRepo.Load — the
// response engine's hot path (see kb.go's own doc comment): every customer
// reply the AI drafts loads the org's approved knowledge base through this
// exact call, so its latency directly gates reply latency.
func BenchmarkKnowledgeBaseRepo_Load(b *testing.B) {
	ctx := context.Background()
	repo, st, db := dbtest.NewKBRepo(b)
	org, err := st.SeedOrganization(ctx, "bench-org")
	if err != nil {
		b.Fatalf("seed org: %v", err)
	}

	mustExec(b, db, `INSERT INTO ai_assistants (organization_id, persona, mission, guardrails, language_policy, reply_max_words)
		VALUES ($1, 'Персона', 'Миссия', 'Правила', 'Языковая политика', 100)`, org.ID)
	for i, slug := range []string{"delivery", "payment", "warranty"} {
		mustExec(b, db, `INSERT INTO ai_topics (organization_id, slug, title, body_md) VALUES ($1, $2, $3, $4)`,
			org.ID, slug, slug, fmt.Sprintf("Содержимое темы номер %d.", i))
	}
	for i := 0; i < 20; i++ {
		mustExec(b, db, `INSERT INTO ai_products (organization_id, ref, name, price, description, category, in_stock)
			VALUES ($1, $2, $3, '1 000 ₸', 'Описание', '', true)`,
			org.ID, fmt.Sprintf("product-%d", i), fmt.Sprintf("Товар %d", i))
	}
	mustExec(b, db, `INSERT INTO ai_contacts (organization_id, phone, working_hours) VALUES ($1, '+7 700 000 00 00', '9:00-18:00')`, org.ID)
	mustExec(b, db, `INSERT INTO ai_policies (organization_id, delivery_cost, delivery_in_days, outside_zones_note) VALUES ($1, '1 000 ₸', '1-2', '')`, org.ID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := repo.Load(ctx, org.ID.String()); err != nil {
			b.Fatalf("Load: %v", err)
		}
	}
}
