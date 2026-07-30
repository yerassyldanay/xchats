package response

import (
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// TestFrameForChannel pins the mapping GenerateRequest.Channel now drives.
// The WhatsApp and simulator rows are the important ones: they must stay on the
// byte-identical evaluated frame, so adding Telegram cannot move the prompt the
// schema_kb_v1 eval graded.
func TestFrameForChannel(t *testing.T) {
	cases := []struct {
		channel   messaging.Channel
		want      string
		wantRef   string
		wantLabel string
	}{
		{messaging.ChannelWhatsApp, aiprompt.FrameShopKBV4RU(), aiprompt.PromptRefShopKBV4, "whatsapp"},
		{messaging.ChannelSimulator, aiprompt.FrameShopKBV4RU(), aiprompt.PromptRefShopKBV4, "simulator"},
		{messaging.ChannelTelegram, aiprompt.FrameShopKBV4TGRU(), aiprompt.PromptRefShopKBV4TG, "telegram"},
		{messaging.Channel(""), aiprompt.FrameShopKBV4RU(), aiprompt.PromptRefShopKBV4, "unset"},
		{messaging.Channel("something-else"), aiprompt.FrameShopKBV4RU(), aiprompt.PromptRefShopKBV4, "unknown"},
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
	if aiprompt.FrameShopKBV4TGRU() == aiprompt.FrameShopKBV4RU() {
		t.Fatal("the Telegram frame is identical to the WhatsApp one — the persona line was not neutralized")
	}
}
