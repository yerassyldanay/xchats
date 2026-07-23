package aiprompt

import "fmt"

// The registry is the code-owned, closed projection of DECISIONS.md
// §"Canonical knowledge-base schema" — which columns are exact-value facts
// (and become {{table.ref.column}} placeholders), which are trusted prose, and
// which are semantic media columns (and become <table>.<ref>.<column> tokens).
// Fact-placeholder table segments are SINGULAR (product, tariff, contact,
// policy); media-token table segments are the model-facing domain names
// (topics, products, tariffs, contacts, policies). The model cannot invent a
// table or column: anything outside this registry fails closed.

// ValueKind drives the FACTS usage note and code-side rendering behavior.
type ValueKind string

const (
	KindMoneyDisplay ValueKind = "money_display" // value already includes currency
	KindTimeDisplay  ValueKind = "time_display"  // value already includes complete time wording
	KindTextComplete ValueKind = "text_complete" // value is complete as written (links, handles)
	KindNumber       ValueKind = "number"        // bare number; unit word comes from the column meaning
	KindNumberRange  ValueKind = "number_range"  // bare number or range like 1–3
	KindStockFlag    ValueKind = "stock_flag"    // boolean; code substitutes reviewed Russian wording
	KindDeliveryFlag ValueKind = "delivery_flag" // boolean; code substitutes reviewed Russian wording
)

// StockWordingRU is the reviewed Russian wording code substitutes for the
// in_stock boolean. The model never writes the boolean literal to a customer.
var StockWordingRU = map[bool]string{
	true:  "в наличии",
	false: "нет в наличии",
}

// DeliveryWordingRU is the reviewed Russian wording code substitutes for the
// delivery_available boolean. The model never writes the boolean literal, and
// never phrases delivery availability itself — it only ever copies the token.
var DeliveryWordingRU = map[bool]string{
	true:  "доставляем",
	false: "не доставляем",
}

// StockWordingKK/DeliveryWordingKK are the Kazakh counterparts, selected when a
// response declares reply_language "kk" (2026-07 Kazakh customer-message
// testing — see ResolveFactLang/SubstituteFactsLang). The prompt and knowledge
// base stay Russian-only; only the CUSTOMER-facing reply may be Kazakh, so
// these wordings exist purely for that substitution path.
//
// DRAFT — machine-translated, pending native Kazakh speaker review before any
// billed comparison treats Kazakh output as production-quality (same doctrine
// already applied to every other Kazakh frame in this repo's history).
var StockWordingKK = map[bool]string{
	true:  "бар",
	false: "жоқ",
}

var DeliveryWordingKK = map[bool]string{
	true:  "жеткіземіз",
	false: "жеткізбейміз",
}

// stockWording/deliveryWording select the reviewed wording table for a
// response's declared reply language — "kk" selects the DRAFT Kazakh table,
// anything else (including "" and "ru") selects the native-reviewed Russian
// table, the always-safe default.
func stockWording(lang string) map[bool]string {
	if lang == "kk" {
		return StockWordingKK
	}
	return StockWordingRU
}

func deliveryWording(lang string) map[bool]string {
	if lang == "kk" {
		return DeliveryWordingKK
	}
	return DeliveryWordingRU
}

// FactColumn describes one exact-value column.
type FactColumn struct {
	Column string
	Label  string // Russian meaning shown in the FACTS catalog
	Kind   ValueKind
	Unit   string // Russian unit word for number/number_range kinds
	// UnitKK is Unit's Kazakh counterpart, told to the model alongside Unit
	// (usageNote renders both) so a Kazakh-language reply appends the correct
	// word instead of the Russian one — empty when Unit itself is empty. DRAFT,
	// pending native Kazakh review (see StockWordingKK's doc comment).
	UnitKK string
}

// MediaKind is the MIME class a media column accepts.
type MediaKind string

const (
	MediaImage    MediaKind = "image"
	MediaVideo    MediaKind = "video"
	MediaAudio    MediaKind = "audio"
	MediaDocument MediaKind = "document"
)

// MediaColumn describes one semantic media column.
type MediaColumn struct {
	Column   string
	Label    string // Russian purpose label shown in the media catalog
	Kind     MediaKind
	Singular bool // true: nullable uuid; false: ordered uuid[]
}

// factColumns maps the SINGULAR fact-table segment to its exact-value columns,
// in FACTS render order.
var factColumns = map[string][]FactColumn{
	"product": {
		{Column: "price", Label: "цена", Kind: KindMoneyDisplay},
		{Column: "in_stock", Label: "наличие", Kind: KindStockFlag},
	},
	"tariff": {
		{Column: "price", Label: "цена", Kind: KindMoneyDisplay},
		{Column: "fee", Label: "комиссия", Kind: KindMoneyDisplay},
	},
	"contact": {
		{Column: "phone", Label: "телефон", Kind: KindTextComplete},
		{Column: "whatsapp", Label: "WhatsApp", Kind: KindTextComplete},
		{Column: "email", Label: "email", Kind: KindTextComplete},
		{Column: "website", Label: "сайт", Kind: KindTextComplete},
		{Column: "instagram", Label: "Instagram", Kind: KindTextComplete},
		{Column: "working_hours", Label: "часы работы", Kind: KindTimeDisplay},
	},
	"policy": {
		{Column: "delivery_cost", Label: "стоимость доставки", Kind: KindMoneyDisplay},
		{Column: "delivery_in_days", Label: "срок доставки, в днях", Kind: KindNumberRange, Unit: "дня", UnitKK: "күн"},
		{Column: "free_delivery_from", Label: "бесплатная доставка от", Kind: KindMoneyDisplay},
		{Column: "min_order", Label: "минимальный заказ", Kind: KindMoneyDisplay},
		{Column: "return_period_in_days", Label: "срок возврата, в днях", Kind: KindNumber, Unit: "дней", UnitKK: "күн"},
		{Column: "outside_zones_note", Label: "доставка вне списка зон", Kind: KindTextComplete},
	},
	// "delivery" is a multi-row fact table (one row per ai_delivery_zones
	// entry, keyed by its own ref), unlike "policy" which is the fixed "main"
	// singleton — see DeliveryZone's doc comment for the containment model.
	"delivery": {
		{Column: "delivery_cost", Label: "стоимость доставки", Kind: KindMoneyDisplay},
		{Column: "delivery_in_days", Label: "срок доставки, в днях", Kind: KindNumberRange, Unit: "дня", UnitKK: "күн"},
		{Column: "delivery_available", Label: "доступность доставки", Kind: KindDeliveryFlag},
	},
}

// mediaColumns maps the model-facing domain (media-token) table segment to its
// closed media-column list, in catalog render order.
var mediaColumns = map[string][]MediaColumn{
	"topics": {
		{Column: "featured_image", Label: "главное изображение темы", Kind: MediaImage, Singular: true},
		{Column: "illustration_images", Label: "иллюстрации", Kind: MediaImage},
		{Column: "explainer_videos", Label: "видео-объяснения", Kind: MediaVideo},
		{Column: "narration_audio_files", Label: "аудио-объяснения", Kind: MediaAudio},
		{Column: "reference_documents", Label: "справочные документы", Kind: MediaDocument},
	},
	"products": {
		{Column: "featured_image", Label: "главное фото товара", Kind: MediaImage, Singular: true},
		{Column: "gallery_images", Label: "фото товара (галерея)", Kind: MediaImage},
		{Column: "demo_videos", Label: "видео-демонстрации", Kind: MediaVideo},
		{Column: "audio_description_files", Label: "аудио-описания", Kind: MediaAudio},
		{Column: "certificate_documents", Label: "сертификаты", Kind: MediaDocument},
		{Column: "manual_documents", Label: "инструкции", Kind: MediaDocument},
		{Column: "guarantee_documents", Label: "гарантийные документы", Kind: MediaDocument},
		{Column: "specification_documents", Label: "технические характеристики", Kind: MediaDocument},
	},
	"tariffs": {
		{Column: "featured_image", Label: "главное изображение тарифа", Kind: MediaImage, Singular: true},
		{Column: "pricing_images", Label: "прайс-карточки", Kind: MediaImage},
		{Column: "explainer_videos", Label: "видео-объяснения", Kind: MediaVideo},
		{Column: "terms_documents", Label: "условия тарифа", Kind: MediaDocument},
	},
	"contacts": {
		{Column: "contact_card_image", Label: "визитка", Kind: MediaImage, Singular: true},
		{Column: "location_map_image", Label: "карта проезда", Kind: MediaImage, Singular: true},
		{Column: "company_legal_documents", Label: "юридические документы", Kind: MediaDocument},
	},
	"policies": {
		{Column: "commerce_policy_documents", Label: "документы об условиях", Kind: MediaDocument},
	},
}

// SingletonRef is the fixed code-owned natural ref for contacts and policies.
const SingletonRef = "main"

// mediaColumnSpec returns the registry entry for table.column, or an error —
// unknown media columns fail closed, never pass through.
func mediaColumnSpec(table, column string) (MediaColumn, error) {
	cols, ok := mediaColumns[table]
	if !ok {
		return MediaColumn{}, fmt.Errorf("aiprompt: unknown media table %q", table)
	}
	for _, c := range cols {
		if c.Column == column {
			return c, nil
		}
	}
	return MediaColumn{}, fmt.Errorf("aiprompt: unknown media column %q.%q", table, column)
}

// factColumnSpec returns the closed registry entry for table.column.
func factColumnSpec(table, column string) (FactColumn, error) {
	cols, ok := factColumns[table]
	if !ok {
		return FactColumn{}, fmt.Errorf("aiprompt: unknown fact table %q", table)
	}
	for _, col := range cols {
		if col.Column == column {
			return col, nil
		}
	}
	return FactColumn{}, fmt.Errorf("aiprompt: unknown fact column %q.%q", table, column)
}

// usageNote renders the FACTS usage-note text for a fact column.
func usageNote(f FactColumn) string {
	switch f.Kind {
	case KindMoneyDisplay:
		return "значение уже содержит валюту; не добавляй ₸/тенге ещё раз"
	case KindTimeDisplay:
		return "значение уже содержит полное время; не добавляй лишних слов о времени"
	case KindTextComplete:
		return "значение полное; вставляй токен как есть"
	case KindNumber, KindNumberRange:
		if f.Unit == "" {
			return "только число"
		}
		if f.UnitKK != "" {
			return "только число; в русском ответе добавь слово «" + f.Unit + "» после токена, в казахском — «" + f.UnitKK + "»"
		}
		return "только число; добавь слово «" + f.Unit + "» после токена"
	case KindStockFlag:
		return "флаг наличия; называя наличие, вставляй сам токен — код подставит правильную формулировку; состояние out_of_stock не предлагай активно, предложи альтернативу in_stock"
	case KindDeliveryFlag:
		return "флаг доступности доставки в это направление; называя доступность, вставляй сам токен — код подставит правильную формулировку. Направление без такого токена и без токена нужной зоны — не «нет», а «неизвестно»: используй {{policy.main.outside_zones_note}}, если он есть, иначе эскалируй"
	default:
		return ""
	}
}
