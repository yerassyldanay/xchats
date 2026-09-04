package kbstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
)

// SeedDemoKB inserts a small, complete "Demo Shop" knowledge base — assistant
// config, contacts, policies, 3 topics, 5 products (3 in stock, 2 not), 2
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

	// 1. Media Assets
	var coffeeMatID, blenderMatID, parcelMatID *uuid.UUID
	if len(demoCoffeeMachineJpg) > 0 {
		id := uuid.New()
		coffeeMatID = &id
		sha := sha256.Sum256(demoCoffeeMachineJpg)
		key := "demo-coffee-machine-" + id.String() + ".jpg"
		if blobStore != nil {
			_, _ = blobStore.Put(key, demoCoffeeMachineJpg, blob.Meta{
				MediaType: "image", Mimetype: "image/jpeg", FileName: "coffee-machine.jpg",
				FileSize: int64(len(demoCoffeeMachineJpg)), OrgID: orgID.String(),
			})
		}
		_, _ = tx.Exec(ctx, `
			INSERT INTO kbd_materials (id, organization_id, source_type, filename, mime_type, size_bytes,
				sha256_checksum, processing_status, customer_visibility, storage_backend, storage_key)
			VALUES ($1, $2, 'file', 'delonghi-coffee-machine.jpg', 'image/jpeg', $3, $4, 'parsed', 'visible', 'disk', $5)`,
			id, orgID, int64(len(demoCoffeeMachineJpg)), hex.EncodeToString(sha[:]), key)
	}

	if len(demoBlenderJpg) > 0 {
		id := uuid.New()
		blenderMatID = &id
		sha := sha256.Sum256(demoBlenderJpg)
		key := "demo-blender-" + id.String() + ".jpg"
		if blobStore != nil {
			_, _ = blobStore.Put(key, demoBlenderJpg, blob.Meta{
				MediaType: "image", Mimetype: "image/jpeg", FileName: "blender.jpg",
				FileSize: int64(len(demoBlenderJpg)), OrgID: orgID.String(),
			})
		}
		_, _ = tx.Exec(ctx, `
			INSERT INTO kbd_materials (id, organization_id, source_type, filename, mime_type, size_bytes,
				sha256_checksum, processing_status, customer_visibility, storage_backend, storage_key)
			VALUES ($1, $2, 'file', 'bosch-blender.jpg', 'image/jpeg', $3, $4, 'parsed', 'visible', 'disk', $5)`,
			id, orgID, int64(len(demoBlenderJpg)), hex.EncodeToString(sha[:]), key)
	}

	if len(demoDeliveryParcelJpg) > 0 {
		id := uuid.New()
		parcelMatID = &id
		sha := sha256.Sum256(demoDeliveryParcelJpg)
		key := "demo-delivery-parcel-" + id.String() + ".jpg"
		if blobStore != nil {
			_, _ = blobStore.Put(key, demoDeliveryParcelJpg, blob.Meta{
				MediaType: "image", Mimetype: "image/jpeg", FileName: "delivery-parcel.jpg",
				FileSize: int64(len(demoDeliveryParcelJpg)), OrgID: orgID.String(),
			})
		}
		_, _ = tx.Exec(ctx, `
			INSERT INTO kbd_materials (id, organization_id, source_type, filename, mime_type, size_bytes,
				sha256_checksum, processing_status, customer_visibility, storage_backend, storage_key)
			VALUES ($1, $2, 'file', 'express-delivery.jpg', 'image/jpeg', $3, $4, 'parsed', 'visible', 'disk', $5)`,
			id, orgID, int64(len(demoDeliveryParcelJpg)), hex.EncodeToString(sha[:]), key)
	}

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

	if err := upsertPolicyRow(ctx, tx, orgID, DraftPolicy{
		FreeDeliveryFrom: "20 000 ₸", MinOrder: "5 000 ₸",
		OutsideZonesNote: "В города и страны за пределами списка зон доставки мы не доставляем.",
	}); err != nil {
		return false, err
	}

	topics := []DraftTopic{
		{Slug: "demo_catalog", Title: "Каталог",
			BodyMD:        "В каталоге бытовая техника для дома и кухни. Актуальные позиции, цены и наличие — только из блоков товаров, не перечисляй товары по памяти.",
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
		{Ref: "demo_coffee-machine", Name: "Кофемашина DeLonghi", Price: "129 900 ₸",
			Description: "Автоматическая кофемашина для дома с капучинатором и жерновковой кофемолкой.", AvailabilityStatus: "in_stock",
			FeaturedImage: coffeeMatID},
		{Ref: "demo_blender", Name: "Блендер Bosch", Price: "11 200 ₸",
			Description: "Мощный блендер для смузи, соусов и супов-пюре — несколько скоростей и импульсный режим.", AvailabilityStatus: "in_stock",
			FeaturedImage: blenderMatID},
		{Ref: "demo_kettle", Name: "Чайник Bosch", Price: "40 200 ₸",
			Description: "Электрический чайник с быстрым закипанием и автоматическим отключением.", AvailabilityStatus: "unavailable"},
		{Ref: "demo_toaster", Name: "Тостер Tefal", Price: "81 600 ₸",
			Description: "Компактный тостер с регулировкой степени поджаривания и функцией разморозки.", AvailabilityStatus: "in_stock"},
		{Ref: "demo_vacuum", Name: "Пылесос Samsung", Price: "83 800 ₸",
			Description: "Пылесос с мешком для сбора пыли и насадками для разных типов покрытий.", AvailabilityStatus: "unavailable"},
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
				Name:               "Тостер Tefal OptiGrill",
				Price:              "74 900 ₸",
				Description:        "Компактный тостер с 7 режимами обжаривания и съемным поддоном для крошек. Специальная осенняя цена!",
				AvailabilityStatus: "in_stock",
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
