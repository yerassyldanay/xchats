package responsestore_test

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/response"
)

func TestKnowledgeBaseRepo_NotConfigured(t *testing.T) {
	repo, st, _ := dbtest.NewKBRepo(t)
	org, err := st.SeedOrganization(context.Background(), "xchats-test")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}

	if _, err := repo.Load(context.Background(), org.ID.String()); err != responsestore.ErrKBNotConfigured {
		t.Fatalf("Load() error = %v, want ErrKBNotConfigured", err)
	}
}

func TestKnowledgeBaseRepo_LoadsFullKB(t *testing.T) {
	repo, st, db := dbtest.NewKBRepo(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "xchats-test")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	orgID := org.ID

	mustExec(t, db, `INSERT INTO ai_assistants (organization_id, persona, mission, guardrails, language_policy, reply_max_words)
		VALUES ($1, 'Персона', 'Миссия', 'Правила', 'Языковая политика', 100)`, orgID)
	mustExec(t, db, `INSERT INTO ai_topics (organization_id, slug, title, body_md)
		VALUES ($1, 'delivery', 'Доставка', 'Доставляем по городу.')`, orgID)
	mustExec(t, db, `INSERT INTO ai_products (organization_id, ref, name, price, description, category, availability_status)
		VALUES ($1, 'widget', 'Виджет', '1 000 ₸', 'Описание', '', 'in_stock')`, orgID)
	mustExec(t, db, `INSERT INTO ai_tariffs (organization_id, ref, name, price, limit_text, fee, summary, pricing_type, advantages, disadvantages)
		VALUES ($1, 'basic', 'Базовый', '5 000 ₸', '', '', '', 'fixed', '', '')`, orgID)
	mustExec(t, db, `INSERT INTO ai_contacts (organization_id, phone, working_hours)
		VALUES ($1, '+7 700 000 00 00', '9:00-18:00')`, orgID)
	mustExec(t, db, `INSERT INTO ai_policies (organization_id, delivery_cost, delivery_in_days, outside_zones_note)
		VALUES ($1, '1 000 ₸', '1-2', '')`, orgID)

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
	if len(kb.Products) != 1 || kb.Products[0].SalesStatus != "active" || kb.Products[0].AvailabilityStatus != "in_stock" {
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
	repo, st, db := dbtest.NewKBRepo(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "xchats-test")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	orgID := org.ID

	mustExec(t, db, `INSERT INTO ai_assistants (organization_id, persona) VALUES ($1, 'p')`, orgID)
	mustExec(t, db, `INSERT INTO ai_policies (organization_id, outside_zones_note) VALUES ($1, 'Вне зон не доставляем.')`, orgID)
	mustExec(t, db, `INSERT INTO ai_delivery_zones (organization_id, ref, name, zone_level, parent_ref, delivery_available, delivery_cost, delivery_in_days, sales_status)
		VALUES ($1, 'kz', 'Казахстан', 'country', '', true, '10 000 ₸', '3-4', 'active')`, orgID)

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

// TestKnowledgeBaseRepo_LoadsSalonKB is the end-to-end proof that ties the
// whole vertical together through the ACTUAL production read path
// (migration 0020 -> raw SQL -> responsestore.Load -> aiprompt.BuildCatalog
// -> response.FrameFor): a specialist/service row and the two new
// ai_contacts columns, inserted exactly as the KB editor/MCP would leave
// them, must come back out with every field intact, build a valid catalog,
// and cause salon-kb@v1 (not shop-kb@v7) to be selected.
func TestKnowledgeBaseRepo_LoadsSalonKB(t *testing.T) {
	repo, st, db := dbtest.NewKBRepo(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "xchats-test")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	orgID := org.ID

	mustExec(t, db, `INSERT INTO ai_assistants (organization_id, persona) VALUES ($1, 'Ассистент салона')`, orgID)
	mustExec(t, db, `INSERT INTO ai_contacts (organization_id, phone, booking_url, schedule)
		VALUES ($1, '+7 707 000 00 00', 'https://xpayment.kz/book/salon',
		        '[{"ref":"mon","day":"Понедельник","start":"10:00","end":"21:00","breaks":[]}]')`, orgID)
	mustExec(t, db, `INSERT INTO ai_specialists (organization_id, ref, full_name, title, experience, schedule, booking_url, portfolio_images, sales_status)
		VALUES ($1, 'alina-kim', 'Алина Ким', 'Топ-стилист', '7 лет',
		        '[{"ref":"tue","day":"Вторник","start":"10:00","end":"19:00","breaks":[{"start":"13:00","end":"14:00"}]}]',
		        'https://xpayment.kz/book/aura-alina', '[]', 'active')`, orgID)
	mustExec(t, db, `INSERT INTO ai_services (organization_id, ref, parent_ref, service_type, category, name, price, duration, description, specialist_refs, sales_status)
		VALUES ($1, 'haircut-women', '', 'base', 'Волосы', 'Женская стрижка', '10 000 ₸', 60, '', '["alina-kim"]', 'active')`, orgID)
	mustExec(t, db, `INSERT INTO ai_services (organization_id, ref, parent_ref, service_type, category, name, price, duration, description, specialist_refs, sales_status)
		VALUES ($1, 'hair-spa-mask', 'haircut-women', 'addon', 'Волосы', 'Спа-уход', '5 000 ₸', 30, '', '[]', 'active')`, orgID)

	kb, err := repo.Load(ctx, orgID.String())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if kb.Contacts == nil || kb.Contacts.BookingURL != "https://xpayment.kz/book/salon" {
		t.Fatalf("contacts.booking_url = %+v", kb.Contacts)
	}
	if len(kb.Contacts.Schedule) != 1 || kb.Contacts.Schedule[0].Ref != "mon" {
		t.Fatalf("contacts.schedule = %+v", kb.Contacts.Schedule)
	}
	if len(kb.Specialists) != 1 {
		t.Fatalf("specialists = %+v", kb.Specialists)
	}
	sp := kb.Specialists[0]
	if sp.Ref != "alina-kim" || sp.FullName != "Алина Ким" || sp.BookingURL != "https://xpayment.kz/book/aura-alina" {
		t.Fatalf("specialist = %+v", sp)
	}
	if len(sp.Schedule) != 1 || sp.Schedule[0].Ref != "tue" || len(sp.Schedule[0].Breaks) != 1 {
		t.Fatalf("specialist schedule = %+v", sp.Schedule)
	}
	if len(kb.Services) != 2 {
		t.Fatalf("services = %+v", kb.Services)
	}
	var addon *aiprompt.Service
	for i := range kb.Services {
		if kb.Services[i].Ref == "hair-spa-mask" {
			addon = &kb.Services[i]
		}
	}
	if addon == nil || addon.ParentRef != "haircut-women" || addon.ServiceType != "addon" || addon.Duration == nil || *addon.Duration != 30 {
		t.Fatalf("addon service = %+v", addon)
	}

	cat, err := aiprompt.BuildCatalog(kb)
	if err != nil {
		t.Fatalf("loaded salon KB must build a valid catalog: %v", err)
	}
	if cat.FactByToken("{{service.haircut-women.price}}") == nil {
		t.Error("want a price token for the loaded base service")
	}
	if cat.FactByToken("{{specialist.alina-kim.schedule_tue}}") == nil {
		t.Error("want a schedule_tue token for the loaded specialist")
	}

	// The whole point: loading this KB through the real repository must
	// cause the response engine to select salon-kb@v1, on every channel.
	if got := response.FrameFor(kb, messaging.ChannelWhatsApp); got != aiprompt.FrameSalonKBV1RU() {
		t.Error("want salon-kb@v1 selected for a KB loaded with real specialist/service rows")
	}
	if got := response.PromptRefFor(kb, messaging.ChannelTelegram); got != aiprompt.PromptRefSalonKBV1 {
		t.Errorf("PromptRefFor = %q, want %q (salon selection must not depend on channel)", got, aiprompt.PromptRefSalonKBV1)
	}
}
