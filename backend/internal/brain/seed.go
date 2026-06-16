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
// shop assistant. Prices/contacts are value tokens, never digits in topic bodies;
// the PriceBook is the single source of numbers (rendered after drafting). This is
// the in-memory ContentSource for v1; a DB-backed snapshot drops in behind the same
// *domain.Snapshot later (todo: ai_* tables).
func SeedSnapshot() *domain.Snapshot {
	return &domain.Snapshot{
		Config: domain.AssistantConfig{
			Version: 1,
			Persona: "Ты — ассистент интернет-магазина «Demo Shop». Дружелюбный и конкретный, " +
				"объясняешь просто, без навязывания. Помогаешь выбрать тариф и оформить заказ.",
			Mission: "Помочь клиенту выбрать подходящий тариф и оформить заказ; всегда предлагай один понятный следующий шаг.",
			Guardrails: "Никогда не выдумывай цены или числа — используй только токены из базы знаний. " +
				"Если спрашивают про возврат денег, юридические вопросы или это недовольный/сложный клиент — эскалируй " +
				"(escalate=true). Всегда предлагай следующий шаг.",
			LanguagePolicy: "Отвечай на языке клиента. Если сообщение смешивает казахский и русский — отвечай по-русски.",
			ReplyMaxWords:  120,
		},
		// Values are free-text per (token, lang): units live IN the value so they
		// render verbatim. Contacts are language-neutral ('*').
		Values: domain.NewValueBook(
			domain.Value{Token: "price.basic", Lang: "ru", Text: "9 900 ₸", Description: "Тариф «Базовый» — цена в месяц"},
			domain.Value{Token: "price.standard", Lang: "ru", Text: "19 900 ₸", Description: "Тариф «Стандарт» — цена в месяц"},
			domain.Value{Token: "price.premium", Lang: "ru", Text: "39 900 ₸", Description: "Тариф «Премиум» — цена в месяц"},
			domain.Value{Token: "price.free_delivery_min", Lang: "ru", Text: "30 000 ₸", Description: "Сумма заказа для бесплатной доставки"},
			domain.Value{Token: "delivery.days", Lang: "ru", Text: "1–3 дня", Description: "Срок доставки"},
			domain.Value{Token: "delivery.days", Lang: "kk", Text: "1–3 күн", Description: "Жеткізу мерзімі"},
			domain.Value{Token: "contact.whatsapp", Lang: "*", Text: "+7 700 123 45 67", Description: "WhatsApp поддержки"},
			domain.Value{Token: "contact.email", Lang: "*", Text: "hello@demoshop.kz", Description: "E-mail поддержки"},
			domain.Value{Token: "contact.address", Lang: "*", Text: "Алматы, ул. Абая, 10", Description: "Адрес"},
		),
		Topics: []domain.Topic{
			{
				Slug: "pricing", Language: "ru", Keywords: "цена, цены, тариф, тарифы, сколько стоит, стоимость, план",
				Title: "Тарифы и цены",
				BodyMD: "У нас три тарифа: Базовый — {{price.basic}}, Стандарт — {{price.standard}}, " +
					"Премиум — {{price.premium}}. Доставка занимает {{delivery.days}}. " +
					"При заказе от {{price.free_delivery_min}} доставка бесплатная.",
			},
			{
				Slug: "delivery", Language: "ru", Keywords: "доставка, доставить, сроки, когда привезут, бесплатная доставка",
				Title: "Доставка",
				BodyMD: "Доставка занимает {{delivery.days}}. При заказе от {{price.free_delivery_min}} " +
					"доставка бесплатная, иначе рассчитывается по адресу.",
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
				BodyMD: "Связаться с нами: WhatsApp {{contact.whatsapp}}, e-mail {{contact.email}}. " +
					"Адрес: {{contact.address}}.",
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
				Ref: RefCatalogPDF, TopicSlug: "pricing", Kind: "document", Language: "ru",
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
