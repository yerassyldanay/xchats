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
// schema_kb_v1 eval grades. Every row moved v4 -> v5 in 2026-08 (v4 could not
// render tariffs at all); the pairing that actually matters is that frameFor
// and PromptRefFor never disagree, since a draft stamped with a ref whose frame
// did not produce it is unreproducible.
func TestFrameForChannel(t *testing.T) {
	cases := []struct {
		channel   messaging.Channel
		want      string
		wantRef   string
		wantLabel string
	}{
		{messaging.ChannelWhatsApp, aiprompt.FrameShopKBV5RU(), aiprompt.PromptRefShopKBV5, "whatsapp"},
		{messaging.ChannelSimulator, aiprompt.FrameShopKBV5RU(), aiprompt.PromptRefShopKBV5, "simulator"},
		{messaging.ChannelTelegram, aiprompt.FrameShopKBV5TGRU(), aiprompt.PromptRefShopKBV5TG, "telegram"},
		{messaging.Channel(""), aiprompt.FrameShopKBV5RU(), aiprompt.PromptRefShopKBV5, "unset"},
		{messaging.Channel("something-else"), aiprompt.FrameShopKBV5RU(), aiprompt.PromptRefShopKBV5, "unknown"},
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
	if aiprompt.FrameShopKBV5TGRU() == aiprompt.FrameShopKBV5RU() {
		t.Fatal("the Telegram frame is identical to the WhatsApp one — the persona line was not neutralized")
	}
}

// TestFrameForChannel_ServesTariffCapableFrame is the regression guard for the
// bug v5 exists to fix: whatever frame a channel gets, it must be able to carry
// tariffs. A future frame bump that drops the slot would put every tariff back
// out of the model's reach without failing any other test here.
func TestFrameForChannel_ServesTariffCapableFrame(t *testing.T) {
	for _, ch := range []messaging.Channel{
		messaging.ChannelWhatsApp, messaging.ChannelSimulator, messaging.ChannelTelegram, messaging.Channel(""),
	} {
		if !strings.Contains(frameFor(ch), aiprompt.SlotTariffs) {
			t.Errorf("frameFor(%q) returned a frame with no %s slot — tariffs would be invisible to the model", ch, aiprompt.SlotTariffs)
		}
	}
}
