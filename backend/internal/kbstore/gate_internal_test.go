package kbstore

import (
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
)

func gateReasons(topicBody string) []string {
	snap := &domain.Snapshot{
		Topics: []domain.Topic{{Slug: "t", Language: "ru", BodyMD: topicBody}},
	}
	return gate(snap, 0)
}

// A pure-prose body passes: facts live in typed columns, quoted only in replies.
func TestGate_PureProseOK(t *testing.T) {
	r := gateReasons("Тариф покрывает основные нужды и включает поддержку.")
	if len(r) != 0 {
		t.Fatalf("pure-prose body should pass, got %v", r)
	}
}

// A fact token in a body is now BLOCKED — bodies must be pure prose (14 D3): a
// token in stored knowledge means a value is living where only prose belongs.
func TestGate_TokenInBodyBlocked(t *testing.T) {
	r := gateReasons("Цена — {{tariff.x.price}} в месяц.")
	if len(r) == 0 || !strings.Contains(strings.Join(r, "; "), "pure prose") {
		t.Fatalf("token-in-body should be blocked, got %v", r)
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
