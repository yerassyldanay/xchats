package responsestore_test

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
)

func TestKnowledgeBaseRepo_NotConfigured(t *testing.T) {
	st, orgID, _ := newTestStore(t)
	repo := &responsestore.KnowledgeBaseRepo{Pool: st.Pool()}

	if _, err := repo.Load(context.Background(), orgID.String()); err != responsestore.ErrKBNotConfigured {
		t.Fatalf("Load() error = %v, want ErrKBNotConfigured", err)
	}
}

func TestKnowledgeBaseRepo_LoadsFullKB(t *testing.T) {
	st, orgID, _ := newTestStore(t)
	ctx := context.Background()
	pool := st.Pool()

	mustExec(t, pool, `INSERT INTO xchats.ai_assistants (organization_id, persona, mission, guardrails, language_policy, reply_max_words)
		VALUES ($1, 'Персона', 'Миссия', 'Правила', 'Языковая политика', 100)`, orgID)
	mustExec(t, pool, `INSERT INTO xchats.ai_topics (organization_id, slug, title, body_md)
		VALUES ($1, 'delivery', 'Доставка', 'Доставляем по городу.')`, orgID)
	mustExec(t, pool, `INSERT INTO xchats.ai_products (organization_id, ref, name, price, description, category, in_stock)
		VALUES ($1, 'widget', 'Виджет', '1 000 ₸', 'Описание', '', true)`, orgID)
	mustExec(t, pool, `INSERT INTO xchats.ai_tariffs (organization_id, ref, name, price, limit_text, fee, summary, pricing_type, advantages, disadvantages)
		VALUES ($1, 'basic', 'Базовый', '5 000 ₸', '', '', '', 'fixed', '', '')`, orgID)
	mustExec(t, pool, `INSERT INTO xchats.ai_contacts (organization_id, phone, working_hours)
		VALUES ($1, '+7 700 000 00 00', '9:00-18:00')`, orgID)
	mustExec(t, pool, `INSERT INTO xchats.ai_policies (organization_id, delivery_cost, delivery_in_days, outside_zones_note)
		VALUES ($1, '1 000 ₸', '1-2', '')`, orgID)

	repo := &responsestore.KnowledgeBaseRepo{Pool: pool}
	kb, err := repo.Load(ctx, orgID.String())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if kb.Assistant == nil || kb.Assistant.Persona != "Персона" {
		t.Fatalf("assistant = %+v", kb.Assistant)
	}
	if len(kb.Topics) != 1 || kb.Topics[0].Title != "Доставка" {
		t.Fatalf("topics = %+v", kb.Topics)
	}
	if len(kb.Products) != 1 || kb.Products[0].SalesStatus != "active" || !kb.Products[0].InStock {
		t.Fatalf("products = %+v", kb.Products)
	}
	if len(kb.Tariffs) != 1 || kb.Tariffs[0].Ref != "basic" {
		t.Fatalf("tariffs = %+v", kb.Tariffs)
	}
	if kb.Contacts == nil || kb.Contacts.Phone != "+7 700 000 00 00" {
		t.Fatalf("contacts = %+v", kb.Contacts)
	}
	if kb.Policies == nil || kb.Policies.DeliveryInDays != "1-2" {
		t.Fatalf("policies = %+v", kb.Policies)
	}
	if len(kb.Materials) != 0 {
		t.Fatalf("materials must stay empty, got %+v", kb.Materials)
	}

	if _, err := aiprompt.BuildCatalog(kb); err != nil {
		t.Fatalf("loaded KB must build a valid catalog: %v", err)
	}
}

func TestKnowledgeBaseRepo_LoadsDeliveryZones(t *testing.T) {
	st, orgID, _ := newTestStore(t)
	ctx := context.Background()
	pool := st.Pool()

	mustExec(t, pool, `INSERT INTO xchats.ai_assistants (organization_id, persona) VALUES ($1, 'p')`, orgID)
	mustExec(t, pool, `INSERT INTO xchats.ai_policies (organization_id, outside_zones_note) VALUES ($1, 'Вне зон не доставляем.')`, orgID)
	mustExec(t, pool, `INSERT INTO xchats.ai_delivery_zones (organization_id, ref, name, zone_level, parent_ref, delivery_available, delivery_cost, delivery_in_days, sales_status)
		VALUES ($1, 'kz', 'Казахстан', 'country', '', true, '10 000 ₸', '3-4', 'active')`, orgID)

	repo := &responsestore.KnowledgeBaseRepo{Pool: pool}
	kb, err := repo.Load(ctx, orgID.String())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(kb.DeliveryZones) != 1 || kb.DeliveryZones[0].Ref != "kz" || kb.DeliveryZones[0].ZoneLevel != "country" {
		t.Fatalf("zones = %+v", kb.DeliveryZones)
	}
	if _, err := aiprompt.BuildCatalog(kb); err != nil {
		t.Fatalf("loaded KB with zones must build a valid catalog: %v", err)
	}
}
