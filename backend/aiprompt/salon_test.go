package aiprompt

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func intPtr(n int) *int { return &n }

// salonFrame is a minimal synthetic frame exercising every salon-kb@v1 slot
// once — the counterpart to helpers_test.go's basicFrame/v2Frame, used to
// unit-test renderServices/renderSpecialists in isolation from the much
// longer real embedded frame text.
const salonFrame = `# ASSISTANT
%%ASSISTANT%%

# SERVICES
%%SERVICES%%

# SPECIALISTS
%%SPECIALISTS%%

# TOPICS
%%TOPICS%%

# BUSINESS FACTS
%%BUSINESS_FACTS%%

# RESPONSE SCHEMA
%%RESPONSE_SCHEMA%%
`

// salonKB returns a minimal-but-complete valid salon KB modeled on TEST.md's
// "Салон красоты «Аура»" golden seed data (simplified): two specialists with
// different, non-overlapping schedules and booking-link setups (one custom,
// one falling back to the salon), and a base/variant/addon hair-service
// hierarchy plus a standalone nail-service base. Tests copy this and mutate
// only the field under test, same discipline as baseKB/zonesKB.
func salonKB() *KB {
	return &KB{
		OrganizationID: testOrg,
		Assistant: &Assistant{
			Persona: "Ты — ассистент салона красоты «Аура».", Mission: "Помогай клиентам с услугами и мастерами.",
			Guardrails: "Никогда не подтверждай запись и не называй время свободным.",
			LanguagePolicy: "Отвечай на языке клиента (ru/kk).", ReplyMaxWords: 120,
		},
		Contacts: &Contacts{
			Phone: "+7 707 890 12 34", Address: "г. Алматы, пр. Достык, 128", WorkingHours: "Пн-Вс: 10:00 - 21:00",
			BookingURL: "https://xpayment.kz/book/aura-almaty",
			Schedule:   Schedule{day(Monday, "10:00", "21:00"), day(Tuesday, "10:00", "21:00")},
		},
		Specialists: []Specialist{
			{
				Ref: "alina-kim", FullName: "Алина Ким", Title: "Топ-стилист / Колорист", Experience: "7 лет",
				Schedule: Schedule{
					day(Tuesday, "10:00", "19:00"), day(Wednesday, "10:00", "19:00"), day(Thursday, "10:00", "19:00"),
					day(Friday, "10:00", "19:00"), day(Saturday, "10:00", "19:00"),
				},
				BookingURL:      "https://xpayment.kz/book/aura-alina",
				PortfolioImages: []string{"m-alina-1", "m-alina-2"},
				SalesStatus:     "active",
			},
			{
				Ref: "diana-nur", FullName: "Диана Нур", Title: "Мастер ногтевого сервиса", Experience: "4 года",
				Schedule: Schedule{
					day(Monday, "11:00", "20:00"), day(Wednesday, "11:00", "20:00"),
					day(Friday, "11:00", "20:00"), day(Sunday, "11:00", "20:00"),
				},
				BookingURL:  "", // blank: falls back to Contacts.BookingURL
				SalesStatus: "active",
			},
			{
				Ref: "archived-master", FullName: "Бывший мастер", SalesStatus: "inactive",
				Schedule: Schedule{day(Monday, "10:00", "18:00")}, BookingURL: "https://example.com/archived",
			},
		},
		Services: []Service{
			{
				Ref: "haircut-women", ServiceType: "base", Category: "Волосы", Name: "Женская стрижка",
				Price: "10 000 ₸", Duration: intPtr(60), Description: "Мытьё, стрижка, укладка.",
				SpecialistRefs: []string{"alina-kim"}, SalesStatus: "active",
			},
			{
				Ref: "haircut-short", ParentRef: "haircut-women", ServiceType: "variant", Category: "Волосы",
				Name: "Короткая стрижка", Price: "8 000 ₸", Duration: intPtr(45),
				SpecialistRefs: []string{"alina-kim"}, SalesStatus: "active",
			},
			{
				Ref: "hair-spa-mask", ParentRef: "haircut-women", ServiceType: "addon", Category: "Волосы",
				Name: "Спа-уход", Price: "5 000 ₸", Duration: intPtr(30), SalesStatus: "active",
			},
			{
				Ref: "manicure-gel", ServiceType: "base", Category: "Ногти", Name: "Комбинированный маникюр",
				Price: "9 000 ₸", Duration: intPtr(90), SpecialistRefs: []string{"diana-nur"}, SalesStatus: "active",
			},
		},
		Materials: []Material{testMaterial("m-alina-1"), testMaterial("m-alina-2")},
	}
}

func TestFrameSalonKBV1RU_MatchesPinnedHash(t *testing.T) {
	const want = "a8c814735a8b3a295a2a26a7d9de44999bf23d4a1f68e7fe77ce87b583168675"
	sum := sha256.Sum256([]byte(FrameSalonKBV1RU()))
	got := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("frames/salon-kb-v1-ru.txt sha256 = %s, want %s (the embedded frame changed — see frame.go's doc comment: cut a v2 instead of editing in place)", got, want)
	}
}

func TestFrameSalonKBV1RU_ContainsExpectedSlotsOnly(t *testing.T) {
	frame := FrameSalonKBV1RU()
	for _, slot := range []string{SlotServices, SlotSpecialists, SlotAssistant, SlotTopics, SlotBusinessFacts, SlotResponseSchema} {
		if !strings.Contains(frame, slot) {
			t.Errorf("want frame to contain slot %s", slot)
		}
	}
	// The salon frame is a focused, different content model from shop-kb — it
	// must never carry the shop-only slots (PLAN.md: it replaces, not
	// extends, the products/tariffs/zones frame).
	for _, slot := range []string{SlotProductsAvailable, SlotTariffCatalog, SlotDeliveryZones} {
		if strings.Contains(frame, slot) {
			t.Errorf("salon frame must not contain shop-only slot %s", slot)
		}
	}
}

func TestBuildCatalog_ServiceHierarchy(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(kb *KB)
		wantErr bool
	}{
		{"valid hierarchy", func(kb *KB) {}, false},
		{"variant without parent_ref", func(kb *KB) {
			kb.Services[1].ParentRef = ""
		}, true},
		{"addon without parent_ref", func(kb *KB) {
			kb.Services[2].ParentRef = ""
		}, true},
		{"unknown parent_ref", func(kb *KB) {
			kb.Services[1].ParentRef = "does-not-exist"
		}, true},
		{"parent_ref points at a variant, not a base (two hierarchy levels)", func(kb *KB) {
			kb.Services[2].ParentRef = "haircut-short"
		}, true},
		{"base service must not have a parent_ref", func(kb *KB) {
			kb.Services[0].ParentRef = "manicure-gel"
		}, true},
		{"invalid service_type", func(kb *KB) {
			kb.Services[0].ServiceType = "combo"
		}, true},
		{"duplicate service ref", func(kb *KB) {
			kb.Services = append(kb.Services, kb.Services[0])
		}, true},
		{"active child with archived base is inconsistent", func(kb *KB) {
			kb.Services[0].SalesStatus = "inactive" // haircut-women, base
			// haircut-short (variant) stays active -> invariant violated
		}, true},
		{"archiving base AND its children together is fine", func(kb *KB) {
			kb.Services[0].SalesStatus = "inactive"
			kb.Services[1].SalesStatus = "inactive"
			kb.Services[2].SalesStatus = "inactive"
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kb := salonKB()
			tc.mutate(kb)
			_, err := BuildCatalog(kb)
			if tc.wantErr && err == nil {
				t.Fatal("want an error, got none")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestBuildCatalog_ServiceDuration(t *testing.T) {
	t.Run("positive duration gets a token", func(t *testing.T) {
		cat, err := BuildCatalog(salonKB())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cat.FactByToken("{{service.haircut-women.duration}}") == nil {
			t.Fatal("want a duration token for a positive duration")
		}
	})
	t.Run("nil duration gets no token", func(t *testing.T) {
		kb := salonKB()
		kb.Services[0].Duration = nil
		cat, err := BuildCatalog(kb)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cat.FactByToken("{{service.haircut-women.duration}}") != nil {
			t.Fatal("want no duration token when Duration is nil")
		}
	})
	t.Run("zero/negative duration gets no token (write-time validation's job to reject it)", func(t *testing.T) {
		kb := salonKB()
		kb.Services[0].Duration = intPtr(0)
		cat, err := BuildCatalog(kb)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cat.FactByToken("{{service.haircut-women.duration}}") != nil {
			t.Fatal("want no duration token for a non-positive duration")
		}
	})
}

func TestBuildCatalog_SpecialistArchived(t *testing.T) {
	cat, err := BuildCatalog(salonKB())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, col := range []string{"booking", "schedule", "schedule_mon"} {
		if cat.FactByToken("{{specialist.archived-master." + col + "}}") != nil {
			t.Errorf("want no %s token for an archived specialist", col)
		}
	}
	if cat.MediaByToken("specialists.archived-master.portfolio") != nil {
		t.Error("want no media entry for an archived specialist")
	}
	for _, a := range cat.Absent {
		if a.Table == "specialists" && a.Ref == "archived-master" {
			t.Error("an archived specialist must not appear in Absent either — it is fully suppressed, not just media-less")
		}
	}
}

func TestBuildCatalog_BookingFallback(t *testing.T) {
	cat, err := BuildCatalog(salonKB())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := ResolveFact("{{specialist.alina-kim.booking}}", salonKB(), cat)
	if err != nil || got != "https://xpayment.kz/book/aura-alina" {
		t.Fatalf("alina-kim.booking = %q, %v — want her own link", got, err)
	}
	got, err = ResolveFact("{{specialist.diana-nur.booking}}", salonKB(), cat)
	if err != nil || got != "https://xpayment.kz/book/aura-almaty" {
		t.Fatalf("diana-nur.booking = %q, %v — want the salon fallback link", got, err)
	}

	t.Run("neither link set means no token at all", func(t *testing.T) {
		kb := salonKB()
		kb.Specialists[1].BookingURL = ""
		kb.Contacts.BookingURL = ""
		cat, err := BuildCatalog(kb)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cat.FactByToken("{{specialist.diana-nur.booking}}") != nil {
			t.Fatal("want no booking token when neither the specialist nor the salon has a link")
		}
	})
}

func TestBuildCatalog_ScheduleTokens_OnlyScheduledDaysGetDailyTokens(t *testing.T) {
	cat, err := BuildCatalog(salonKB())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// alina-kim works tue-sat.
	for _, ref := range []string{"tue", "wed", "thu", "fri", "sat"} {
		if cat.FactByToken("{{specialist.alina-kim.schedule_" + ref + "}}") == nil {
			t.Errorf("want a schedule_%s token for a day alina-kim works", ref)
		}
	}
	for _, ref := range []string{"mon", "sun"} {
		if cat.FactByToken("{{specialist.alina-kim.schedule_" + ref + "}}") != nil {
			t.Errorf("want NO schedule_%s token for a day alina-kim does not work", ref)
		}
	}
	if cat.FactByToken("{{specialist.alina-kim.schedule}}") == nil {
		t.Error("want a full-week schedule token whenever any day is scheduled")
	}
}

func TestResolveFactLang_ScheduleWording(t *testing.T) {
	kb := salonKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := ResolveFactLang("{{specialist.alina-kim.schedule_tue}}", kb, cat, "ru")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Вторник: 10:00–19:00" {
		t.Fatalf("got %q", got)
	}
	gotKK, err := ResolveFactLang("{{specialist.alina-kim.schedule_tue}}", kb, cat, "kk")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotKK == "" || gotKK == got {
		t.Fatalf("want a distinct Kazakh rendering, got %q", gotKK)
	}
}

// TestResolveFactLang_ScheduleStaleRejection mirrors contract_test.go's
// existing "stale inactive row"/"stale empty value" pattern (BuildCatalog
// once, mutate kb in place, then resolve): a schedule_<day> token must fail
// closed if that weekday is no longer in the specialist's CURRENT schedule
// by the time of substitution — PLAN.md: "reject stale schedule tokens when
// the normalized schedule changed after prompt construction."
func TestResolveFactLang_ScheduleStaleRejection(t *testing.T) {
	kb := salonKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// alina-kim stops working Tuesdays after the catalog/prompt was built.
	kb.Specialists[0].Schedule = Schedule{day(Wednesday, "10:00", "19:00")}
	if _, err := ResolveFactLang("{{specialist.alina-kim.schedule_tue}}", kb, cat, "ru"); err == nil {
		t.Fatal("want a stale-token error once Tuesday is no longer scheduled")
	}
	// The full-week token and a still-valid day both keep resolving fine —
	// only the specific stale claim fails.
	if _, err := ResolveFactLang("{{specialist.alina-kim.schedule_wed}}", kb, cat, "ru"); err != nil {
		t.Fatalf("schedule_wed should still resolve: %v", err)
	}
	if _, err := ResolveFactLang("{{specialist.alina-kim.schedule}}", kb, cat, "ru"); err != nil {
		t.Fatalf("the full schedule token should still resolve: %v", err)
	}
}

func TestValidateResponseV7_StaleScheduleTokenIsAContractIssue(t *testing.T) {
	kb := salonKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	kb.Specialists[0].Schedule = Schedule{day(Wednesday, "10:00", "19:00")}
	raw := `{"reply_text":"Алина работает во вторник: {{specialist.alina-kim.schedule_tue}}. Ссылка: {{specialist.alina-kim.booking}}","reply_language":"ru","media_files_to_send":[],"escalate":false}`
	_, issues := ValidateResponseV7(raw, kb, cat)
	if !containsCode(issues, "stale_fact_placeholder") {
		t.Fatalf("want stale_fact_placeholder, got %v", issueCodes(issues))
	}
}

// TestValidateScheduleLiteralContract covers PLAN.md's literal-leak
// extension: the model must never write a raw clock time itself, even
// paraphrased, when the organization has salon schedule tokens at all — but
// a non-salon organization's existing behavior must never change (the
// regression TEST.md's Verification Matrix calls for).
func TestValidateScheduleLiteralContract(t *testing.T) {
	t.Run("model-authored time outside any token is flagged", func(t *testing.T) {
		kb := salonKB()
		cat, err := BuildCatalog(kb)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		raw := `{"reply_text":"Алина работает с 10:00 до 19:00 во вторник.","reply_language":"ru","media_files_to_send":[],"escalate":false}`
		_, issues := ValidateResponseV7(raw, kb, cat)
		if !containsCode(issues, "schedule_time_literal") {
			t.Fatalf("want schedule_time_literal, got %v", issueCodes(issues))
		}
	})

	t.Run("using only tokens raises no schedule_time_literal issue", func(t *testing.T) {
		kb := salonKB()
		cat, err := BuildCatalog(kb)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		raw := `{"reply_text":"Алина работает во вторник: {{specialist.alina-kim.schedule_tue}}. Запись: {{specialist.alina-kim.booking}}","reply_language":"ru","media_files_to_send":[],"escalate":false}`
		_, issues := ValidateResponseV7(raw, kb, cat)
		if containsCode(issues, "schedule_time_literal") {
			t.Fatalf("did not want schedule_time_literal, got %v", issueCodes(issues))
		}
	})

	t.Run("non-salon organizations are never affected (regression guard)", func(t *testing.T) {
		kb := baseKB() // the plain shop KB fixture — no specialists/services at all
		cat, err := BuildCatalog(kb)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		raw := `{"reply_text":"Работаем с 9:00 до 18:00 ежедневно.","reply_language":"ru","media_files_to_send":[],"escalate":false}`
		_, issues := ValidateResponseV7(raw, kb, cat)
		if containsCode(issues, "schedule_time_literal") {
			t.Fatalf("a non-salon organization must never trigger schedule_time_literal, got %v", issueCodes(issues))
		}
	})
}

// TestSalonPromptNoRawLeaks is the TEST.md-named regression guard for
// "TestSalonPromptNoRawLeaks": raw prices, booking URLs, and material
// storage identifiers must never appear in the rendered PRE-INFERENCE
// prompt (only their {{token}}/media-ref line may). Raw shift/break clock
// times are the one deliberate exception — schedule.go's doc comment and
// prompt.go's SlotServices/SlotSpecialists doc comment explain why the
// model must see them (to reason about an arbitrary customer-requested
// interval) and how the customer-facing leak is prevented at the RESPONSE
// boundary instead (validateScheduleLiteralContract, exercised above) —
// there is no way to keep the model's shift/break math correct while also
// hiding those numbers from its own input, so this test asserts the
// boundary that is actually enforceable: the reasoning block's numbers stay
// confined to the prompt, and separately (above) can never leak into a
// validated customer reply.
func TestSalonPromptNoRawLeaks(t *testing.T) {
	kb := salonKB()
	prompt, _, err := BuildPromptV7(FrameSalonKBV1RU(), kb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	forbidden := []string{
		"10 000 ₸", "8 000 ₸", "5 000 ₸", "9 000 ₸", // raw service prices
		"https://xpayment.kz/book/aura-alina", "https://xpayment.kz/book/aura-almaty", // raw booking URLs
		"m-alina-1", "m-alina-2", // raw material ids
	}
	for _, s := range forbidden {
		if strings.Contains(prompt, s) {
			t.Errorf("prompt leaks raw value %q pre-inference", s)
		}
	}
	// The deliberate exception: raw shift boundaries DO appear, inside the
	// machine-readable reasoning block only.
	if !strings.Contains(prompt, "tue: 10:00-19:00") {
		t.Error("want the schedule_reasoning block to carry alina-kim's raw Tuesday boundaries for the model to reason over")
	}
}

func TestRenderServices_TreeAndAddonInstruction(t *testing.T) {
	kb := salonKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := renderServices(kb.PromptInput(), cat)

	for _, want := range []string{
		"category: Волосы",
		"service: haircut-women",
		"type: base",
		"service: haircut-short",
		"parent_ref: haircut-women",
		"type: variant",
		"service: hair-spa-mask",
		"type: addon",
		"standalone: forbidden",
		"price_placeholder: {{service.haircut-women.price}}",
		"duration_placeholder: {{service.haircut-women.duration}}",
		"category: Ногти",
		"service: manicure-gel",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("renderServices output missing %q\n---\n%s", want, out)
		}
	}
	// haircut-women's block must come immediately before its own children,
	// nesting the variant/addon under their base within the same category.
	womenIdx := strings.Index(out, "service: haircut-women")
	shortIdx := strings.Index(out, "service: haircut-short")
	maskIdx := strings.Index(out, "service: hair-spa-mask")
	nailsIdx := strings.Index(out, "category: Ногти")
	if !(womenIdx < shortIdx && shortIdx < maskIdx && maskIdx < nailsIdx) {
		t.Errorf("want haircut-women's children nested directly beneath it, before the next category; got order: women=%d short=%d mask=%d nails=%d", womenIdx, shortIdx, maskIdx, nailsIdx)
	}
}

func TestRenderSpecialists_RosterAndReasoning(t *testing.T) {
	kb := salonKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := renderSpecialists(kb.PromptInput(), cat)

	for _, want := range []string{
		"specialist: alina-kim",
		"full_name: Алина Ким",
		"title: Топ-стилист / Колорист",
		"experience: 7 лет",
		"services: haircut-women, haircut-short",
		"schedule_reasoning:",
		"mon: off",
		"tue: 10:00-19:00",
		"schedule_placeholder: {{specialist.alina-kim.schedule}}",
		"schedule_tue_placeholder: {{specialist.alina-kim.schedule_tue}}",
		"booking_placeholder: {{specialist.alina-kim.booking}}",
		"portfolio_ref: specialists.alina-kim.portfolio",
		"specialist: diana-nur",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("renderSpecialists output missing %q\n---\n%s", want, out)
		}
	}
	if strings.Contains(out, "archived-master") {
		t.Error("an archived specialist must not appear in the roster at all")
	}
	// diana-nur has no booking_url of her own; her block must still carry a
	// booking_placeholder (resolved via the salon fallback at BuildCatalog
	// time) rather than omitting it.
	dianaBlock := out[strings.Index(out, "specialist: diana-nur"):]
	if !strings.Contains(dianaBlock, "booking_placeholder: {{specialist.diana-nur.booking}}") {
		t.Error("want diana-nur's block to carry a booking_placeholder via the salon fallback")
	}
}

func TestBuildPromptV7_Salon_EndToEnd(t *testing.T) {
	kb := salonKB()
	prompt, cat, err := BuildPromptV7(FrameSalonKBV1RU(), kb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidatePrompt(prompt, cat); err != nil {
		t.Fatalf("ValidatePrompt: %v", err)
	}
	if err := ValidateNoStorageLocatorLeak(prompt); err != nil {
		t.Fatalf("ValidateNoStorageLocatorLeak: %v", err)
	}
	if strings.Contains(prompt, "%%") {
		t.Error("want no leftover %%SLOT%% markers")
	}
}
