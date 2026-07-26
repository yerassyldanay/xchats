package aiprompt

// Shared fixtures for the aiprompt test suite. Every test that needs a valid
// starting point copies baseKB() and mutates only what it is testing — this
// keeps each fail-closed case isolated to the one field it breaks.

const (
	testOrg  = "11111111-1111-1111-1111-111111111111"
	otherOrg = "99999999-9999-9999-9999-999999999999"
)

// testMaterial returns a fully valid, customer-sendable image material owned
// by testOrg. Callers mutate individual fields to construct fail-closed cases.
func testMaterial(id string) Material {
	return Material{
		ID:                 id,
		OrganizationID:     testOrg,
		SourceType:         "file",
		SourceRef:          "source-ref-" + id + ".jpg",
		Filename:           "photo-" + id + ".jpg",
		MimeType:           "image/jpeg",
		SizeBytes:          2048,
		StorageBackend:     "fake",
		StorageKey:         "fake://bucket/" + id,
		ProcessingStatus:   "built",
		CustomerVisibility: "visible",
	}
}

// testDocMaterial returns a valid, customer-sendable PDF document material.
func testDocMaterial(id string) Material {
	m := testMaterial(id)
	m.MimeType = "application/pdf"
	m.Filename = "doc-" + id + ".pdf"
	return m
}

// baseKB returns a minimal-but-complete valid KB: one fully media-populated
// product, one media-less product, one topic, one tariff, contacts, and
// policies. Every referenced material is valid. Tests copy this and mutate
// only the field under test.
func baseKB() *KB {
	return &KB{
		OrganizationID: testOrg,
		Assistant: &Assistant{
			Persona:        "Ты — ассистент интернет-магазина «Demo Shop».",
			Mission:        "Помогай клиентам выбрать товар и оформить заказ.",
			Guardrails:     "Никогда не выдумывай цены, числа или контакты.",
			LanguagePolicy: "Отвечай только на русском языке.",
			ReplyMaxWords:  120,
		},
		Topics: []Topic{
			{
				Slug: "delivery", Title: "Доставка", Keywords: []string{"доставка", "сроки"},
				BodyMD: "Доставляем по всему Алматы в течение нескольких дней.",
			},
		},
		Products: []Product{
			{
				Ref: "coffee-machine", Name: "Кофемашина DeLonghi", Price: "129 900 ₸",
				Description: "Автоматическая кофемашина для дома.", Category: "Кофемашины",
				InStock: true, SalesStatus: "active",
				FeaturedImage: "m-cm-featured",
				GalleryImages: []string{"m-cm-gallery-1", "m-cm-gallery-2"},
			},
			{
				Ref: "cookware-set", Name: "Набор посуды", Price: "24 900 ₸",
				InStock: false, SalesStatus: "active",
			},
		},
		Tariffs: []Tariff{
			{Ref: "basic", Name: "Базовый", Price: "5 000 ₸", PricingType: "fixed", SalesStatus: "active"},
		},
		Contacts: &Contacts{
			Phone: "+7 700 000 00 00", WorkingHours: "9:00–18:00",
		},
		Policies: &Policies{
			DeliveryCost: "2 000 ₸", DeliveryInDays: "1–3",
		},
		Materials: []Material{
			testMaterial("m-cm-featured"),
			testMaterial("m-cm-gallery-1"),
			testMaterial("m-cm-gallery-2"),
		},
	}
}

// zonesKB returns a valid KB with ai_delivery_zones populated: one country
// fallback ("kazakhstan"), one available city inside it ("astana"), and one
// explicit deny city inside it ("baikonur") — enough to exercise most-
// specific-wins resolution, the deny-beats-parent case, and the flat/zone
// mutual-exclusion invariant. Callers copy and mutate only the field under
// test, same discipline as baseKB.
func zonesKB() *KB {
	kb := baseKB()
	kb.Policies.DeliveryCost = ""
	kb.Policies.DeliveryInDays = ""
	kb.Policies.OutsideZonesNote = "В другие страны и регионы не доставляем."
	kb.DeliveryZones = []DeliveryZone{
		{
			Ref: "kazakhstan", Name: "Казахстан (остальные города)", ZoneLevel: "country",
			DeliveryAvailable: true, DeliveryCost: "10 000 ₸", DeliveryInDays: "3–4",
			SalesStatus: "active",
		},
		{
			Ref: "astana", Name: "Астана", ZoneLevel: "city", ParentRef: "kazakhstan",
			DeliveryAvailable: true, DeliveryCost: "4 000 ₸", DeliveryInDays: "1",
			SalesStatus: "active",
		},
		{
			Ref: "baikonur", Name: "Байконур", ZoneLevel: "city", ParentRef: "kazakhstan",
			DeliveryAvailable: false, Notes: "Закрытый административный статус — доставка недоступна.",
			SalesStatus: "active",
		},
	}
	return kb
}

// basicFrame is a minimal frame exercising every prompt slot once.
const basicFrame = `# ASSISTANT
%%ASSISTANT%%

# KNOWLEDGE BASE
%%KNOWLEDGE_BASE%%

# DESCRIPTIONS
%%DESCRIPTIONS%%

# FACTS
%%FACTS%%

# MEDIA
%%MEDIA%%

# MEDIA ABSENT
%%MEDIA_ABSENT%%

# RESPONSE SCHEMA
%%RESPONSE_SCHEMA%%
`

// v2Frame is a minimal frame exercising every v2 canonical-block slot once —
// the counterpart to basicFrame for the 2026-07 canonical-block prompt.
const v2Frame = `# ASSISTANT
%%ASSISTANT%%

# PRODUCTS IN STOCK
%%PRODUCTS_IN_STOCK%%
%%STOCK_WORDING_NOTE%%

# PRODUCTS OUT OF STOCK
%%PRODUCTS_OUT_OF_STOCK%%

# TOPICS
%%TOPICS%%

# DELIVERY ZONES
%%DELIVERY_ZONES%%

# BUSINESS FACTS
%%BUSINESS_FACTS%%

# RESPONSE SCHEMA
%%RESPONSE_SCHEMA%%
`

func issueCodes(issues []ContractIssue) []string {
	codes := make([]string, len(issues))
	for i, is := range issues {
		codes[i] = is.Code
	}
	return codes
}

func containsCode(issues []ContractIssue, code string) bool {
	for _, is := range issues {
		if is.Code == code {
			return true
		}
	}
	return false
}

func issueDetail(issues []ContractIssue, code string) string {
	for _, is := range issues {
		if is.Code == code {
			return is.Detail
		}
	}
	return ""
}
