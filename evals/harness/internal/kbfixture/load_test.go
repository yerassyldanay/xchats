package kbfixture

import (
	"strings"
	"testing"
)

// validFixtureYAML is a small but complete schema-shaped fixture: one of
// everything, three products (so limit tests have something to truncate),
// and two kbd_materials rows referenced by the first product's featured_image
// and gallery_images.
const validFixtureYAML = `
organization_id: "11111111-1111-1111-1111-111111111111"

ai_assistants:
  persona: "Ты — ассистент интернет-магазина «Demo Shop»."
  mission: "Помогай клиентам выбрать товар."
  guardrails: "Никогда не выдумывай цены и факты."
  language_policy: "Отвечай только на русском языке."
  reply_max_words: 120

ai_topics:
  - slug: delivery
    title: "Доставка"
    body_md: "Доставляем по всему Алматы."
    illustration_images: []

ai_products:
  - ref: coffee-machine
    name: "Кофемашина DeLonghi"
    price: "129 900 ₸"
    description: "Автоматическая кофемашина для дома."
    category: "Кофемашины"
    in_stock: true
    sales_status: active
    featured_image: "22222222-2222-2222-2222-222222222222"
    gallery_images: ["33333333-3333-3333-3333-333333333333"]
  - ref: cookware-set
    name: "Набор посуды"
    price: "24 900 ₸"
    in_stock: false
    sales_status: active
  - ref: toaster
    name: "Тостер Philips"
    price: "15 000 ₸"
    in_stock: true
    sales_status: active

ai_tariffs:
  - ref: basic
    name: "Базовый"
    price: "5 000 ₸"
    pricing_type: fixed
    sales_status: active

ai_contacts:
  phone: "+7 700 000 00 00"
  working_hours: "9:00–18:00"

ai_policies:
  delivery_cost: "2 000 ₸"
  delivery_in_days: "1–3"

kbd_materials:
  - id: "22222222-2222-2222-2222-222222222222"
    source_type: file
    source_ref: "coffee-front.jpg"
    filename: "coffee-front.jpg"
    mime_type: "image/jpeg"
    size_bytes: 248315
    storage_backend: fake
    storage_key: "fake://bucket/2222"
    processing_status: built
    customer_visibility: visible
  - id: "33333333-3333-3333-3333-333333333333"
    source_type: file
    source_ref: "coffee-side.jpg"
    filename: "coffee-side.jpg"
    mime_type: "image/jpeg"
    size_bytes: 198211
    storage_backend: fake
    storage_key: "fake://bucket/3333"
    processing_status: built
    customer_visibility: auto
`

func TestDecode_Valid(t *testing.T) {
	kb, err := Decode([]byte(validFixtureYAML))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if kb.OrganizationID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("OrganizationID = %q", kb.OrganizationID)
	}
	if kb.Assistant == nil || kb.Assistant.Persona == "" {
		t.Error("expected Assistant to be populated")
	}
	if len(kb.Topics) != 1 || kb.Topics[0].Slug != "delivery" {
		t.Errorf("Topics = %+v", kb.Topics)
	}
	if len(kb.Products) != 3 {
		t.Fatalf("expected 3 products, got %d", len(kb.Products))
	}
	if kb.Products[0].Ref != "coffee-machine" || !kb.Products[0].InStock {
		t.Errorf("Products[0] = %+v", kb.Products[0])
	}
	if kb.Products[1].SalesStatus != "active" {
		t.Errorf("cookware-set SalesStatus = %q, want active", kb.Products[1].SalesStatus)
	}
	if kb.Products[1].InStock {
		t.Error("cookware-set must be in_stock=false")
	}
	if len(kb.Tariffs) != 1 || kb.Tariffs[0].PricingType != "fixed" {
		t.Errorf("Tariffs = %+v", kb.Tariffs)
	}
	if kb.Contacts == nil || kb.Contacts.Phone == "" {
		t.Error("expected Contacts to be populated")
	}
	if kb.Policies == nil || kb.Policies.DeliveryCost == "" {
		t.Error("expected Policies to be populated")
	}
	if len(kb.Materials) != 2 {
		t.Fatalf("expected 2 materials, got %d", len(kb.Materials))
	}
	for _, m := range kb.Materials {
		if m.OrganizationID != kb.OrganizationID {
			t.Errorf("material %s: expected inherited organization_id %q, got %q", m.ID, kb.OrganizationID, m.OrganizationID)
		}
	}
}

// TestDecode_UnknownAliases proves the loader rejects every generic alias
// this family deliberately does not support — including the specific ones
// DECISIONS.md calls out by name (availability, delivery_time, work_hours)
// and generic nesting bags (values, media, files) — as decode errors, not
// silently-ignored keys.
func TestDecode_UnknownAliases(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "top-level unknown key",
			yaml: `
organization_id: "org-1"
some_unknown_key: 1
`,
		},
		{
			name: "product availability alias for in_stock",
			yaml: `
organization_id: "org-1"
ai_products:
  - ref: p1
    name: "P1"
    in_stock: true
    sales_status: active
    availability: "В наличии"
`,
		},
		{
			name: "policy delivery_time alias for delivery_in_days",
			yaml: `
organization_id: "org-1"
ai_policies:
  delivery_time: "1-3 дня"
`,
		},
		{
			name: "contact work_hours alias for working_hours",
			yaml: `
organization_id: "org-1"
ai_contacts:
  work_hours: "9-18"
`,
		},
		{
			name: "generic values bag on a product",
			yaml: `
organization_id: "org-1"
ai_products:
  - ref: p1
    name: "P1"
    in_stock: true
    sales_status: active
    values: { price: "1000" }
`,
		},
		{
			name: "generic media bag on a topic",
			yaml: `
organization_id: "org-1"
ai_topics:
  - slug: t1
    title: "T1"
    body_md: "body"
    media: [{field: images, files: ["a.jpg"]}]
`,
		},
		{
			name: "generic files key on a material",
			yaml: `
organization_id: "org-1"
kbd_materials:
  - id: "m1"
    source_type: file
    files: ["a.jpg"]
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decode([]byte(tc.yaml)); err == nil {
				t.Fatalf("expected a decode error for %s", tc.name)
			}
		})
	}
}

// TestDecode_InStockStrictBoolean: in_stock must be an actual YAML boolean.
// Quoted strings — even ones that spell out true/false — are rejected, and
// omitting the key entirely is rejected too (NOT NULL, no default).
func TestDecode_InStockStrictBoolean(t *testing.T) {
	reject := []string{`"yes"`, `"no"`, `"true"`, `"false"`, `yes`, `no`}
	for _, v := range reject {
		t.Run("reject_"+v, func(t *testing.T) {
			yamlSrc := `
organization_id: "org-1"
ai_products:
  - ref: p1
    name: "P1"
    sales_status: active
    in_stock: ` + v + "\n"
			if _, err := Decode([]byte(yamlSrc)); err == nil {
				t.Fatalf("expected in_stock: %s to be rejected", v)
			}
		})
	}

	t.Run("reject_omitted", func(t *testing.T) {
		yamlSrc := `
organization_id: "org-1"
ai_products:
  - ref: p1
    name: "P1"
    sales_status: active
`
		if _, err := Decode([]byte(yamlSrc)); err == nil {
			t.Fatal("expected an omitted in_stock key to be rejected")
		} else if !strings.Contains(err.Error(), "in_stock") {
			t.Errorf("expected the error to name in_stock, got: %v", err)
		}
	})

	for _, v := range []string{"true", "false"} {
		t.Run("accept_"+v, func(t *testing.T) {
			yamlSrc := `
organization_id: "org-1"
ai_products:
  - ref: p1
    name: "P1"
    sales_status: active
    in_stock: ` + v + "\n"
			kb, err := Decode([]byte(yamlSrc))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			want := v == "true"
			if kb.Products[0].InStock != want {
				t.Errorf("InStock = %v, want %v", kb.Products[0].InStock, want)
			}
		})
	}
}

// TestDecode_SingularMediaColumnRejectsList proves the fixture/loader
// boundary rejects more than one reference in a singular media column: since
// featured_image is a plain string field, a YAML sequence there is a decode
// type error, not a value the loader has to notice and reject by hand.
func TestDecode_SingularMediaColumnRejectsList(t *testing.T) {
	yamlSrc := `
organization_id: "org-1"
ai_products:
  - ref: p1
    name: "P1"
    sales_status: active
    in_stock: true
    featured_image: ["id-1", "id-2"]
`
	if _, err := Decode([]byte(yamlSrc)); err == nil {
		t.Fatal("expected a decode error when featured_image holds a list")
	}
}

func TestDecode_RequiredFieldsMissing(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{"assistant missing persona", `
organization_id: "org-1"
ai_assistants:
  guardrails: "g"
  language_policy: "ru"
`},
		{"topic missing body_md", `
organization_id: "org-1"
ai_topics:
  - slug: t1
    title: "T1"
`},
		{"product missing name", `
organization_id: "org-1"
ai_products:
  - ref: p1
    in_stock: true
    sales_status: active
`},
		{"tariff missing pricing_type", `
organization_id: "org-1"
ai_tariffs:
  - ref: t1
    name: "T1"
    sales_status: active
`},
		{"material missing id", `
organization_id: "org-1"
kbd_materials:
  - source_type: file
`},
		{"missing organization_id entirely", `
ai_products: []
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decode([]byte(tc.yaml)); err == nil {
				t.Fatalf("expected an error for %s", tc.name)
			}
		})
	}
}

func TestDecode_ClosedEnums(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{"bad sales_status", `
organization_id: "org-1"
ai_products:
  - ref: p1
    name: "P1"
    in_stock: true
    sales_status: discontinued
`},
		{"bad pricing_type", `
organization_id: "org-1"
ai_tariffs:
  - ref: t1
    name: "T1"
    sales_status: active
    pricing_type: subscription
`},
		{"bad source_type", `
organization_id: "org-1"
kbd_materials:
  - id: m1
    source_type: screenshot
`},
		{"bad processing_status", `
organization_id: "org-1"
kbd_materials:
  - id: m1
    source_type: file
    processing_status: done
`},
		{"bad customer_visibility", `
organization_id: "org-1"
kbd_materials:
  - id: m1
    source_type: file
    customer_visibility: sometimes
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decode([]byte(tc.yaml)); err == nil {
				t.Fatalf("expected an error for %s", tc.name)
			}
		})
	}
}

func TestDecode_DuplicateNaturalKeys(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{"duplicate product ref", `
organization_id: "org-1"
ai_products:
  - {ref: p1, name: "A", in_stock: true, sales_status: active}
  - {ref: p1, name: "B", in_stock: true, sales_status: active}
`},
		{"duplicate topic slug", `
organization_id: "org-1"
ai_topics:
  - {slug: t1, title: "A", body_md: "a"}
  - {slug: t1, title: "B", body_md: "b"}
`},
		{"duplicate material id", `
organization_id: "org-1"
kbd_materials:
  - {id: m1, source_type: text}
  - {id: m1, source_type: text}
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decode([]byte(tc.yaml)); err == nil {
				t.Fatalf("expected a duplicate-key error for %s", tc.name)
			}
		})
	}
}

// TestDecode_MaterialCrossOrgOverride proves a kbd_materials row MAY declare
// its own organization_id (used only by negative fixtures to construct a
// cross-organization material), while inheriting the fixture-level one by
// default (already covered by TestDecode_Valid).
func TestDecode_MaterialCrossOrgOverride(t *testing.T) {
	yamlSrc := `
organization_id: "org-1"
kbd_materials:
  - id: m1
    organization_id: "org-2"
    source_type: file
    filename: "x.jpg"
    mime_type: "image/jpeg"
    size_bytes: 10
    storage_backend: fake
    storage_key: "fake://x"
    processing_status: built
    customer_visibility: visible
`
	kb, err := Decode([]byte(yamlSrc))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if kb.Materials[0].OrganizationID != "org-2" {
		t.Errorf("expected the material's explicit organization_id to override the fixture default, got %q", kb.Materials[0].OrganizationID)
	}
}

// TestDecode_DeliveryZones covers the loader's own shape validation for
// ai_delivery_zones — required fields, the closed zone_level/sales_status
// enums, delivery_available as a required StrictBool (same NOT-NULL
// discipline as in_stock), and duplicate refs. Cross-row business rules
// (available/cost consistency, parent_ref existence, the flat/zone
// mutual-exclusion invariant) are aiprompt.BuildCatalog's job, not the
// loader's — see delivery_test.go in backend/aiprompt and this package's
// testdata/invalid fixtures.
func TestDecode_DeliveryZones(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{"missing ref", `
organization_id: "org-1"
ai_delivery_zones:
  - name: "Zone"
    zone_level: city
    delivery_available: true
    delivery_cost: "1 000 ₸"
    delivery_in_days: "1"
    sales_status: active
`, true},
		{"missing name", `
organization_id: "org-1"
ai_delivery_zones:
  - ref: z1
    zone_level: city
    delivery_available: true
    delivery_cost: "1 000 ₸"
    delivery_in_days: "1"
    sales_status: active
`, true},
		{"bad zone_level", `
organization_id: "org-1"
ai_delivery_zones:
  - ref: z1
    name: "Zone"
    zone_level: planet
    delivery_available: true
    delivery_cost: "1 000 ₸"
    delivery_in_days: "1"
    sales_status: active
`, true},
		{"delivery_available omitted", `
organization_id: "org-1"
ai_delivery_zones:
  - ref: z1
    name: "Zone"
    zone_level: city
    delivery_cost: "1 000 ₸"
    delivery_in_days: "1"
    sales_status: active
`, true},
		{"delivery_available non-canonical spelling", `
organization_id: "org-1"
ai_delivery_zones:
  - ref: z1
    name: "Zone"
    zone_level: city
    delivery_available: "yes"
    delivery_cost: "1 000 ₸"
    delivery_in_days: "1"
    sales_status: active
`, true},
		{"bad sales_status", `
organization_id: "org-1"
ai_delivery_zones:
  - ref: z1
    name: "Zone"
    zone_level: city
    delivery_available: true
    delivery_cost: "1 000 ₸"
    delivery_in_days: "1"
    sales_status: discontinued
`, true},
		{"duplicate zone ref", `
organization_id: "org-1"
ai_delivery_zones:
  - {ref: z1, name: "A", zone_level: city, delivery_available: false, sales_status: active}
  - {ref: z1, name: "B", zone_level: city, delivery_available: false, sales_status: active}
`, true},
		{"valid deny zone with no parent_ref", `
organization_id: "org-1"
ai_delivery_zones:
  - ref: z1
    name: "Zone"
    zone_level: city
    delivery_available: false
    sales_status: active
`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode([]byte(tc.yaml))
			if tc.wantErr && err == nil {
				t.Fatalf("expected a decode error for %s", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected decode error for %s: %v", tc.name, err)
			}
		})
	}
}

// TestDecode_DeliveryZoneFieldsRoundTrip proves every AIDeliveryZone field —
// including the optional parent_ref and notes — decodes into the matching
// aiprompt.DeliveryZone field.
func TestDecode_DeliveryZoneFieldsRoundTrip(t *testing.T) {
	yamlSrc := `
organization_id: "org-1"
ai_policies:
  outside_zones_note: "Не доставляем за пределы зоны."
ai_delivery_zones:
  - ref: astana
    name: "Астана"
    zone_level: city
    parent_ref: kazakhstan
    delivery_available: true
    delivery_cost: "4 000 ₸"
    delivery_in_days: "1"
    sales_status: active
  - ref: kazakhstan
    name: "Казахстан"
    zone_level: country
    delivery_available: true
    delivery_cost: "10 000 ₸"
    delivery_in_days: "3–4"
    sales_status: active
  - ref: baikonur
    name: "Байконур"
    zone_level: city
    parent_ref: kazakhstan
    delivery_available: false
    notes: "Особый статус."
    sales_status: active
`
	kb, err := Decode([]byte(yamlSrc))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(kb.DeliveryZones) != 3 {
		t.Fatalf("expected 3 delivery zones, got %d", len(kb.DeliveryZones))
	}
	astana := kb.DeliveryZones[0]
	if astana.Ref != "astana" || astana.Name != "Астана" || astana.ZoneLevel != "city" ||
		astana.ParentRef != "kazakhstan" || !astana.DeliveryAvailable ||
		astana.DeliveryCost != "4 000 ₸" || astana.DeliveryInDays != "1" || astana.SalesStatus != "active" {
		t.Errorf("astana = %+v", astana)
	}
	baikonur := kb.DeliveryZones[2]
	if baikonur.DeliveryAvailable || baikonur.Notes != "Особый статус." {
		t.Errorf("baikonur = %+v", baikonur)
	}
	if kb.Policies == nil || kb.Policies.OutsideZonesNote == "" {
		t.Error("expected outside_zones_note to round-trip onto Policies")
	}
}

func TestApplyLimits(t *testing.T) {
	kb, err := Decode([]byte(validFixtureYAML))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	kb, err = ApplyLimits(kb, map[string]int{"ai_products": 2})
	if err != nil {
		t.Fatalf("ApplyLimits: %v", err)
	}
	if len(kb.Products) != 2 {
		t.Fatalf("expected 2 products after limiting, got %d", len(kb.Products))
	}
	if kb.Products[0].Ref != "coffee-machine" || kb.Products[1].Ref != "cookware-set" {
		t.Errorf("unexpected truncation order: %+v", kb.Products)
	}
}

func TestApplyLimits_LargerThanAvailableIsNoop(t *testing.T) {
	kb, err := Decode([]byte(validFixtureYAML))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	kb, err = ApplyLimits(kb, map[string]int{"ai_products": 1000})
	if err != nil {
		t.Fatalf("ApplyLimits: %v", err)
	}
	if len(kb.Products) != 3 {
		t.Errorf("expected all 3 products to survive a large limit, got %d", len(kb.Products))
	}
}

func TestApplyLimits_UnknownTable(t *testing.T) {
	kb, err := Decode([]byte(validFixtureYAML))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if _, err := ApplyLimits(kb, map[string]int{"product": 10}); err == nil {
		t.Fatal("expected an error for the legacy singular table name \"product\"")
	}
}

func TestApplyLimits_Deterministic(t *testing.T) {
	for i := 0; i < 5; i++ {
		kb, err := Decode([]byte(validFixtureYAML))
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		kb, err = ApplyLimits(kb, map[string]int{"ai_products": 2, "ai_topics": 1, "ai_tariffs": 1})
		if err != nil {
			t.Fatalf("ApplyLimits: %v", err)
		}
		if len(kb.Products) != 2 || kb.Products[0].Ref != "coffee-machine" {
			t.Fatalf("run %d: non-deterministic limiting: %+v", i, kb.Products)
		}
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	if _, err := Load("/no/such/fixture.yaml"); err == nil {
		t.Fatal("expected an error for a nonexistent fixture path")
	}
}
