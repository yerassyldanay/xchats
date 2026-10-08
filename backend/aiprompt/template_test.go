package aiprompt

import (
	"errors"
	"strings"
	"testing"
)

// mixedKB is a business that sells products AND tariffs AND has delivery zones
// AND offers booked services with specialists AND carries policy/contact prose —
// the case the old single-frame selection could not serve.
func mixedKB() *KB {
	kb := zonesKB()
	salon := salonKB()
	kb.Specialists = salon.Specialists
	kb.Services = salon.Services
	kb.Contacts.Address = "г. Алматы, пр. Достык, 128"
	kb.Contacts.BookingURL = salon.Contacts.BookingURL
	kb.Contacts.CallbackTime = "в рабочее время"
	kb.Policies.Prepayment = "Предоплата 30% для товаров под заказ."
	kb.Policies.Installment = "Рассрочка доступна от трёх месяцев."
	kb.Policies.Warranty = "Гарантия 12 месяцев."
	kb.Materials = append(kb.Materials, testMaterial("m-alina-1"), testMaterial("m-alina-2"))
	return kb
}

func squash(s string) string { return strings.Join(strings.Fields(s), " ") }

func TestDefaultTemplates_ShippedAndValid(t *testing.T) {
	if got := PromptTemplateIDs(); len(got) != 4 || got[0] != TemplateGeneral {
		t.Fatalf("PromptTemplateIDs = %v; want four IDs, general first", got)
	}
	var bodies []string
	for _, id := range PromptTemplateIDs() {
		text, ok := DefaultTemplateInstructions(id)
		if !ok || strings.TrimSpace(text) == "" {
			t.Fatalf("no default text for %q", id)
		}
		if err := ValidateTemplateInstructions(text); err != nil {
			t.Errorf("default %q fails its own validation: %v", id, err)
		}
		i := strings.Index(text, "ИСТОЧНИК ФАКТОВ")
		if i < 0 {
			t.Fatalf("default %q has no ИСТОЧНИК ФАКТОВ section", id)
		}
		bodies = append(bodies, text[i:])
	}
	for i := 1; i < len(bodies); i++ {
		if bodies[i] != bodies[0] {
			t.Errorf("profile %q rule body differs from general: profiles must differ only in their opening", PromptTemplateIDs()[i])
		}
	}
	if _, ok := DefaultTemplateInstructions("nope"); ok {
		t.Error("unknown ID must not have default text")
	}
	if IsPromptTemplateID("") || IsPromptTemplateID("Online-Shop") {
		t.Error("IsPromptTemplateID must be exact")
	}
}

func TestValidateTemplateInstructions(t *testing.T) {
	cases := []struct {
		name string
		text string
		want error
	}{
		{"ok", "Отвечай вежливо.", nil},
		{"empty", "  \n\t", ErrTemplateEmpty},
		{"too long", strings.Repeat("я", MaxTemplateRunes+1), ErrTemplateTooLong},
		{"exactly max", strings.Repeat("я", MaxTemplateRunes), nil},
		{"slot marker", "Вставь %%FACTS%% сюда", ErrTemplateReservedSyntax},
		{"percent pair", "100%% гарантия", ErrTemplateReservedSyntax},
		{"open placeholder", "цена {{product.x.price", ErrTemplateReservedSyntax},
		{"close placeholder", "цена product.x.price}}", ErrTemplateReservedSyntax},
		{"uuid", "id 123e4567-e89b-12d3-a456-426614174000", ErrTemplateLeakShapedString},
		{"file ext", "см. файл price.pdf", ErrTemplateLeakShapedString},
		{"https url", "наш сайт https://example.com/shop", ErrTemplateLeakShapedString},
		{"storage locator", "ключ s3://bucket/key", ErrTemplateLeakShapedString},
		{"colon-slash prose ok", "пример: так / иначе", nil},
		{"single percent ok", "скидка 10% по акции", nil},
		{"single brace ok", "формат {json}", nil},
	}
	for _, tc := range cases {
		if got := ValidateTemplateInstructions(tc.text); !errors.Is(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The composed frame carries ONLY technical structure around the instructions:
// no rules, no behavior sentences, nothing an operator could contradict.
func TestComposeFrame_ProtectedStructureOnly(t *testing.T) {
	got := ComposeFrame("", mixedKB(), "WhatsApp")
	want := strings.Join([]string{
		"Канал переписки с клиентом: WhatsApp.",
		OutputContractLine,
		"%%RESPONSE_SCHEMA%%",
		"%%ASSISTANT%%",
		LabelProductsAvailable, "%%PRODUCTS_AVAILABLE%%",
		LabelProductsUnavailable, "%%PRODUCTS_UNAVAILABLE%%",
		LabelTariffs, "%%TARIFF_CATALOG%%",
		LabelServices, "%%SERVICE_CATALOG%%",
		LabelSpecialists, "%%SPECIALISTS%%",
		LabelBusinessTerms, "%%BUSINESS_TERMS%%",
		LabelTopics, "%%TOPICS%%",
		LabelDeliveryZones, "%%DELIVERY_ZONES%%",
		LabelBusinessFacts, "%%BUSINESS_FACTS%%",
	}, "\n\n") + "\n"
	if got != want {
		t.Fatalf("composed frame mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	// Instructions come first and are trimmed; no channel line when none given.
	got = ComposeFrame("\n  Правила.  \n", baseKB(), "")
	if !strings.HasPrefix(got, "Правила.\n\n"+OutputContractLine) {
		t.Errorf("instructions must lead, trimmed, with no channel line: %q", got[:80])
	}
	if strings.Contains(got, ChannelLinePrefix) {
		t.Error("an empty channel label must add no channel line")
	}
}

func TestComposeFrame_SectionsFollowTheData(t *testing.T) {
	type exp struct{ present, absent []string }
	cases := []struct {
		name string
		kb   *KB
		exp  exp
	}{
		{"nil KB", nil, exp{
			present: []string{LabelTopics, LabelBusinessFacts},
			absent:  []string{LabelProductsAvailable, LabelServices, LabelSpecialists, LabelTariffs, LabelDeliveryZones, LabelBusinessTerms},
		}},
		{"shop only", baseKB(), exp{
			present: []string{LabelProductsAvailable, LabelProductsUnavailable, LabelTariffs, LabelTopics, LabelBusinessFacts},
			absent:  []string{LabelServices, LabelSpecialists, LabelDeliveryZones, LabelBusinessTerms, LabelTariffInfo},
		}},
		{"salon only", salonKB(), exp{
			present: []string{LabelServices, LabelSpecialists, LabelBusinessTerms, LabelTopics, LabelBusinessFacts},
			absent:  []string{LabelProductsAvailable, LabelProductsUnavailable, LabelTariffs, LabelDeliveryZones},
		}},
		{"mixed", mixedKB(), exp{
			present: []string{LabelProductsAvailable, LabelProductsUnavailable, LabelTariffs, LabelServices, LabelSpecialists, LabelBusinessTerms, LabelDeliveryZones, LabelTopics, LabelBusinessFacts},
		}},
	}
	for _, tc := range cases {
		frame := ComposeFrame("x", tc.kb, "WhatsApp")
		for _, l := range tc.exp.present {
			if !strings.Contains(frame, l) {
				t.Errorf("%s: missing section %q", tc.name, l)
			}
		}
		for _, l := range tc.exp.absent {
			if strings.Contains(frame, l) {
				t.Errorf("%s: unexpected section %q", tc.name, l)
			}
		}
	}
}

// ruleSentences are the behavior sentences that must reach the model for each
// kind of data: presence of a data block alone proves nothing about whether the
// model was told how to treat it.
var ruleSentences = map[string]string{
	"add-on decline":            "ДОПОЛНИТЕЛЬНАЯ УСЛУГА ЗАПРОШЕНА ОТДЕЛЬНО",
	"add-on attribute rule":     "standalone: forbidden",
	"schedule calc only":        "schedule_reasoning мастера — служебные данные ТОЛЬКО для твоих расчётов",
	"never confirm booking":     "ты НИКОГДА не подтверждаешь запись",
	"shift state table":         "НА СМЕНЕ:",
	"unavailable product":       "ТОВАР ИЗВЕСТЕН, НО НЕДОСТУПЕН",
	"delivery zones":            "Доставка (география и доступность направления)",
	"text/flag consistency":     "СВЯЗЬ ТЕКСТА И ФЛАГА",
	"tariff fee":                "fee_placeholder",
	"service price/duration":    "duration_placeholder",
	"booking link":              "booking_placeholder",
	"portfolio media":           "portfolio_ref",
	"payment question answered": "ВОПРОС об условиях",
	"payment question not esc":  "Это НЕ эскалация",
	"payment operation":         "ДЕЙСТВИЕ с оплатой",
	"ambiguous time clarifies":  "задай один уточняющий вопрос вместо догадки",
}

func TestTemplates_MixedKBUnderEveryProfile(t *testing.T) {
	kb := mixedKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("mixed KB must build a catalog: %v", err)
	}
	for _, id := range PromptTemplateIDs() {
		t.Run(id, func(t *testing.T) {
			text, _ := DefaultTemplateInstructions(id)
			out, err := RenderPromptV7(ComposeFrame(text, kb, "WhatsApp"), kb.PromptInput(), cat)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if err := ValidateNoMaterialLeak(out, kb.Materials); err != nil {
				t.Fatal(err)
			}
			// 1. every kind of data reached the prompt.
			for _, want := range []string{
				"product: coffee-machine", "- Кофемашина DeLonghi" /* unavailable list is name-only */, "tariff: basic",
				"service: haircut-women", "specialist: alina-kim", "zone_level: country",
				"- Предоплата: Предоплата 30% для товаров под заказ.", "- Рассрочка: Рассрочка доступна",
				"- Гарантия: Гарантия 12 месяцев.", "price_placeholder: {{product.coffee-machine.price}}",
				"duration_placeholder: {{service.haircut-women.duration}}",
				"booking_placeholder: {{specialist.alina-kim.booking}}",
				"standalone: forbidden",
			} {
				if want == "- Кофемашина DeLonghi" {
					want = "- Набор посуды" // the unavailable product
				}
				if !strings.Contains(out, want) {
					t.Errorf("rendered prompt lacks %q", want)
				}
			}
			// 2. the matching behavior instructions are present too.
			norm := squash(out)
			for name, sentence := range ruleSentences {
				if !strings.Contains(norm, squash(sentence)) {
					t.Errorf("rendered prompt lacks the %s rule (%q)", name, sentence)
				}
			}
			// 3. the protected structure came out in order.
			iRules := strings.Index(out, "ИСТОЧНИК ФАКТОВ")
			iChan := strings.Index(out, ChannelLinePrefix+"WhatsApp.")
			iJSON := strings.Index(out, OutputContractLine)
			iData := strings.Index(out, LabelProductsAvailable)
			if !(0 <= iRules && iRules < iChan && iChan < iJSON && iJSON < iData) {
				t.Errorf("order wrong: rules=%d channel=%d contract=%d data=%d", iRules, iChan, iJSON, iData)
			}
			// 4. the service add-on line is a bare attribute — behavior is template text.
			if strings.Contains(out, "decline and offer the base service") {
				t.Error("renderer output carries add-on behavior; it must live in the template")
			}
			if strings.Contains(out, "%%") {
				t.Error("unfilled slot")
			}
		})
	}
}

// Conditional rules must stay inert (and the render clean) when their data is absent.
func TestTemplates_SingleKindKBsRenderCleanlyUnderEveryProfile(t *testing.T) {
	for name, kb := range map[string]*KB{"shop": baseKB(), "salon": salonKB(), "zones": zonesKB()} {
		cat, err := BuildCatalog(kb)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, id := range PromptTemplateIDs() {
			text, _ := DefaultTemplateInstructions(id)
			out, err := RenderPromptV7(ComposeFrame(text, kb, "Telegram"), kb.PromptInput(), cat)
			if err != nil {
				t.Errorf("%s/%s: %v", name, id, err)
				continue
			}
			if strings.Contains(out, "%%") {
				t.Errorf("%s/%s: unfilled slot", name, id)
			}
		}
	}
	// A salon-only KB must not grow product/tariff sections from a shop-flavoured profile.
	kb := salonKB()
	cat, _ := BuildCatalog(kb)
	text, _ := DefaultTemplateInstructions(TemplateOnlineShop)
	out, _ := RenderPromptV7(ComposeFrame(text, kb, ""), kb.PromptInput(), cat)
	for _, label := range []string{LabelProductsAvailable, LabelTariffs} {
		if strings.Contains(out, label) {
			t.Errorf("online-shop profile on a salon-only KB rendered %q", label)
		}
	}
}

// A payment QUESTION is answered from the KB; only payment OPERATIONS escalate.
func TestTemplates_PaymentRuleIsNotBlanket(t *testing.T) {
	for _, id := range PromptTemplateIDs() {
		text, _ := DefaultTemplateInstructions(id)
		norm := squash(text)
		if strings.Contains(norm, "оплата, вопросы вне базы") {
			t.Errorf("%s carries the salon frame's blanket «оплата → unsupported» wording", id)
		}
		a := strings.Index(norm, "a. ВОПРОС об условиях")
		b := strings.Index(norm, "b. ДЕЙСТВИЕ с оплатой")
		if a < 0 || b < 0 || a > b {
			t.Fatalf("%s: payment rule 11 a/b missing", id)
		}
		if !strings.Contains(norm[a:b], "Это НЕ эскалация") {
			t.Errorf("%s: 11a must say a payment question is not an escalation", id)
		}
		if !strings.Contains(norm[b:], "всегда эскалируй") {
			t.Errorf("%s: 11b must escalate payment operations", id)
		}
	}
}

func TestRenderBusinessTerms_OnlyNonEmptyAndShared(t *testing.T) {
	kb := mixedKB()
	got := renderBusinessTerms(kb.PromptInput())
	for _, want := range []string{"- Адрес: г. Алматы", "- Обратный звонок: в рабочее время", "- Предоплата:", "- Рассрочка:", "- Гарантия:"} {
		if !strings.Contains(got, want) {
			t.Errorf("business terms lack %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Реквизиты") {
		t.Error("an empty field must be omitted")
	}
	if got := renderBusinessTerms(&PromptInput{}); got != "—" {
		t.Errorf("empty terms = %q, want —", got)
	}
}

// The new structural slots must not disturb any pinned frame: renderServices
// (salon-kb@v1) keeps its behavior sentence, renderServiceCatalog drops it.
func TestRenderServiceCatalog_BareAttributeOnly(t *testing.T) {
	kb := salonKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	legacy := renderServices(kb.PromptInput(), cat)
	fresh := renderServiceCatalog(kb.PromptInput(), cat)
	if !strings.Contains(legacy, "decline and offer the base service instead") {
		t.Fatal("the pinned salon rendering lost its add-on sentence")
	}
	if strings.Contains(fresh, "decline") || !strings.Contains(fresh, "standalone: forbidden\n") {
		t.Errorf("service catalog must carry the bare attribute only:\n%s", fresh)
	}
	if strings.Replace(legacy, standaloneLineWithBehavior, standaloneLineAttribute, -1) != fresh {
		t.Error("service catalog must differ from the legacy rendering ONLY in the standalone line")
	}
}
