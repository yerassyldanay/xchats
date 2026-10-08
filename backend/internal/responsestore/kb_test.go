package responsestore_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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
	now := time.Now()

	repo, st, db := dbtest.NewKBRepo(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "xchats-test")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	orgID := org.ID

	mustExec(t, db, `INSERT INTO ai_assistants (organization_id, persona, mission, guardrails, language_policy, reply_max_words, id, created_at, updated_at)
		VALUES ($1, 'Персона', 'Миссия', 'Правила', 'Языковая политика', 100, $2, $3, $3)`, orgID, uuid.New(), now)
	mustExec(t, db, `INSERT INTO ai_topics (organization_id, slug, title, body_md, id, created_at, updated_at)
		VALUES ($1, 'delivery', 'Доставка', 'Доставляем по городу.', $2, $3, $3)`, orgID, uuid.New(), now)
	mustExec(t, db, `INSERT INTO ai_products (organization_id, ref, name, price, description, category, availability_status, id, created_at, updated_at)
		VALUES ($1, 'widget', 'Виджет', '1 000 ₸', 'Описание', '', 'in_stock', $2, $3, $3)`, orgID, uuid.New(), now)
	mustExec(t, db, `INSERT INTO ai_tariffs (organization_id, ref, name, price, limit_text, fee, summary, pricing_type, advantages, disadvantages, id, created_at, updated_at)
		VALUES ($1, 'basic', 'Базовый', '5 000 ₸', '', '', '', 'fixed', '', '', $2, $3, $3)`, orgID, uuid.New(), now)
	mustExec(t, db, `INSERT INTO ai_contacts (organization_id, phone, working_hours, id, created_at, updated_at)
		VALUES ($1, '+7 700 000 00 00', '9:00-18:00', $2, $3, $3)`, orgID, uuid.New(), now)
	mustExec(t, db, `INSERT INTO ai_policies (organization_id, delivery_cost, delivery_in_days, outside_zones_note, id, created_at, updated_at)
		VALUES ($1, '1 000 ₸', '1-2', '', $2, $3, $3)`, orgID, uuid.New(), now)

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
	now := time.Now()

	repo, st, db := dbtest.NewKBRepo(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "xchats-test")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	orgID := org.ID

	mustExec(t, db, `INSERT INTO ai_assistants (organization_id, persona, id, created_at, updated_at) VALUES ($1, 'p', $2, $3, $3)`, orgID, uuid.New(), now)
	mustExec(t, db, `INSERT INTO ai_policies (organization_id, outside_zones_note, id, created_at, updated_at) VALUES ($1, 'Вне зон не доставляем.', $2, $3, $3)`, orgID, uuid.New(), now)
	mustExec(t, db, `INSERT INTO ai_delivery_zones (organization_id, ref, name, zone_level, parent_ref, delivery_available, delivery_cost, delivery_in_days, sales_status, id, created_at, updated_at)
		VALUES ($1, 'kz', 'Казахстан', 'country', '', true, '10 000 ₸', '3-4', 'active', $2, $3, $3)`, orgID, uuid.New(), now)

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
	now := time.Now()

	repo, st, db := dbtest.NewKBRepo(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "xchats-test")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	orgID := org.ID

	mustExec(t, db, `INSERT INTO ai_assistants (organization_id, persona, id, created_at, updated_at) VALUES ($1, 'Ассистент салона', $2, $3, $3)`, orgID, uuid.New(), now)
	mustExec(t, db, `INSERT INTO ai_contacts (organization_id, phone, booking_url, schedule, id, created_at, updated_at)
		VALUES ($1, '+7 707 000 00 00', 'https://xpayment.kz/book/salon',
		        '[{"ref":"mon","day":"Понедельник","start":"10:00","end":"21:00","breaks":[]}]', $2, $3, $3)`, orgID, uuid.New(), now)
	mustExec(t, db, `INSERT INTO ai_specialists (organization_id, ref, full_name, title, experience, schedule, booking_url, portfolio_images, sales_status, id, created_at, updated_at)
		VALUES ($1, 'alina-kim', 'Алина Ким', 'Топ-стилист', '7 лет',
		        '[{"ref":"tue","day":"Вторник","start":"10:00","end":"19:00","breaks":[{"start":"13:00","end":"14:00"}]}]',
		        'https://xpayment.kz/book/aura-alina', '[]', 'active', $2, $3, $3)`, orgID, uuid.New(), now)
	mustExec(t, db, `INSERT INTO ai_services (organization_id, ref, parent_ref, service_type, category, name, price, duration, description, specialist_refs, sales_status, id, created_at, updated_at)
		VALUES ($1, 'haircut-women', '', 'base', 'Волосы', 'Женская стрижка', '10 000 ₸', 60, '', '["alina-kim"]', 'active', $2, $3, $3)`, orgID, uuid.New(), now)
	mustExec(t, db, `INSERT INTO ai_services (organization_id, ref, parent_ref, service_type, category, name, price, duration, description, specialist_refs, sales_status, id, created_at, updated_at)
		VALUES ($1, 'hair-spa-mask', 'haircut-women', 'addon', 'Волосы', 'Спа-уход', '5 000 ₸', 30, '', '[]', 'active', $2, $3, $3)`, orgID, uuid.New(), now)

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

	// The whole point: the KB loaded through the real repository carries the
	// org's active template (general, seeded by SeedOrganization), and the frame
	// composed from it includes the salon data sections on every channel — there
	// is no longer a separate salon frame to select.
	if kb.PromptTemplate == nil || kb.PromptTemplate.ID != aiprompt.TemplateGeneral || kb.PromptTemplate.Instructions == "" {
		t.Fatalf("loaded KB must carry the seeded general template, got %+v", kb.PromptTemplate)
	}
	for _, ch := range []messaging.Channel{messaging.ChannelWhatsApp, messaging.ChannelTelegram} {
		frame := response.FrameFor(kb, ch)
		for _, want := range []string{aiprompt.LabelServices, aiprompt.LabelSpecialists, aiprompt.SlotServiceCatalog, aiprompt.SlotSpecialists} {
			if !strings.Contains(frame, want) {
				t.Errorf("%s frame lacks %q for a KB with real specialist/service rows", ch, want)
			}
		}
	}
	if got := response.PromptRefFor(kb); !strings.HasPrefix(got, "template:general@") {
		t.Errorf("PromptRefFor = %q, want template:general@<revision>", got)
	}
}

// The loader attaches the org's SELECTED template, read from the database, so
// the cached KB the reply path and the preview share carries operator edits.
func TestKnowledgeBaseRepo_LoadsSelectedTemplate(t *testing.T) {
	now := time.Now()
	repo, st, db := dbtest.NewKBRepo(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "xchats-test")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO ai_assistants (organization_id, persona, prompt_template_id, id, created_at, updated_at) VALUES ($1, 'p', 'service-business', $2, $3, $3)`, org.ID, uuid.New(), now)
	mustExec(t, db, `UPDATE ai_prompt_templates SET instructions = 'ПРАВКА-ОПЕРАТОРА', updated_at = 1791417612345 WHERE organization_id = $1 AND id = 'service-business'`, org.ID)

	kb, err := repo.Load(ctx, org.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if kb.PromptTemplate == nil || kb.PromptTemplate.ID != "service-business" || kb.PromptTemplate.Instructions != "ПРАВКА-ОПЕРАТОРА" ||
		kb.PromptTemplate.UpdatedAt.UnixMilli() != 1791417612345 {
		t.Fatalf("template = %+v", kb.PromptTemplate)
	}
	if got := response.PromptRefFor(kb); got != "template:service-business@1791417612345" {
		t.Errorf("PromptRefFor = %q", got)
	}
}

// A profile-selection stub (configured = FALSE) is not a configured assistant.
func TestKnowledgeBaseRepo_ProfileStubIsStillNotConfigured(t *testing.T) {
	now := time.Now()
	repo, st, db := dbtest.NewKBRepo(t)
	ctx := context.Background()
	org, _ := st.SeedOrganization(ctx, "xchats-test")
	mustExec(t, db, `INSERT INTO ai_assistants (organization_id, prompt_template_id, configured, id, created_at, updated_at) VALUES ($1, 'online-shop', FALSE, $2, $3, $3)`, org.ID, uuid.New(), now)
	if _, err := repo.Load(ctx, org.ID.String()); err != responsestore.ErrKBNotConfigured {
		t.Fatalf("Load() = %v, want ErrKBNotConfigured for a stub row", err)
	}
}

// Missing/unknown selections fall back: selected row -> the org's general row ->
// the shipped default in memory (never written back).
func TestKnowledgeBaseRepo_TemplateFallbacks(t *testing.T) {
	now := time.Now()
	repo, st, db := dbtest.NewKBRepo(t)
	ctx := context.Background()
	org, _ := st.SeedOrganization(ctx, "xchats-test")
	mustExec(t, db, `INSERT INTO ai_assistants (organization_id, prompt_template_id, id, created_at, updated_at) VALUES ($1, 'online-shop', $2, $3, $3)`, org.ID, uuid.New(), now)
	mustExec(t, db, `UPDATE ai_prompt_templates SET instructions = 'ПРАВКА-GENERAL' WHERE organization_id = $1 AND id = 'general'`, org.ID)

	load := func() *aiprompt.PromptTemplate {
		t.Helper()
		kb, err := repo.Load(ctx, org.ID.String())
		if err != nil {
			t.Fatal(err)
		}
		return kb.PromptTemplate
	}
	// selected row missing -> the org's general row
	mustExec(t, db, `DELETE FROM ai_prompt_templates WHERE organization_id = $1 AND id = 'online-shop'`, org.ID)
	if tpl := load(); tpl.ID != "general" || tpl.Instructions != "ПРАВКА-GENERAL" {
		t.Errorf("fallback to general row: %+v", tpl)
	}
	// an unknown ID in the column behaves the same
	mustExec(t, db, `UPDATE ai_assistants SET prompt_template_id = 'bogus' WHERE organization_id = $1`, org.ID)
	if tpl := load(); tpl.ID != "general" || tpl.Instructions != "ПРАВКА-GENERAL" {
		t.Errorf("unknown ID: %+v", tpl)
	}
	// no rows at all -> shipped default, in memory only
	mustExec(t, db, `DELETE FROM ai_prompt_templates WHERE organization_id = $1`, org.ID)
	want, _ := aiprompt.DefaultTemplateInstructions(aiprompt.TemplateGeneral)
	if tpl := load(); tpl.ID != "general" || tpl.Instructions != want || !tpl.UpdatedAt.IsZero() {
		t.Errorf("shipped default fallback: id=%s zeroTime=%v", tpl.ID, tpl.UpdatedAt.IsZero())
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM ai_prompt_templates WHERE organization_id = $1`, org.ID).Scan(&n); err != nil || n != 0 {
		t.Errorf("the read path must never write: %d rows (%v)", n, err)
	}
}
