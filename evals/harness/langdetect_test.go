package main

import "testing"

// TestDetectLang_AgainstTestBankMessages calibrates detectLang against the ACTUAL messages
// in common/shop-questions.yaml — not invented examples — so a future edit to that bank
// that breaks the router's assumptions fails loudly here, not silently at eval time.
func TestDetectLang_AgainstTestBankMessages(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		// "2. price question, Kazakh" — Latin brand name inside an otherwise-Kazakh
		// clause must not force "ru".
		{"kk price question with Latin brand", "Кофемашина DeLonghi қанша тұрады?", "kk"},
		// "3. delivery cost + time, Kazakh" — pure Kazakh, no Latin/Russian at all.
		{"kk delivery question", "Жеткізу қанша тұрады және қанша күнде жетеді?", "kk"},
		// "20. mixed Kazakh/Russian message, rule says answer Russian when mixed" — a
		// Kazakh greeting clause followed by independent Russian clauses ("Скажите",
		// "пожалуйста") must route "ru", the core case this heuristic exists for.
		{"genuinely mixed kk+ru message", "Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?", "ru"},
		// Plain Russian messages from the bank — zero Kazakh-only letters anywhere.
		{"ru stock question", "Набор посуды есть в наличии?", "ru"},
		{"ru photo request", "Пришлите фото кофемашины, пожалуйста", "ru"},
		{"ru certificate question", "А сертификат качества на кофемашину есть? Пришлите.", "ru"},
		{"ru delivery zones question", "Куда вы вообще доставляете, есть карта зон?", "ru"},
		{"ru greeting only", "Здравствуйте!", "ru"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// nil history: these cases are specifically about message-only
			// classification — see TestDetectLang_HistoryFallback for the fallback.
			if got := detectLang(tc.message, nil); got != tc.want {
				t.Errorf("detectLang(%q) = %q, want %q", tc.message, got, tc.want)
			}
		})
	}
}

// TestDetectLang_ShortSharedAlphabetParticleRidesAlong proves the calibration constant
// directly: a short shared-alphabet word (below minRussianClauseRunes) inside the SAME
// clause as a Kazakh-only letter must not, on its own, create a competing "Russian" signal.
func TestDetectLang_ShortSharedAlphabetParticleRidesAlong(t *testing.T) {
	// "бе" (2 letters) is the Kazakh interrogative particle riding inside "Сәлеметсіз бе" —
	// same clause (no separating punctuation before it), so it never reaches the
	// independent-clause check at all. Isolated here as a clause of its own instead, to
	// pin the minRussianClauseRunes threshold: 2 < 4, so a lone "бе" clause must not, by
	// itself, be enough to read as Russian.
	if got := detectLang("Сәлеметсіз, бе?", nil); got != "kk" {
		t.Errorf("detectLang with a short trailing particle = %q, want kk (particle too short to count as its own Russian clause)", got)
	}
}

func TestDetectLang_PureLatinOrEmptyDefaultsToRussian(t *testing.T) {
	tests := []struct{ message, want string }{
		{"DeLonghi?", "ru"},
		{"", "ru"},
		{"12345", "ru"},
	}
	for _, tc := range tests {
		// Confirms the default is unchanged EVEN with history present: zero Cyrillic
		// letters means cyrillicLetterCount is 0, which the fallback's own `n > 0` guard
		// deliberately excludes — a message with NO Cyrillic content at all isn't "a
		// short Cyrillic message", it's a different case entirely, and must keep
		// defaulting to "ru" exactly as before this fallback existed.
		history := []HistoryTurn{{Role: "client", Text: "Жеткізу қанша тұрады?"}}
		if got := detectLang(tc.message, history); got != tc.want {
			t.Errorf("detectLang(%q, kk history) = %q, want %q (zero-Cyrillic messages must not consult history)", tc.message, got, tc.want)
		}
	}
}

// TestDetectLang_HistoryFallback is the core proof for the Бар ма?-after-a-Kazakh-
// conversation case: a short, orthographically-ambiguous message alone routes "ru" by
// default (no signal), but a recent Kazakh customer turn should tip it to "kk" — and,
// symmetrically, a recent Russian turn should confirm "ru" rather than leaving it to
// chance.
func TestDetectLang_HistoryFallback(t *testing.T) {
	kkHistory := []HistoryTurn{
		{Role: "client", Text: "Кофемашина DeLonghi қанша тұрады?"},
		{Role: "assistant", Text: "Кофемашина {{product.coffee-machine.price}}."},
	}
	ruHistory := []HistoryTurn{
		{Role: "client", Text: "Сколько стоит кофемашина?"},
		{Role: "assistant", Text: "Кофемашина {{product.coffee-machine.price}}."},
	}

	tests := []struct {
		name    string
		message string
		history []HistoryTurn
		want    string
	}{
		{"short ambiguous message, no history -> unchanged ru default", "Бар ма?", nil, "ru"},
		{"short ambiguous message, prior Kazakh client turn -> kk", "Бар ма?", kkHistory, "kk"},
		{"short ambiguous message, prior Russian client turn -> ru", "Бар ма?", ruHistory, "ru"},
		{"short ambiguous message ignores an assistant-only turn (not role=client)",
			"Бар ма?", []HistoryTurn{{Role: "assistant", Text: "Жеткізу қанша тұрады?"}}, "ru"},
		{"short ambiguous message, most recent of TWO client turns wins (ru then kk)",
			"Бар ма?", []HistoryTurn{
				{Role: "client", Text: "Сколько стоит кофемашина?"},
				{Role: "assistant", Text: "129 900 ₸."},
				{Role: "client", Text: "Жеткізу қанша тұрады?"},
			}, "kk"},
		{"short ambiguous message, most recent of TWO client turns wins (kk then ru)",
			"Бар ма?", []HistoryTurn{
				{Role: "client", Text: "Жеткізу қанша тұрады?"},
				{Role: "assistant", Text: "{{policy.main.delivery_cost}}."},
				{Role: "client", Text: "Сколько стоит кофемашина?"},
			}, "ru"},
		{"a history turn that's itself ambiguous is not usable -> unchanged ru default",
			"Бар ма?", []HistoryTurn{{Role: "client", Text: "Ок."}}, "ru"},
		{"confidently-Kazakh current message ignores disagreeing ru history",
			"Жеткізу қанша тұрады?", ruHistory, "kk"},
		{"long confident-Russian current message ignores disagreeing kk history",
			"Здравствуйте, подскажите пожалуйста стоимость доставки по городу.", kkHistory, "ru"},
		{"genuinely mixed current message ignores kk history (still routes ru)",
			"Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?", kkHistory, "ru"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectLang(tc.message, tc.history); got != tc.want {
				t.Errorf("detectLang(%q, history=%v) = %q, want %q", tc.message, tc.history, got, tc.want)
			}
		})
	}
}

func TestCyrillicLetterCount(t *testing.T) {
	tests := []struct {
		message string
		want    int
	}{
		{"Бар ма?", 5},
		{"Рахмет", 6},
		{"Здравствуйте!", 12},
		{"DeLonghi?", 0},
		{"", 0},
	}
	for _, tc := range tests {
		if got := cyrillicLetterCount(tc.message); got != tc.want {
			t.Errorf("cyrillicLetterCount(%q) = %d, want %d", tc.message, got, tc.want)
		}
	}
}

func TestDetectLangFromHistory(t *testing.T) {
	if _, ok := detectLangFromHistory(nil); ok {
		t.Error("want ok=false for nil history")
	}
	if _, ok := detectLangFromHistory([]HistoryTurn{{Role: "assistant", Text: "Жеткізу қанша тұрады?"}}); ok {
		t.Error("want ok=false when the only turn is not role=client")
	}
	lang, ok := detectLangFromHistory([]HistoryTurn{{Role: "client", Text: "Жеткізу қанша тұрады?"}})
	if !ok || lang != "kk" {
		t.Errorf("want (kk, true), got (%q, %v)", lang, ok)
	}
	lang, ok = detectLangFromHistory([]HistoryTurn{{Role: "client", Text: "Сколько стоит кофемашина?"}})
	if !ok || lang != "ru" {
		t.Errorf("want (ru, true), got (%q, %v)", lang, ok)
	}
}
