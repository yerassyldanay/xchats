package kbstore

import (
	"context"

	"github.com/google/uuid"
)

// SeedDemoKB inserts a small, complete "Demo Shop" knowledge base — assistant
// config, contacts, policies, 3 topics, 5 products (3 in stock, 2 not), 2
// tariffs, and 3 delivery zones — into an org that has none yet. Content is
// the same fixed dataset migration 0008_kb_response_demo_data used to insert
// automatically (0009_remove_kb_response_demo_data deletes it again in the
// same migration run in every real environment, so that pair nets to zero
// today) — moved here so it only ever runs when explicitly asked for.
//
// Explicit and opt-in only: nothing calls this from serve's boot path or from
// RunMigrations. It is wired to its own CLI subcommand
// ("xchats seed-kb-demo" / "make seed-kb-demo") for test/demo use.
//
// Guarded on the org having zero live topics — the same invariant
// SeedLiveIfEmpty uses — so calling this against an org that already has KB
// content (including a second call against the org it just seeded) is always
// a safe no-op, never a silent overwrite of real data.
func (s *Store) SeedDemoKB(ctx context.Context, orgID uuid.UUID) (inserted bool, err error) {
	var exists bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ai_topics WHERE organization_id = $1)`,
		orgID).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	persona := "Ты — ассистент по подготовке ответов клиентам интернет-магазина «Demo Shop» в WhatsApp. Ты готовишь ОДИН черновик ответа, который проверит и отправит человек — ты никогда не отправляешь сообщения сам."
	mission := "Помогай клиентам выбрать товар, узнать актуальную цену и наличие, и оформить заказ."
	guardrails := "Никогда не выдумывай цены, наличие, сроки или контактные данные — используй только подтверждённые значения. Никогда не обещай медиафайл, которого нет в каталоге."
	languagePolicy := "Отвечай на языке сообщения клиента — по-русски или по-казахски."
	replyMaxWords := 120
	if err := upsertConfigRow(ctx, tx, orgID, ConfigPatch{
		Persona: &persona, Mission: &mission, Guardrails: &guardrails,
		LanguagePolicy: &languagePolicy, ReplyMaxWords: &replyMaxWords,
	}); err != nil {
		return false, err
	}

	if err := upsertContactRow(ctx, tx, orgID, DraftContact{
		WorkingHours: "Пн–Сб, 9:00–19:00", Phone: "+7 700 000 00 00", Instagram: "@demo.shop",
	}); err != nil {
		return false, err
	}

	// Zone-compatible from the start (blank flat delivery fields, a real
	// outside_zones_note) — see aiprompt.BuildCatalog's fail-closed invariant,
	// same reasoning as 0008's own zone-insert guard below.
	if err := upsertPolicyRow(ctx, tx, orgID, DraftPolicy{
		FreeDeliveryFrom: "20 000 ₸", MinOrder: "5 000 ₸",
		OutsideZonesNote: "В города и страны за пределами списка зон доставки мы не доставляем.",
	}); err != nil {
		return false, err
	}

	topics := []DraftTopic{
		{Slug: "demo_catalog", Title: "Каталог",
			BodyMD: "В каталоге бытовая техника для дома и кухни. Актуальные позиции, цены и наличие — только из блоков товаров, не перечисляй товары по памяти."},
		{Slug: "demo_payment", Title: "Оплата",
			BodyMD: "Принимаем оплату картой, через Kaspi и наличными при получении. Оформление — прямо в WhatsApp."},
		{Slug: "demo_warranty", Title: "Гарантия",
			BodyMD: "На технику действует гарантия производителя — она покрывает заводской брак и оформляется чеком или подтверждением заказа."},
	}
	for _, t := range topics {
		if err := upsertTopicRow(ctx, tx, orgID, t); err != nil {
			return false, err
		}
	}

	products := []DraftProduct{
		{Ref: "demo_coffee-machine", Name: "Кофемашина DeLonghi", Price: "129 900 ₸",
			Description: "Автоматическая кофемашина для дома с капучинатором и жерновковой кофемолкой.", InStock: true},
		{Ref: "demo_blender", Name: "Блендер Bosch", Price: "11 200 ₸",
			Description: "Мощный блендер для смузи, соусов и супов-пюре — несколько скоростей и импульсный режим.", InStock: true},
		{Ref: "demo_kettle", Name: "Чайник Bosch", Price: "40 200 ₸",
			Description: "Электрический чайник с быстрым закипанием и автоматическим отключением.", InStock: false},
		{Ref: "demo_toaster", Name: "Тостер Tefal", Price: "81 600 ₸",
			Description: "Компактный тостер с регулировкой степени поджаривания и функцией разморозки.", InStock: true},
		{Ref: "demo_vacuum", Name: "Пылесос Samsung", Price: "83 800 ₸",
			Description: "Пылесос с мешком для сбора пыли и насадками для разных типов покрытий.", InStock: false},
	}
	for _, p := range products {
		if err := upsertProductRow(ctx, tx, orgID, p); err != nil {
			return false, err
		}
	}

	tariffs := []DraftTariff{
		{Ref: "demo_basic", Name: "Базовая доставка", Price: "5 000 ₸",
			Summary: "Доставка курьером в удобное время.", PricingType: "fixed"},
		{Ref: "demo_express", Name: "Экспресс-доставка", Price: "8 000 ₸",
			Summary: "Доставка в течение нескольких часов.", PricingType: "fixed"},
	}
	for _, t := range tariffs {
		if err := upsertTariffRow(ctx, tx, orgID, t); err != nil {
			return false, err
		}
	}

	zones := []ZoneInput{
		{Ref: "demo_kazakhstan", Name: "Казахстан (остальные города)", ZoneLevel: "country",
			DeliveryAvailable: true, DeliveryCost: "10 000 ₸", DeliveryInDays: "3–4"},
		{Ref: "demo_almaty", Name: "Алматы", ZoneLevel: "city", ParentRef: "demo_kazakhstan",
			DeliveryAvailable: true, DeliveryCost: "5 000 ₸", DeliveryInDays: "1"},
		{Ref: "demo_baikonur", Name: "Байконур", ZoneLevel: "city", ParentRef: "demo_kazakhstan",
			DeliveryAvailable: false,
			Notes:             "Особый административный статус города — курьерская доставка туда не осуществляется."},
	}
	for _, z := range zones {
		if err := upsertZoneRow(ctx, tx, orgID, z); err != nil {
			return false, err
		}
	}

	if err := auditRow(ctx, tx, orgID, uuid.Nil, "seed", "demo KB content seeded (seed-kb-demo)"); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
