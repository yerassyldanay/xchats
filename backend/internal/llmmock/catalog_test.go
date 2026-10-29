package llmmock

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/llm"
)

// toolShopKB is a small auto-tools shop: three available products (two jacks
// so a bare "домкрат" is ambiguous, one with photo + video, one with photo
// only), one unavailable lift, address/opening topics, and city/region zones.
func toolShopKB() *aiprompt.KB {
	const org = "22222222-2222-2222-2222-222222222222"
	mat := func(id, mime string) aiprompt.Material {
		return aiprompt.Material{
			ID: id, OrganizationID: org, SourceType: "file", Filename: id, MimeType: mime, SizeBytes: 1024,
			StorageBackend: "disk", StorageKey: "k-" + id, ProcessingStatus: "parsed", CustomerVisibility: "visible",
		}
	}
	return &aiprompt.KB{
		OrganizationID: org,
		Assistant:      &aiprompt.Assistant{Persona: "Ты — консультант магазина автоинструментов.", ReplyMaxWords: 120},
		Topics: []aiprompt.Topic{
			{Slug: "address", Title: "Адрес магазина", BodyMD: "Мы находимся по адресу: ул. Жибек жолы, 114/2, Шымкент. Есть парковка у входа."},
			{Slug: "opening", Title: "Открытие магазина", BodyMD: "Магазин открывается в октябре. До открытия принимаем заказы в WhatsApp."},
		},
		Products: []aiprompt.Product{
			{
				Ref: "jack-trolley-3t", Name: "Домкрат подкатной 3 т", Price: "45 000 ₸", Brand: "Force",
				Description: "Низкопрофильный подкатной домкрат для СТО.", AvailabilityStatus: "in_stock", SalesStatus: "active",
				GalleryImages: []string{"img-jack"}, DemoVideos: []string{"vid-jack"},
			},
			{
				Ref: "jack-bottle-5t", Name: "Домкрат бутылочный 5 т", Price: "12 500 ₸",
				AvailabilityStatus: "in_stock", SalesStatus: "active", GalleryImages: []string{"img-bottle"},
			},
			{
				Ref: "compressor-50l", Name: "Компрессор масляный 50 л", Price: "89 000 ₸",
				Description: "Поршневой компрессор для пневмоинструмента.", AvailabilityStatus: "preorder", SalesStatus: "active",
			},
			{Ref: "lift-2post", Name: "Подъёмник двухстоечный 4 т", Price: "1 250 000 ₸", AvailabilityStatus: "unavailable", SalesStatus: "active"},
		},
		Contacts: &aiprompt.Contacts{Address: "ул. Жибек жолы, 114/2, Шымкент", WorkingHours: "Пн–Сб 09:00–19:00"},
		Policies: &aiprompt.Policies{OutsideZonesNote: "За пределы Казахстана не доставляем."},
		DeliveryZones: []aiprompt.DeliveryZone{
			{Ref: "kz", Name: "Казахстан (другие города)", ZoneLevel: "country", DeliveryAvailable: true, DeliveryCost: "7 000 ₸", DeliveryInDays: "3–5", SalesStatus: "active"},
			{Ref: "turkestan", Name: "Туркестанская область", ZoneLevel: "region", ParentRef: "kz", DeliveryAvailable: true, DeliveryCost: "4 000 ₸", DeliveryInDays: "1–2", SalesStatus: "active"},
			{Ref: "shymkent", Name: "Шымкент", ZoneLevel: "city", ParentRef: "turkestan", DeliveryAvailable: true, DeliveryCost: "1 500 ₸", DeliveryInDays: "1", SalesStatus: "active"},
		},
		Materials: []aiprompt.Material{mat("img-jack", "image/jpeg"), mat("vid-jack", "video/mp4"), mat("img-bottle", "image/jpeg")},
	}
}

type mockTurn struct {
	history []aiprompt.HistoryTurn
	message string
}

// askMock renders the real shop-kb@v7 prompt for kb plus the conversation
// tail, runs the mock, and requires the answer to pass the same
// ValidateResponseV7 + substitution the response engine applies.
func askMock(t *testing.T, kb *aiprompt.KB, turn mockTurn) (aiprompt.Response, string) {
	t.Helper()
	prefix, cat, err := aiprompt.BuildPromptV7(aiprompt.FrameShopKBV7RU(), kb)
	if err != nil {
		t.Fatalf("BuildPromptV7: %v", err)
	}
	tail := aiprompt.ConversationTail(aiprompt.RenderHistory(turn.history), turn.message)
	resp, err := NewClient().Complete(context.Background(), llm.ChatRequest{Messages: []llm.Message{
		{Role: "system", Content: prefix}, {Role: "user", Content: tail},
	}})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	parsed, issues := aiprompt.ValidateResponseV7(resp.Text, kb, cat)
	if len(issues) > 0 || parsed == nil {
		t.Fatalf("mock reply failed ValidateResponseV7: %+v\nbody: %s", issues, resp.Text)
	}
	text, err := aiprompt.SubstituteFactsLang(parsed.ReplyText, kb, cat, parsed.ReplyLanguage)
	if err != nil {
		t.Fatalf("SubstituteFactsLang: %v\nbody: %s", err, resp.Text)
	}
	if _, err := aiprompt.ResolveSend(parsed.MediaFilesToSend, kb, cat); err != nil {
		t.Fatalf("ResolveSend: %v\nbody: %s", err, resp.Text)
	}
	return *parsed, text
}

func TestCatalogReply_AnswersFromThePromptCatalog(t *testing.T) {
	kb := toolShopKB()
	cases := []struct {
		name         string
		turn         mockTurn
		lang         string
		escalate     bool
		wantText     []string
		wantMedia    []string
		wantNoMedia  bool
		forbidInText []string
	}{
		{
			name: "single product price", turn: mockTurn{message: "Здравствуйте! Сколько стоит бутылочный домкрат?"},
			lang: "ru", wantText: []string{"Домкрат бутылочный 5 т", "12 500 ₸"}, wantNoMedia: true,
		},
		{
			name: "ambiguous name lists every match", turn: mockTurn{message: "Какие у вас есть домкраты?"},
			lang: "ru", wantText: []string{"45 000 ₸", "12 500 ₸"}, wantNoMedia: true,
		},
		{
			name: "photo request attaches the gallery", turn: mockTurn{message: "Скиньте фото подкатного домкрата"},
			lang: "ru", wantText: []string{"45 000 ₸"}, wantMedia: []string{"products.jack-trolley-3t.gallery_images"},
		},
		{
			name: "video follow-up uses the conversation's product",
			turn: mockTurn{
				history: []aiprompt.HistoryTurn{
					{Role: "client", Text: "Сколько стоит подкатной домкрат?"},
					{Role: "assistant", Text: "«Домкрат подкатной 3 т» — 45 000 ₸."},
				},
				message: "А видео есть?",
			},
			lang: "ru", wantMedia: []string{"products.jack-trolley-3t.demo_videos"},
		},
		{
			name: "no video falls back to photo", turn: mockTurn{message: "Есть видео бутылочного домкрата?"},
			lang: "ru", wantText: []string{"Видео"}, wantMedia: []string{"products.jack-bottle-5t.gallery_images"},
		},
		{
			name: "preorder status is stated", turn: mockTurn{message: "Сколько стоит компрессор?"},
			lang: "ru", wantText: []string{"89 000 ₸", "предзаказ"},
		},
		{
			name: "unavailable product is not escalated", turn: mockTurn{message: "Есть двухстоечный подъёмник?"},
			lang: "ru", wantText: []string{"нет в наличии", "Подъёмник двухстоечный"}, forbidInText: []string{"1 250 000"},
		},
		{
			name: "unavailable product beats a partial match on an available one",
			turn: mockTurn{message: "Есть двухстоечный подъёмник? Или компрессор подскажите"},
			lang: "ru", wantText: []string{"нет в наличии"},
		},
		{
			name: "follow-up worded with a verb still uses the conversation's product",
			turn: mockTurn{
				history: []aiprompt.HistoryTurn{
					{Role: "client", Text: "Сколько стоит подкатной домкрат?"},
					{Role: "assistant", Text: "«Домкрат подкатной 3 т» — 45 000 ₸."},
				},
				message: "А видео есть, как он работает?",
			},
			lang: "ru", wantMedia: []string{"products.jack-trolley-3t.demo_videos"},
		},
		{
			name: "delivery to a named zone", turn: mockTurn{message: "Доставите в Туркестанскую область?"},
			lang: "ru", wantText: []string{"4 000 ₸", "1–2"},
		},
		{
			name: "address from a topic", turn: mockTurn{message: "Где вы находитесь?"},
			lang: "ru", wantText: []string{"Жибек жолы"},
		},
		{
			name: "wholesale escalates", turn: mockTurn{message: "Нужно 10 комплектов оптом для нашего СТО, какие условия?"},
			lang: "ru", escalate: true, wantText: []string{"нет этой информации"},
		},
		{
			name: "unknown product escalates", turn: mockTurn{message: "Сколько стоит стенд развал-схождения?"},
			lang: "ru", escalate: true,
		},
		{
			name: "kazakh in russian letters", turn: mockTurn{message: "Салем! Бутылочный домкрат канша турады?"},
			lang: "kk", wantText: []string{"бағасы", "12 500 ₸"},
		},
		{
			name: "kazakh photo request", turn: mockTurn{message: "Сәлеметсіз бе, подкатной домкраттың суретін жіберіңізші"},
			lang: "kk", wantMedia: []string{"products.jack-trolley-3t.gallery_images"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, text := askMock(t, kb, tc.turn)
			t.Logf("reply [%s, escalate=%v, media=%v]: %s", resp.ReplyLanguage, resp.Escalate, resp.MediaFilesToSend, text)
			if resp.ReplyLanguage != tc.lang {
				t.Errorf("reply_language = %q, want %q (text: %s)", resp.ReplyLanguage, tc.lang, text)
			}
			if resp.Escalate != tc.escalate {
				t.Errorf("escalate = %v, want %v (text: %s)", resp.Escalate, tc.escalate, text)
			}
			for _, w := range tc.wantText {
				if !strings.Contains(text, w) {
					t.Errorf("substituted text %q does not contain %q", text, w)
				}
			}
			for _, f := range tc.forbidInText {
				if strings.Contains(text, f) {
					t.Errorf("substituted text %q must not contain %q", text, f)
				}
			}
			if strings.Contains(text, "{{") {
				t.Errorf("substituted text still has a placeholder: %q", text)
			}
			if tc.wantNoMedia && len(resp.MediaFilesToSend) != 0 {
				t.Errorf("media_files_to_send = %v, want none", resp.MediaFilesToSend)
			}
			for i, m := range tc.wantMedia {
				if i >= len(resp.MediaFilesToSend) || resp.MediaFilesToSend[i] != m {
					t.Errorf("media_files_to_send = %v, want %v", resp.MediaFilesToSend, tc.wantMedia)
					break
				}
			}
		})
	}
}

func TestCatalogReply_EscalationCarriesKBGap(t *testing.T) {
	prefix, _, err := aiprompt.BuildPromptV7(aiprompt.FrameShopKBV7RU(), toolShopKB())
	if err != nil {
		t.Fatal(err)
	}
	resp, err := NewClient().Complete(context.Background(), llm.ChatRequest{Messages: []llm.Message{
		{Role: "system", Content: prefix},
		{Role: "user", Content: aiprompt.ConversationTail(aiprompt.RenderHistory(nil), "Позовите менеджера, пожалуйста")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Escalate bool `json:"escalate"`
		KBGap    struct {
			ReasonCode string `json:"reason_code"`
		} `json:"kb_gap"`
	}
	if err := json.Unmarshal([]byte(resp.Text), &body); err != nil {
		t.Fatalf("not JSON: %v (%s)", err, resp.Text)
	}
	if !body.Escalate || body.KBGap.ReasonCode != "human_requested" {
		t.Fatalf("got escalate=%v kb_gap=%q, want true/human_requested (%s)", body.Escalate, body.KBGap.ReasonCode, resp.Text)
	}
}

func TestCatalogReply_UnmatchedMessageKeepsGenericGreeting(t *testing.T) {
	prefix, _, err := aiprompt.BuildPromptV7(aiprompt.FrameShopKBV7RU(), toolShopKB())
	if err != nil {
		t.Fatal(err)
	}
	resp := customerResponse(prefix + "\n" + aiprompt.ConversationTail(aiprompt.RenderHistory(nil), "ок"))
	var out v7Response
	if err := json.Unmarshal([]byte(resp.Text), &out); err != nil {
		t.Fatal(err)
	}
	if out.ReplyText != "Здравствуйте! Чем я могу вам помочь?" {
		t.Fatalf("reply_text = %q, want the generic greeting", out.ReplyText)
	}
}
