package kbstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
)

// SeedDemoKB inserts a small, complete "Qazan Home" knowledge base — assistant
// config, contacts, policies, 3 topics, 5 photographed products, 2
// tariffs, 3 delivery zones, and staged draft changes into an org that has none yet.
func (s *Store) SeedDemoKB(ctx context.Context, orgID uuid.UUID) (inserted bool, err error) {
	return s.SeedDemoKBWithBlob(ctx, orgID, nil)
}

// SeedDemoKBWithBlob is SeedDemoKB with optional image asset uploads into blob.Store.
func (s *Store) SeedDemoKBWithBlob(ctx context.Context, orgID uuid.UUID, blobStore blob.Store) (inserted bool, err error) {
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

	// 1. Media assets. Every catalog product gets a real image so the demo is
	// useful for both visual evaluation and grounded media replies.
	seedImage := func(data []byte, slug, filename string) (*uuid.UUID, error) {
		if len(data) == 0 {
			return nil, nil
		}
		id := uuid.New()
		sha := sha256.Sum256(data)
		key := "demo-" + slug + "-" + id.String() + ".jpg"
		if blobStore != nil {
			if _, err := blobStore.Put(key, data, blob.Meta{
				MediaType: "image", Mimetype: "image/jpeg", FileName: filename,
				FileSize: int64(len(data)), OrgID: orgID.String(),
			}); err != nil {
				return nil, err
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO kbd_materials (id, organization_id, source_type, filename, mime_type, size_bytes,
				sha256_checksum, processing_status, customer_visibility, storage_backend, storage_key)
			VALUES ($1, $2, 'file', $3, 'image/jpeg', $4, $5, 'parsed', 'visible', 'disk', $6)`,
			id, orgID, filename, int64(len(data)), hex.EncodeToString(sha[:]), key); err != nil {
			return nil, err
		}
		return &id, nil
	}

	coffeeMatID, err := seedImage(demoCoffeeMachineJpg, "coffee-machine", "aurora-coffee-machine.jpg")
	if err != nil {
		return false, err
	}
	blenderMatID, err := seedImage(demoBlenderJpg, "blender", "pulse-blender.jpg")
	if err != nil {
		return false, err
	}
	parcelMatID, err := seedImage(demoDeliveryParcelJpg, "delivery-parcel", "express-delivery.jpg")
	if err != nil {
		return false, err
	}
	kettleMatID, err := seedImage(demoElectricKettleJpg, "electric-kettle", "ember-electric-kettle.jpg")
	if err != nil {
		return false, err
	}
	toasterMatID, err := seedImage(demoToasterJpg, "toaster", "sage-two-slot-toaster.jpg")
	if err != nil {
		return false, err
	}
	airFryerMatID, err := seedImage(demoAirFryerJpg, "air-fryer", "compact-air-fryer.jpg")
	if err != nil {
		return false, err
	}

	persona := "Ты — ассистент по подготовке ответов клиентам интернет-магазина кухонной техники «Qazan Home». Ты готовишь ОДИН черновик ответа, который проверит и отправит человек — ты никогда не отправляешь сообщения сам."
	mission := "Помогай клиентам выбрать технику для кухни, узнать актуальную цену и наличие, подобрать доставку и оформить заказ."
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
		WorkingHours: "Пн–Сб, 9:00–19:00", Phone: "+7 700 700 70 70", Instagram: "@qazan.home",
	}); err != nil {
		return false, err
	}

	if err := upsertPolicyRow(ctx, tx, orgID, DraftPolicy{
		FreeDeliveryFrom: "20 000 ₸", MinOrder: "5 000 ₸",
		OutsideZonesNote: "В города и страны за пределами списка зон доставки мы не доставляем.",
	}); err != nil {
		return false, err
	}

	topics := []DraftTopic{
		{Slug: "demo_catalog", Title: "Каталог",
			BodyMD:        "В каталоге кофемашины, блендеры, чайники, тостеры и другая техника для кухни. Актуальные позиции, цены и наличие — только из блоков товаров, не перечисляй товары по памяти.",
			FeaturedImage: parcelMatID},
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
		{Ref: "demo_coffee-machine", Name: "Кофемашина Aurora Barista Pro", Price: "289 900 ₸",
			Description: "Автоматическая кофемашина с жерновковой кофемолкой, капучинатором и пятью напитками в одно касание.", AvailabilityStatus: "in_stock",
			FeaturedImage: coffeeMatID},
		{Ref: "demo_blender", Name: "Блендер Pulse 1200", Price: "59 900 ₸",
			Description: "Стационарный блендер 1200 Вт со стеклянной чашей, шестью скоростями и импульсным режимом.", AvailabilityStatus: "in_stock",
			FeaturedImage: blenderMatID},
		{Ref: "demo_kettle", Name: "Чайник Ember 1.7", Price: "34 900 ₸",
			Description: "Матовый электрический чайник 1,7 л с быстрым закипанием, защитой от сухого включения и автоотключением.", AvailabilityStatus: "in_stock",
			FeaturedImage: kettleMatID},
		{Ref: "demo_toaster", Name: "Тостер Sage Toast 2", Price: "39 900 ₸",
			Description: "Двухслотовый тостер с семью степенями поджаривания, разморозкой и съёмным поддоном для крошек.", AvailabilityStatus: "in_stock",
			FeaturedImage: toasterMatID},
		{Ref: "demo_air-fryer", Name: "Аэрофритюрница Crisp Air 5L", Price: "74 900 ₸",
			Description: "Компактная аэрофритюрница на 5 литров с восемью программами и съёмной корзиной.", AvailabilityStatus: "unavailable",
			FeaturedImage: airFryerMatID},
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

	// 2. Initial Staged Draft Changes for /playground (Playground Review Demo)
	draft := DraftBlob{
		Topics: []DraftTopic{
			{
				Slug:   "autumn_deals",
				Title:  "Осенние спецпредложения",
				BodyMD: "До конца месяца действуют скидки до 25% на всю мелкую бытовую технику и аксессуары. При покупке от 50 000 ₸ дарим фирменный набор посуды!",
			},
		},
		Products: []DraftProduct{
			{
				Ref:                "demo_toaster",
				Name:               "Тостер Sage Toast 2",
				Price:              "34 900 ₸",
				Description:        "Двухслотовый тостер с семью степенями поджаривания, разморозкой и съёмным поддоном для крошек. Специальная осенняя цена!",
				AvailabilityStatus: "in_stock",
				FeaturedImage:      toasterMatID,
			},
		},
		DeliveryZones: []DraftDeliveryZone{
			{
				Ref:               "demo_taraz",
				Name:              "г. Тараз",
				ZoneLevel:         "city",
				ParentRef:         "demo_kazakhstan",
				DeliveryAvailable: true,
				DeliveryCost:      "1 800 ₸",
				DeliveryInDays:    "2–3 дня",
				Notes:             "Доставка курьерской службой до двери",
				SalesStatus:       "active",
			},
		},
		Policies: []DraftPolicy{
			{
				FreeDeliveryFrom: "15 000 ₸",
				MinOrder:         "5 000 ₸",
				OutsideZonesNote: "В города и страны за пределами списка зон доставки мы не доставляем.",
			},
		},
	}
	draftRaw, _ := json.Marshal(draft)
	_, _ = tx.Exec(ctx, `
		INSERT INTO kbd_draft (organization_id, draft, base_version)
		VALUES ($1, $2, 1)
		ON CONFLICT (organization_id) DO UPDATE SET draft = $2, base_version = base_version + 1`,
		orgID, draftRaw)

	if err := auditRow(ctx, tx, orgID, uuid.Nil, "seed", "demo KB content seeded (seed-kb-demo)"); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
