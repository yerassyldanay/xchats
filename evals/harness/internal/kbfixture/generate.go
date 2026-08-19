package kbfixture

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// This file generates the committed evals/scenarios/shop-kb-v1/data-ru.yaml
// fixture deterministically: every value derives from a fixed input (an
// index, a ref, a column name — never time or randomness), so re-running the
// generator reproduces the committed file byte-for-byte. See
// generate_test.go for the drift check and cmd/genkbfixture for the CLI that
// writes the file.

// shopKBOrgID is the fixed organization_id every shop-kb-v1 fixture row
// belongs to.
const shopKBOrgID = "22222222-2222-2222-2222-222222222222"

// generatedFileHeader is prepended verbatim to the committed YAML so a reader
// (and a future editor) sees immediately that hand edits do not survive
// regeneration.
const generatedFileHeader = `# evals/scenarios/shop-kb-v1/data-ru.yaml — GENERATED, do not hand-edit.
#
# Produced by running: cd evals/harness && go run ./cmd/genkbfixture
# from internal/kbfixture/generate.go. To change this fixture, edit the
# generator and regenerate — hand edits desync from generate_test.go's
# byte-for-byte drift check and will be silently overwritten by the next run.
#
# Schema-shaped Russian fixture for the shop-kb-v1 eval family: every
# top-level key is an exact ai_*/kbd_materials table name and every nested
# key is an exact business column name (see this package's doc comment for
# the fixture-projection rules). 100 products — two pinned (coffee-machine:
# in stock, full media; cookware-set: out of stock, gallery only) plus 98
# generated across a 14-kind x 7-brand matrix, in_stock false at index%3==0,
# roughly half carrying purpose-named media in a rotating mix of full/
# partial/none coverage so every size band (30/60/100) sees all three. Nine
# topics, including number-free warranty prose and delivery's illustration
# image. ai_policies.main.return_period_in_days is deliberately empty — the
# missing-exact-value escalation case. kbd_materials holds one metadata row
# per referenced file (deterministic UUIDv5 ids, a mix of "visible" and
# "auto" customer_visibility) plus a couple of deliberately unreferenced
# rows, proving an unreferenced material never affects rendering.
#
# ai_delivery_zones holds four rows exercising the closed-world delivery
# model: "kazakhstan" (country-level fallback), "astana" and "almaty" (cities
# nested under it, each with its own cheaper/faster price), and "baikonur"
# (an explicit deny row nested under the same country — proves a deny row
# beats its parent's allow). Because zones are present, ai_policies.main's
# flat delivery_cost/delivery_in_days are blank (aiprompt.BuildCatalog
# requires this — a flat answer would contradict per-zone pricing) and
# outside_zones_note carries the seller-authored refusal for every
# unlisted direction.
#
# ai_tariffs holds four paid service plans, added with prompt v5 (the first
# frame able to render a tariff at all — before it this list was empty and
# the whole table was invisible to the model): "setup-basic" (price plus a
# terms document, so a tariff media_ref renders), "setup-pro" (price plus
# advantages/disadvantages prose), "installment-12" (pricing_type
# "percentage" carrying a FEE and no price — the only fee_placeholder in the
# fixture) and "courier-legacy" (sales_status "inactive", which must never
# reach a prompt). None of them touch warranty: a warranty-duration question
# is a deliberate escalation case in this family, and a priced warranty plan
# would silently turn it into a false pass.
`

// Generate returns the deterministic shop-kb-v1 fixture.
func Generate() *Fixture {
	mb := &materialBuilder{}

	fx := &Fixture{
		OrganizationID: shopKBOrgID,
		AIAssistants: &AIAssistant{
			Persona: "Ты — ассистент по подготовке ответов клиентам интернет-магазина «Demo Shop» " +
				"в WhatsApp. Ты готовишь ОДИН черновик ответа, который проверит и отправит человек — " +
				"ты никогда не отправляешь сообщения сам.",
			Mission:        "Помогай клиентам выбрать товар, узнать актуальную цену и наличие, и оформить заказ.",
			Guardrails:     "Никогда не выдумывай цены, наличие, сроки или контактные данные — используй только подтверждённые значения. Никогда не обещай медиафайл, которого нет в каталоге.",
			LanguagePolicy: "Отвечай только на русском языке.",
			ReplyMaxWords:  120,
		},
		AITopics:   generateTopics(mb),
		AIProducts: generateProducts(mb),
		AITariffs:  generateTariffs(mb),
		AIContacts: &AIContacts{
			Phone:        "+7 727 300 00 00",
			Instagram:    "@demoshop.kz",
			WorkingHours: "Пн–Сб, 9:00–19:00",
		},
		AIPolicies: &AIPolicies{
			// DeliveryCost/DeliveryInDays are deliberately blank: ai_delivery_zones
			// below is non-empty, and a flat KB-wide delivery answer would
			// contradict per-zone pricing (aiprompt.BuildCatalog enforces this).
			FreeDeliveryFrom: "20 000 ₸",
			MinOrder:         "5 000 ₸",
			OutsideZonesNote: "В другие города, регионы и страны за пределами Казахстана мы не доставляем.",
			// ReturnPeriodInDays deliberately left empty: no placeholder is
			// generated for it, so a return-period question must escalate
			// instead of inventing a value.
		},
		AIDeliveryZones: generateDeliveryZones(),
	}

	// A couple of materials nothing references — proves an unreferenced row
	// never affects rendering (it must not appear as a token, and must not
	// prevent one either).
	mb.addUnreferenced("spare/unreferenced-1", "image")
	mb.addUnreferenced("spare/unreferenced-2", "document")

	fx.KBDMaterials = mb.materials
	return fx
}

// GenerateYAML renders Generate()'s fixture as the exact committed
// data-ru.yaml content: the header comment followed by the fixture, strict-
// decodable by Decode (see load_test.go's round-trip coverage for the
// general shape; generate_test.go pins this exact output).
func GenerateYAML() ([]byte, error) {
	body, err := yaml.Marshal(Generate())
	if err != nil {
		return nil, fmt.Errorf("kbfixture: marshal generated fixture: %w", err)
	}
	return append([]byte(generatedFileHeader), body...), nil
}

// generateTopics returns the family's ~9 Russian topics. Only "delivery"
// carries media (one illustration image); the rest are prose-only, including
// "warranty", whose body is deliberately number-free so a warranty-duration
// question cannot be answered from it and must escalate ("warranty" also has
// no fact column anywhere in the registry — prose is the only route, by
// design).
func generateTopics(mb *materialBuilder) []AITopic {
	return []AITopic{
		{
			Slug: "catalog", Title: "Каталог",
			BodyMD: "В каталоге бытовая техника для дома и кухни — от кофемашин и мелкой техники до " +
				"пылесосов и обогревателей. Актуальные позиции, цены и наличие — только из FACTS и " +
				"каталога, не перечисляй товары по памяти.",
		},
		{
			Slug: "delivery", Title: "Доставка",
			BodyMD: "Доставляем по Алматы и области; срок и стоимость зависят от адреса, а при заказе " +
				"на крупную сумму доставка становится бесплатной. Называй точные значения только из FACTS.",
			IllustrationImages: mb.addN("topics/delivery/illustration_images", "image", 1),
		},
		{
			Slug: "payment", Title: "Оплата",
			BodyMD: "Принимаем оплату картой, через Kaspi и наличными при получении. Оформление — прямо в WhatsApp.",
		},
		{
			Slug: "how_to_order", Title: "Как заказать",
			BodyMD: "Оформить заказ просто: напишите, какой товар интересует, укажите адрес доставки и " +
				"подтвердите заказ — мы пришлём счёт и оформим доставку.",
		},
		{
			Slug: "warranty", Title: "Гарантия",
			BodyMD: "На технику действует гарантия производителя — она покрывает заводской брак и " +
				"оформляется чеком или подтверждением заказа. Точный срок гарантии уточняйте у менеджера.",
		},
		{
			Slug: "returns", Title: "Возврат",
			BodyMD: "Возврат и обмен возможны, если товар не был в использовании и сохранена упаковка. Уточняйте условия у менеджера при оформлении.",
		},
		{
			Slug: "promo", Title: "Акции",
			BodyMD: "Актуальные акции и промокоды уточняйте у менеджера — здесь фиксированного списка нет.",
		},
		{
			Slug: "support", Title: "Поддержка",
			BodyMD: "Поддержка на связи в рабочее время — контакты для связи смотрите в разделе контактов.",
		},
		{
			Slug: "pickup", Title: "Самовывоз",
			BodyMD: "Самовывоз возможен со склада в Алматы по предварительной договорённости с менеджером.",
		},
	}
}

// generateTariffs returns the family's paid service plans — the fixture side of
// the v5 ТАРИФЫ block (aiprompt.SlotTariffs). Before v5 this was `ai_tariffs:
// []`, because no frame could render a tariff at all; the empty list is exactly
// why the "tariffs never reach the model" bug went ungraded for so long.
//
// Deliberately service plans for an appliance shop rather than, say, an
// extended-warranty product: "warranty" is a load-bearing ESCALATION case in
// this family (the warranty topic's body is number-free and no warranty fact
// column exists, so a duration question must escalate — see generateTopics and
// common/kb-questions-ru.yaml's «На сколько месяцев у вас гарантия…»). A priced
// warranty tariff would quietly answer that question and turn a deliberate
// escalation test into a false pass.
//
// The four rows cover the block's whole surface in one pass:
//   - setup-basic: price only, plus a terms document (media_ref rendering)
//   - setup-pro:   price plus advantages/disadvantages prose
//   - installment-12: pricing_type "percentage" with a FEE and no price — the
//     only row exercising fee_placeholder, and the reason pricing_type is
//     rendered at all (a percentage fee is a rate, not an amount)
//   - courier-legacy: sales_status "inactive" — must never appear in a prompt
func generateTariffs(mb *materialBuilder) []AITariff {
	return []AITariff{
		{
			Ref: "setup-basic", Name: "Базовая установка",
			Price:       "7 900 ₸",
			Summary:     "Подключение и первый запуск техники на месте, с проверкой работы.",
			LimitText:   "Один прибор за визит",
			PricingType: "fixed",
			SalesStatus: "active",
			// The only tariff with media: proves a "<вид>_ref" line renders for
			// tariffs exactly as it does for products.
			TermsDocuments: mb.addN("tariffs/setup-basic/terms_documents", "document", 1),
		},
		{
			Ref: "setup-pro", Name: "Расширенная установка",
			Price:         "14 900 ₸",
			Summary:       "Установка со сборкой, подключением к воде или электросети и настройкой режимов.",
			Advantages:    "Мастер настраивает технику под ваши задачи и показывает, как ей пользоваться.",
			Disadvantages: "Занимает больше времени и требует согласования даты заранее.",
			PricingType:   "fixed",
			SalesStatus:   "active",
		},
		{
			Ref: "installment-12", Name: "Рассрочка на 12 месяцев",
			Fee:         "1,5%",
			Summary:     "Оплата покупки равными частями в течение года.",
			LimitText:   "От 50 000 ₸",
			PricingType: "percentage",
			SalesStatus: "active",
		},
		{
			Ref: "courier-legacy", Name: "Курьер выходного дня",
			Price:       "4 500 ₸",
			Summary:     "Старый тариф выходного дня — больше не продаётся.",
			PricingType: "fixed",
			SalesStatus: "inactive",
		},
	}
}

// generateDeliveryZones returns the family's four delivery zones: a country-
// level fallback ("kazakhstan"), two cities nested under it with their own
// cheaper/faster pricing ("astana", "almaty"), and one city nested under the
// same country that explicitly denies delivery ("baikonur" — proving a deny
// row beats its parent's allow). None carry media or notes beyond baikonur's,
// which explains the denial instead of leaving it silently unremarkable.
func generateDeliveryZones() []AIDeliveryZone {
	yes, no := StrictBool(true), StrictBool(false)
	return []AIDeliveryZone{
		{
			Ref: "kazakhstan", Name: "Казахстан (остальные города)", ZoneLevel: "country",
			DeliveryAvailable: &yes, DeliveryCost: "10 000 ₸", DeliveryInDays: "3–4",
			SalesStatus: "active",
		},
		{
			Ref: "astana", Name: "Астана", ZoneLevel: "city", ParentRef: "kazakhstan",
			DeliveryAvailable: &yes, DeliveryCost: "4 000 ₸", DeliveryInDays: "1",
			SalesStatus: "active",
		},
		{
			Ref: "almaty", Name: "Алматы", ZoneLevel: "city", ParentRef: "kazakhstan",
			DeliveryAvailable: &yes, DeliveryCost: "5 000 ₸", DeliveryInDays: "1",
			SalesStatus: "active",
		},
		{
			Ref: "baikonur", Name: "Байконур", ZoneLevel: "city", ParentRef: "kazakhstan",
			DeliveryAvailable: &no,
			Notes:             "Особый административный статус города — курьерская доставка туда не осуществляется.",
			SalesStatus:       "active",
		},
	}
}

// productMediaPlan names which media columns one generated product populates
// and how many files each holds; zero value means fully media-less.
type productMediaPlan struct {
	featured bool
	galleryN int
	videoN   int
	certN    int
}

// mediaPlanFor rotates every generated (non-pinned) product through five
// deterministic profiles keyed by its full-array index: media-less, three
// distinct partial shapes, and one fully-populated shape. Roughly half land
// on the media-less branch (i%2==0), so "about half the products" carry
// media, and because the rotation is keyed by index (not by trailing rows),
// every size band (30/60/100) sees the same mix a full render would.
func mediaPlanFor(i int) productMediaPlan {
	if i%2 == 0 {
		return productMediaPlan{}
	}
	switch (i / 2) % 4 {
	case 0:
		return productMediaPlan{featured: true}
	case 1:
		return productMediaPlan{galleryN: 2}
	case 2:
		return productMediaPlan{galleryN: 2, videoN: 1}
	default:
		return productMediaPlan{featured: true, galleryN: 2, certN: 1}
	}
}

// productKind is one of the 14 appliance kinds the 98 generated products are
// built from; description is shared verbatim across that kind's 7 brands
// (brand identity lives in ref/name only) and is deliberately free of stock
// wording and digits.
type productKind struct {
	slug, label, description string
}

func productKinds() []productKind {
	return []productKind{
		{"blender", "Блендер", "Мощный блендер для смузи, соусов и супов-пюре — несколько скоростей и импульсный режим."},
		{"kettle", "Чайник", "Электрический чайник с быстрым закипанием и автоматическим отключением."},
		{"toaster", "Тостер", "Компактный тостер с регулировкой степени поджаривания и функцией разморозки."},
		{"multicooker", "Мультиварка", "Мультиварка с несколькими программами приготовления и отложенным стартом."},
		{"microwave", "Микроволновая печь", "Микроволновая печь с грилем и несколькими режимами мощности."},
		{"vacuum", "Пылесос", "Пылесос с мешком для сбора пыли и насадками для разных типов покрытий."},
		{"iron", "Утюг", "Утюг с функцией отпаривания и керамической подошвой."},
		{"fan", "Вентилятор", "Напольный вентилятор с несколькими скоростями обдува и таймером."},
		{"heater", "Обогреватель", "Компактный обогреватель для дома с термостатом и защитой от перегрева."},
		{"air-purifier", "Очиститель воздуха", "Очиститель воздуха с многоступенчатой фильтрацией для дома."},
		{"juicer", "Соковыжималка", "Соковыжималка для фруктов и овощей с большим загрузочным отверстием."},
		{"food-processor", "Кухонный комбайн", "Кухонный комбайн с набором насадок для нарезки, измельчения и замеса."},
		{"waffle-maker", "Вафельница", "Вафельница с антипригарным покрытием для венских и бельгийских вафель."},
		{"sandwich-maker", "Сэндвичница", "Сэндвичница с антипригарными пластинами для быстрого завтрака."},
	}
}

type productBrand struct {
	slug, label string
}

func productBrands() []productBrand {
	return []productBrand{
		{"bosch", "Bosch"}, {"philips", "Philips"}, {"xiaomi", "Xiaomi"}, {"tefal", "Tefal"},
		{"redmond", "Redmond"}, {"panasonic", "Panasonic"}, {"samsung", "Samsung"},
	}
}

// generateProducts returns exactly 100 products: two pinned (index 0, 1) and
// 98 generated (index 2..99) across the 14-kind x 7-brand matrix.
func generateProducts(mb *materialBuilder) []AIProduct {
	products := make([]AIProduct, 0, 100)

	products = append(products, buildProduct(mb, "coffee-machine", "Кофемашина DeLonghi", "129 900 ₸",
		"Автоматическая кофемашина для дома с капучинатором и жерновковой кофемолкой.",
		true, productMediaPlan{featured: true, galleryN: 3, videoN: 1, certN: 1}))

	products = append(products, buildProduct(mb, "cookware-set", "Набор посуды", "24 900 ₸",
		"Набор посуды из нержавеющей стали: кастрюли, сковороды и крышки, 12 предметов.",
		false, productMediaPlan{galleryN: 2}))

	kinds := productKinds()
	brands := productBrands()
	for i := 2; i < 100; i++ {
		kind := kinds[(i-2)/len(brands)]
		brand := brands[(i-2)%len(brands)]
		ref := kind.slug + "-" + brand.slug
		name := kind.label + " " + brand.label
		price := formatPriceTenge(3_000 + (i*4_137)%97_000)
		inStock := i%3 != 0
		products = append(products, buildProduct(mb, ref, name, price, kind.description, inStock, mediaPlanFor(i)))
	}
	return products
}

func buildProduct(mb *materialBuilder, ref, name, price, description string, inStock bool, plan productMediaPlan) AIProduct {
	stock := StrictBool(inStock)
	p := AIProduct{
		Ref: ref, Name: name, Price: price, Description: description,
		InStock: &stock, SalesStatus: "active",
	}
	if plan.featured {
		p.FeaturedImage = mb.add("products/"+ref+"/featured_image", "image")
	}
	if plan.galleryN > 0 {
		p.GalleryImages = mb.addN("products/"+ref+"/gallery_images", "image", plan.galleryN)
	}
	if plan.videoN > 0 {
		p.DemoVideos = mb.addN("products/"+ref+"/demo_videos", "video", plan.videoN)
	}
	if plan.certN > 0 {
		p.CertificateDocuments = mb.addN("products/"+ref+"/certificate_documents", "document", plan.certN)
	}
	return p
}

// formatPriceTenge renders n (rounded to the nearest 100) as "NN NNN ₸" —
// space-grouped thousands, matching every other scenario's price format.
func formatPriceTenge(n int) string {
	n = (n / 100) * 100
	digits := fmt.Sprintf("%d", n)
	var groups []string
	for len(digits) > 3 {
		cut := len(digits) - 3
		groups = append([]string{digits[cut:]}, groups...)
		digits = digits[:cut]
	}
	groups = append([]string{digits}, groups...)
	return strings.Join(groups, " ") + " ₸"
}

// materialBuilder accumulates kbd_materials rows as product/topic media
// columns are populated, so a column's ids and their backing metadata rows
// are always generated together and can never drift apart.
type materialBuilder struct {
	materials []KBDMaterial
	n         int // running count; drives the visible/auto rotation below
}

var mediaExtByKind = map[string]string{"image": "jpg", "video": "mp4", "audio": "mp3", "document": "pdf"}
var mediaMimeByKind = map[string]string{
	"image": "image/jpeg", "video": "video/mp4", "audio": "audio/mpeg", "document": "application/pdf",
}

// add generates and records one metadata-only material row keyed by a
// human-legible path (e.g. "products/coffee-machine/gallery_images/0") and
// returns its deterministic id. Every fifth material is customer_visibility
// "auto" rather than "visible" — both are sendable on the live path
// (DECISIONS.md §customer_visibility), and the main fixture must exercise
// both, not just the more common "visible".
func (b *materialBuilder) add(key, kind string) string {
	id := deterministicMaterialID(key)
	visibility := "visible"
	if b.n%5 == 4 {
		visibility = "auto"
	}
	b.n++
	size := 50_000 + (len(key)*6_907)%750_000
	b.materials = append(b.materials, KBDMaterial{
		ID:                 id,
		SourceType:         "file",
		SourceRef:          "generated://" + key,
		Filename:           slugifyKey(key) + "." + mediaExtByKind[kind],
		MimeType:           mediaMimeByKind[kind],
		SizeBytes:          int64(size),
		StorageBackend:     "fake",
		StorageKey:         "fake://shop-kb-v1/" + key,
		ProcessingStatus:   "built",
		CustomerVisibility: visibility,
	})
	return id
}

// addN generates n materials sharing keyPrefix, returning their ids in
// order — the shape every plural media column needs.
func (b *materialBuilder) addN(keyPrefix, kind string, n int) []string {
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		ids[i] = b.add(fmt.Sprintf("%s/%d", keyPrefix, i), kind)
	}
	return ids
}

// addUnreferenced records a material row without returning its id anywhere a
// product/topic could adopt it — used for the "an unreferenced material
// never affects rendering" proof.
func (b *materialBuilder) addUnreferenced(key, kind string) {
	b.add(key, kind)
}

// slugifyKey turns a "/"-separated material key into a filename stem that
// still shows its full provenance at a glance (e.g.
// "products/coffee-machine/gallery_images/0" ->
// "products-coffee-machine-gallery_images-0") — filenames never reach the
// model (ValidateNoMaterialLeak asserts exactly that), but a fixture a human
// reviews should still be legible without cross-referencing ids.
func slugifyKey(key string) string {
	return strings.ReplaceAll(key, "/", "-")
}
