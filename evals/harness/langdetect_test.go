package main

import "testing"

// TestDetectLang_DottedIKazakhStillRoutesKazakh guards a case the OLD hand-rolled
// letter-list detector originally got wrong until і (U+0456) was added to its list: Kazakh
// written with і as its only visually-distinctive letter — no ә/ғ/қ/ң/ө/ұ/ү/һ anywhere.
// lingua-go's statistical model has no letter list to maintain, but this behavior is worth
// pinning explicitly anyway — it is exactly the kind of case a routing regression would
// silently reintroduce.
func TestDetectLang_DottedIKazakhStillRoutesKazakh(t *testing.T) {
	if got := detectLang("Сіздерде кофемашина бар ма?", nil); got != "kk" {
		t.Errorf("detectLang(і-only Kazakh question) = %q, want kk", got)
	}
}

// TestDetectLang_AgainstTestBankMessages calibrates detectLang against the ACTUAL messages
// in the harness's own test banks — not invented examples — so a future edit to those
// banks that breaks the router's assumptions fails loudly here, not silently at eval time.
func TestDetectLang_AgainstTestBankMessages(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		// Latin brand name inside an otherwise-Kazakh sentence must not force "ru".
		{"kk price question with Latin brand", "Кофемашина DeLonghi қанша тұрады?", "kk"},
		{"kk delivery question", "Жеткізу қанша тұрады және қанша күнде жетеді?", "kk"},
		// Genuinely mixed message: a Kazakh greeting, independent Russian clauses
		// ("Скажите", "пожалуйста"), and a KAZAKH question clause. The dominant-language
		// rule routes by the language the question itself is asked in.
		{"genuinely mixed message, question clause is Kazakh", "Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?", "kk"},
		// The symmetric control: Kazakh greeting, RUSSIAN question — dominant language ru.
		{"genuinely mixed message, question clause is Russian", "Сәлеметсіз! Сколько стоит кофемашина?", "ru"},
		// Mixed with NO question clause at all — statement dominance decides.
		{"mixed without a question, Russian statement", "Сәлеметсіз бе. Хочу заказать кофемашину, привезите завтра вечером.", "ru"},
		// Two question clauses in different languages — the LAST one wins (it is the
		// question the reply will actually answer).
		{"two questions, last one Kazakh", "Сколько стоит кофемашина? Жеткізу қанша тұрады?", "kk"},
		{"two questions, last one Russian", "Жеткізу қанша тұрады? Сколько стоит доставка кофемашины?", "ru"},
		{"mixed without a question stays ru", "Сәлеметсіз бе. Хочу кофемашину домой.", "ru"},
		// Plain Russian messages from the bank.
		{"ru stock question", "Набор посуды есть в наличии?", "ru"},
		{"ru photo request", "Пришлите фото кофемашины, пожалуйста", "ru"},
		{"ru certificate question", "А сертификат качества на кофемашину есть? Пришлите.", "ru"},
		{"ru delivery zones question", "Куда вы вообще доставляете, есть карта зон?", "ru"},
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

// TestDetectLang_SharedAlphabetKazakh is the core proof of the 2026-07-27 decision:
// Kazakh must be recognized by its words and grammar, even when the customer types it
// using only the Russian keyboard (no ә/ғ/қ/ң/ө/ұ/ү/һ/і anywhere) — a routine occurrence,
// not an edge case. Covers the five input classes the decision calls out explicitly:
// fully Russian, fully Kazakh (native orthography), shared-alphabet Kazakh, mixed
// Kazakh/Russian clauses in one message, and code-switching within a single clause.
// Negative controls are messages containing a token that reads as a Kazakh word ONLY as
// part of a longer Kazakh construction ("бар" the particle vs. "бар" the Russian noun
// "bar") — these must stay "ru" on their own.
func TestDetectLang_SharedAlphabetKazakh(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		// Shared-alphabet Kazakh — the user's own worked examples (2026-07-27 design
		// note), adapted to nothing (verbatim): a customer asking about stock and intent
		// to buy, entirely in Kazakh grammar, zero Kazakh-only letters.
		{"shared-alphabet: stock question + purchase intent", "Сиздерде Филипс блендери бар ма? Билейин деген едим.", "kk"},
		{"shared-alphabet: price question", "Кофемашина DeLonghi канша турады?", "kk"},
		// NOTE: a bare "Бар ма?" is deliberately NOT a case here — at only 5 Cyrillic
		// letters it falls under shortMessageCyrillicThreshold and defers to
		// conversation history instead of a direct read; see TestDetectLang_HistoryFallback.
		// Mixed RU greeting + shared-alphabet KK question clause: the whole-string
		// confidence value alone (0.673 Russian) would get this WRONG — only clause
		// segmentation (see classifyText's doc comment) resolves it correctly.
		{"mixed RU greeting + shared-alphabet KK question", "Здравствуйте! Астанага жеткизу канша турады?", "kk"},
		// Code-switching within a single message (Kazakh greeting + Russian verb +
		// Kazakh noun phrase) — still dominated by the Kazakh question content.
		{"code-switching within one message", "Сәлеметсіз бе! Подскажите бағасын кофемашины DeLonghi", "kk"},
		// Native Kazakh orthography, for completeness (not the hard case, but must keep
		// working under the new detector too).
		{"native orthography Kazakh", "Астанаға жеткізу қанша тұрады?", "kk"},
		{"native orthography with і only", "Сәлеметсіз, бе?", "kk"},

		// Negative controls: a shared-alphabet token that is a genuine, common RUSSIAN
		// word must NOT be read as the Kazakh particle/word that happens to share its
		// spelling.
		{"ru: бар as the Russian noun \"bar\", not the kk particle", "Где ближайший бар?", "ru"},
		{"ru: бар as the Russian noun, second phrasing", "Сколько стоит бар?", "ru"},
		{"ru: едим (\"we eat\"), not a form of Kazakh білу/едім", "Мы едим дома", "ru"},
		{"ru: ба as an interjection, not a question particle", "Ба! Какие люди!", "ru"},
		{"ru: unrelated question", "Открыто ли кафе?", "ru"},
		{"ru: plain request", "Пришлите фото, пожалуйста", "ru"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectLang(tc.message, nil); got != tc.want {
				t.Errorf("detectLang(%q) = %q, want %q", tc.message, got, tc.want)
			}
		})
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
		// letters means cyrillicLetterCount is 0, which detectLang's `n > 0` guard
		// deliberately excludes — a message with NO Cyrillic content at all isn't "a
		// short Cyrillic message", it's a different case entirely, and must keep
		// defaulting to "ru" regardless of history.
		history := []HistoryTurn{{Role: "client", Text: "Жеткізу қанша тұрады?"}}
		if got := detectLang(tc.message, history); got != tc.want {
			t.Errorf("detectLang(%q, kk history) = %q, want %q (zero-Cyrillic messages must not consult history)", tc.message, got, tc.want)
		}
	}
}

// TestDetectLang_HistoryFallback is the core proof for the Бар ма?-after-a-Kazakh-
// conversation case: a short message (at or under shortMessageCyrillicThreshold) is too
// thin a sample for the statistical model alone — lingua-go's OWN opinion of "Бар ма?" in
// isolation is irrelevant here, the length gate defers to history unconditionally — so it
// falls back to the most recent client history turn instead, and, symmetrically, a recent
// Russian turn confirms "ru" rather than leaving it to chance.
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
		{"short message, no history -> unchanged ru default", "Бар ма?", nil, "ru"},
		{"short message, prior Kazakh client turn -> kk", "Бар ма?", kkHistory, "kk"},
		{"short message, prior Russian client turn -> ru", "Бар ма?", ruHistory, "ru"},
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
		{"a history turn that's itself too short to read is not usable -> unchanged ru default",
			"Бар ма?", []HistoryTurn{{Role: "client", Text: "Ок."}}, "ru"},
		{"confidently-Kazakh current message ignores disagreeing ru history",
			"Жеткізу қанша тұрады?", ruHistory, "kk"},
		{"long confident-Russian current message ignores disagreeing kk history",
			"Здравствуйте, подскажите пожалуйста стоимость доставки по городу.", kkHistory, "ru"},
		{"genuinely mixed current message resolves by its own question clause, history ignored",
			"Сәлеметсіз бе! Скажите, пожалуйста, кофемашина DeLonghi қанша тұрады?", kkHistory, "kk"},
		{"genuinely mixed current message with a Russian question ignores disagreeing kk history",
			"Сәлеметсіз! Сколько стоит доставка кофемашины по городу?", kkHistory, "ru"},
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
