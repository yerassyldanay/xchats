package config

import "testing"

func TestAccountIDDeterministic(t *testing.T) {
	a := AccountID("77011111111@s.whatsapp.net")
	b := AccountID(" 77011111111@S.WhatsApp.Net ") // different case/spacing, same number
	if a != b {
		t.Fatalf("account id not stable across canonicalization: %s vs %s", a, b)
	}
	c := AccountID("77000000000@s.whatsapp.net") // a different number (the customer)
	if a == c {
		t.Fatalf("different numbers collided on the same account id")
	}
}

func TestCanonicalAndPhone(t *testing.T) {
	if got := CanonicalJID("77011111111"); got != "77011111111@s.whatsapp.net" {
		t.Errorf("bare phone not coerced: %q", got)
	}
	if got := PhoneFromJID("77000000000@s.whatsapp.net"); got != "77000000000" {
		t.Errorf("phone extraction: %q", got)
	}
}
