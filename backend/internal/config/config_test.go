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

func TestTelegramResolvedWebhookSecret(t *testing.T) {
	cases := []struct {
		name                  string
		telegramWebhookSecret string
		webhookToken          string
		want                  string
	}{
		{"both set: TG_WEBHOOK_SECRET wins", "tg-secret", "shared-token", "tg-secret"},
		{"only WEBHOOK_TOKEN set: falls back", "", "shared-token", "shared-token"},
		{"neither set: empty", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{TelegramWebhookSecret: tc.telegramWebhookSecret, WebhookToken: tc.webhookToken}
			if got := c.TelegramResolvedWebhookSecret(); got != tc.want {
				t.Errorf("TelegramResolvedWebhookSecret() = %q, want %q", got, tc.want)
			}
		})
	}
}
