package brain

import (
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
)

// mediaURLPrefix is the relative path the frontend resolves media by (served by
// GET /xchats/api/v1/media/{ref}). An asset's ref IS its blob id, so the same ref
// is used to attach the file on approve (see LoadMedia + assistant.RealDrafter).
const mediaURLPrefix = "/xchats/api/v1/media/"

// Asset refs — also the blob ids LoadMedia writes (must match KB-media files).
const (
	RefPricingCard = "pricing_card"
	RefCatalogPDF  = "catalog_pdf"
	RefIntroAudio  = "intro_audio"
)

// SeedSnapshot returns the embedded "Demo Shop" knowledge base — a small online
// shop assistant. Facts (tariff prices, product prices, support contacts) are
// CONCRETE COLUMNS on typed rows (ai_tariffs / ai_products / ai_contacts), quoted
// in replies only as {{table.slug.field}} tokens; topic bodies are PURE PROSE
// (no digits, no tokens — 14 Decision 3). This is the in-memory ContentSource for
// v1 and the boot-time fallback; the DB-backed snapshot loads the same shape.
func SeedSnapshot() *domain.Snapshot {
	tariffs := []domain.Tariff{
		{Ref: "basic", Lang: "ru", Name: "Базовый", Price: "9 900 ₸", PricingType: "fixed",
			Summary: "Стартовый тариф для небольших заказов."},
		{Ref: "standard", Lang: "ru", Name: "Стандарт", Price: "19 900 ₸", PricingType: "fixed",
			Summary: "Оптимальный тариф — доставка и поддержка включены."},
		{Ref: "premium", Lang: "ru", Name: "Премиум", Price: "39 900 ₸", PricingType: "fixed",
			Summary: "Максимум возможностей и приоритетная поддержка."},
	}
	products := []domain.Product{
		{Ref: "coffee-machine", Lang: "ru", Name: "Кофемашина DeLonghi", Price: "129 900 ₸",
			Category: "Техника", Description: "Автоматическая кофемашина для дома."},
		{Ref: "cookware-set", Lang: "ru", Name: "Набор посуды", Price: "24 900 ₸",
			Category: "Кухня", Description: "Набор из 12 предметов."},
	}
	contacts := []domain.Contact{
		{Lang: "*", WhatsApp: "+7 700 123 45 67", Email: "hello@demoshop.kz",
			Address: "Алматы, ул. Абая, 10", CallbackTime: "в течение часа"},
	}

	return &domain.Snapshot{
		Config: domain.AssistantConfig{
			Version: 1,
			Persona: "Ты — ассистент интернет-магазина «Demo Shop». Дружелюбный и конкретный, " +
				"объясняешь просто, без навязывания. Помогаешь выбрать тариф или товар и оформить заказ.",
			Mission: "Помочь клиенту выбрать подходящий тариф или товар и оформить заказ; всегда предлагай один понятный следующий шаг.",
			Guardrails: "Никогда не выдумывай цены, числа или контакты — используй только токены из списка FACTS. " +
				"Если спрашивают про возврат денег, юридические вопросы или это недовольный/сложный клиент — эскалируй " +
				"(escalate=true). Всегда предлагай следующий шаг.",
			LanguagePolicy: "Отвечай на языке клиента. Если сообщение смешивает казахский и русский — отвечай по-русски.",
			ReplyMaxWords:  120,
		},
		Tariffs:  tariffs,
		Products: products,
		Contacts: contacts,
		Facts:    domain.NewFactBook(tariffs, products, contacts),
		// Topic bodies are PURE PROSE: names may appear, but exact prices/contacts
		// never — the model quotes those as FACTS tokens when it drafts.
		Topics: []domain.Topic{
			{
				Slug: "pricing", Language: "ru", Keywords: "цена, цены, тариф, тарифы, сколько стоит, стоимость, план",
				Title: "Тарифы и цены",
				BodyMD: "У нас три тарифа: Базовый, Стандарт и Премиум — от самого доступного к самому полному. " +
					"Если спрашивают о конкретном тарифе, называйте его точную цену; для общего вопроса кратко " +
					"перечислите тарифы и предложите подобрать подходящий.",
			},
			{
				Slug: "catalog", Language: "ru", Keywords: "товар, товары, каталог, купить, техника, посуда",
				Title: "Каталог товаров",
				BodyMD: "В каталоге есть техника и товары для дома — например, кофемашина и набор посуды. " +
					"Скажите, что ищете, и я назову цену и подберу вариант.",
			},
			{
				Slug: "delivery", Language: "ru", Keywords: "доставка, доставить, сроки, когда привезут",
				Title: "Доставка",
				BodyMD: "Доставка занимает 1–3 дня. Стоимость зависит от адреса — уточню её при оформлении заказа.",
			},
			{
				Slug: "how_to_order", Language: "ru", Keywords: "как заказать, оформить, заказ, купить, оплата",
				Title: "Как оформить заказ",
				BodyMD: "Оформить заказ просто: 1) напишите, какой товар или тариф интересует; " +
					"2) укажите адрес доставки; 3) подтвердите заказ — мы пришлём счёт и оформим доставку прямо в WhatsApp.",
			},
			{
				Slug: "contacts", Language: "ru", Keywords: "контакты, связаться, телефон, почта, адрес, где вы находитесь",
				Title: "Контакты",
				BodyMD: "Связаться с нами можно в WhatsApp или по e-mail; наш адрес и реквизиты доступны по запросу.",
			},
		},
		Assets: []domain.Asset{
			{
				Ref: RefPricingCard, TopicSlug: "pricing", Kind: "image", Language: "ru",
				URL:         mediaURLPrefix + RefPricingCard,
				Title:       "Карточка тарифов",
				Description: "Карточка с тремя тарифами и ценами (RU). Для вопросов о цене.",
			},
			{
				Ref: RefCatalogPDF, TopicSlug: "catalog", Kind: "document", Language: "ru",
				URL:         mediaURLPrefix + RefCatalogPDF,
				Title:       "Каталог (PDF)",
				Description: "PDF-каталог товаров с ценами. Когда просят подробности.",
			},
			{
				Ref: RefIntroAudio, TopicSlug: "how_to_order", Kind: "audio", Language: "ru",
				URL:         mediaURLPrefix + RefIntroAudio,
				Title:       "Как оформить заказ (аудио)",
				Description: "Голосовое: как оформить заказ за минуту.",
			},
		},
		Loaded: time.Now(),
	}
}
