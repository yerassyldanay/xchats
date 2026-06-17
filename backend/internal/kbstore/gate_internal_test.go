package kbstore

import (
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
)

func gateReasons(topicBody string, values ...domain.Value) []string {
	snap := &domain.Snapshot{
		Values: domain.NewValueBook(values...),
		Topics: []domain.Topic{{Slug: "t", Language: "ru", BodyMD: topicBody}},
	}
	return gate(snap, 0, nil)
}

// A tokenized price body passes: the amount lives in ai_values, the body holds a token.
func TestGate_TokenizedPriceOK(t *testing.T) {
	r := gateReasons("Цена — {{price.x}} в месяц.",
		domain.Value{Token: "price.x", Lang: "ru", Text: "25 000 ₸/мес"})
	if len(r) != 0 {
		t.Fatalf("tokenized body should pass, got %v", r)
	}
}

// A literal currency amount in a body is blocked (unconfirmed number → customer).
func TestGate_LiteralCurrencyBlocked(t *testing.T) {
	for _, body := range []string{"Цена 25 000 ₸ в месяц.", "Стоит 9900тг.", "Now $50 only."} {
		r := gateReasons(body)
		if len(r) == 0 || !strings.Contains(strings.Join(r, "; "), "literal amount") {
			t.Fatalf("body %q should be blocked for a literal amount, got %v", body, r)
		}
	}
}

// Step numbers and bare counts are not currency and must pass (seed how_to_order).
func TestGate_StepNumbersOK(t *testing.T) {
	r := gateReasons("Оформить заказ: 1) выбрать 2) адрес 3) подтвердить — 2 дня на доставку.")
	if len(r) != 0 {
		t.Fatalf("step numbers / bare counts should pass, got %v", r)
	}
}
