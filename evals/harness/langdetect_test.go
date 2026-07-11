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
			if got := detectLang(tc.message); got != tc.want {
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
	if got := detectLang("Сәлеметсіз, бе?"); got != "kk" {
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
		if got := detectLang(tc.message); got != tc.want {
			t.Errorf("detectLang(%q) = %q, want %q", tc.message, got, tc.want)
		}
	}
}
