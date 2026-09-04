package response

import (
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// TestFrameForChannel pins the mapping GenerateRequest.Channel now drives.
// The WhatsApp and simulator rows are the important ones: they must stay on the
// same byte-identical frame, so adding Telegram cannot move the prompt the
// schema_kb_v1 eval grades. Every row moved v5 -> v6 in 2026-09 (v5 could not
// render virtual fact columns at all); the pairing that actually matters is
// that frameFor and PromptRefFor never disagree, since a draft stamped with a
// ref whose frame did not produce it is unreproducible.
func TestFrameForChannel(t *testing.T) {
	cases := []struct {
		channel   messaging.Channel
		want      string
		wantRef   string
		wantLabel string
	}{
		{messaging.ChannelWhatsApp, aiprompt.FrameShopKBV6RU(), aiprompt.PromptRefShopKBV6, "whatsapp"},
		{messaging.ChannelSimulator, aiprompt.FrameShopKBV6RU(), aiprompt.PromptRefShopKBV6, "simulator"},
		{messaging.ChannelTelegram, aiprompt.FrameShopKBV6TGRU(), aiprompt.PromptRefShopKBV6TG, "telegram"},
		{messaging.Channel(""), aiprompt.FrameShopKBV6RU(), aiprompt.PromptRefShopKBV6, "unset"},
		{messaging.Channel("something-else"), aiprompt.FrameShopKBV6RU(), aiprompt.PromptRefShopKBV6, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.wantLabel, func(t *testing.T) {
			if got := frameFor(tc.channel); got != tc.want {
				t.Fatalf("frameFor(%q) picked the wrong frame", tc.channel)
			}
			if got := PromptRefFor(tc.channel); got != tc.wantRef {
				t.Fatalf("PromptRefFor(%q) = %q, want %q", tc.channel, got, tc.wantRef)
			}
		})
	}
}

func TestTelegramFrameIsADistinctFrame(t *testing.T) {
	if aiprompt.FrameShopKBV6TGRU() == aiprompt.FrameShopKBV6RU() {
		t.Fatal("the Telegram frame is identical to the WhatsApp one — the persona line was not neutralized")
	}
}

// TestFrameForChannel_ServesTariffCapableFrame is the regression guard for the
// bug v5 exists to fix: whatever frame a channel gets, it must be able to carry
// tariffs. v6 carries tariffs through SlotTariffCatalog rather than v5's
// SlotTariffs (see prompt.go's slot doc comment) — a future frame bump that
// drops it would put every tariff back out of the model's reach without
// failing any other test here.
func TestFrameForChannel_ServesTariffCapableFrame(t *testing.T) {
	for _, ch := range []messaging.Channel{
		messaging.ChannelWhatsApp, messaging.ChannelSimulator, messaging.ChannelTelegram, messaging.Channel(""),
	} {
		if !strings.Contains(frameFor(ch), aiprompt.SlotTariffCatalog) {
			t.Errorf("frameFor(%q) returned a frame with no %s slot — tariffs would be invisible to the model", ch, aiprompt.SlotTariffCatalog)
		}
	}
}
