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
// schema_kb_v1 eval grades. Every row moved v6 -> v7 in 2026-09 (v6 had no
// structured way for an escalation to report why — see aiprompt.frameFor's
// doc comment); the pairing that actually matters is that frameFor and
// PromptRefFor never disagree, since a draft stamped with a ref whose frame
// did not produce it is unreproducible.
func TestFrameForChannel(t *testing.T) {
	cases := []struct {
		channel   messaging.Channel
		want      string
		wantRef   string
		wantLabel string
	}{
		{messaging.ChannelWhatsApp, aiprompt.FrameShopKBV7RU(), aiprompt.PromptRefShopKBV7, "whatsapp"},
		{messaging.ChannelSimulator, aiprompt.FrameShopKBV7RU(), aiprompt.PromptRefShopKBV7, "simulator"},
		{messaging.ChannelTelegram, aiprompt.FrameShopKBV7TGRU(), aiprompt.PromptRefShopKBV7TG, "telegram"},
		{messaging.Channel(""), aiprompt.FrameShopKBV7RU(), aiprompt.PromptRefShopKBV7, "unset"},
		{messaging.Channel("something-else"), aiprompt.FrameShopKBV7RU(), aiprompt.PromptRefShopKBV7, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.wantLabel, func(t *testing.T) {
			if got := FrameFor(nil, tc.channel); got != tc.want {
				t.Fatalf("FrameFor(nil, %q) picked the wrong frame", tc.channel)
			}
			if got := PromptRefFor(nil, tc.channel); got != tc.wantRef {
				t.Fatalf("PromptRefFor(nil, %q) = %q, want %q", tc.channel, got, tc.wantRef)
			}
		})
	}
}

// TestFrameForSalon pins the salon-kb@v1 selection rule (PLAN.md): any
// specialist or service row picks the salon frame over shop-kb@v7,
// regardless of channel, and FrameFor/PromptRefFor never disagree about it.
func TestFrameForSalon(t *testing.T) {
	withSpecialist := &aiprompt.KB{Specialists: []aiprompt.Specialist{{Ref: "alina"}}}
	withService := &aiprompt.KB{Services: []aiprompt.Service{{Ref: "haircut"}}}
	empty := &aiprompt.KB{}

	for _, tc := range []struct {
		label   string
		kb      *aiprompt.KB
		channel messaging.Channel
		wantSalon bool
	}{
		{"specialist-whatsapp", withSpecialist, messaging.ChannelWhatsApp, true},
		{"specialist-telegram", withSpecialist, messaging.ChannelTelegram, true},
		{"service-only", withService, messaging.ChannelWhatsApp, true},
		{"no-salon-data", empty, messaging.ChannelWhatsApp, false},
		{"nil-kb", nil, messaging.ChannelWhatsApp, false},
	} {
		t.Run(tc.label, func(t *testing.T) {
			gotFrame := FrameFor(tc.kb, tc.channel)
			gotRef := PromptRefFor(tc.kb, tc.channel)
			if tc.wantSalon {
				if gotFrame != aiprompt.FrameSalonKBV1RU() {
					t.Errorf("FrameFor: want the salon frame, got a different one")
				}
				if gotRef != aiprompt.PromptRefSalonKBV1 {
					t.Errorf("PromptRefFor = %q, want %q", gotRef, aiprompt.PromptRefSalonKBV1)
				}
			} else {
				if gotFrame == aiprompt.FrameSalonKBV1RU() {
					t.Errorf("FrameFor: unexpectedly picked the salon frame")
				}
				if gotRef == aiprompt.PromptRefSalonKBV1 {
					t.Errorf("PromptRefFor: unexpectedly picked the salon ref")
				}
			}
		})
	}
}

func TestTelegramFrameIsADistinctFrame(t *testing.T) {
	if aiprompt.FrameShopKBV7TGRU() == aiprompt.FrameShopKBV7RU() {
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
		if !strings.Contains(FrameFor(nil, ch), aiprompt.SlotTariffCatalog) {
			t.Errorf("FrameFor(nil, %q) returned a frame with no %s slot — tariffs would be invisible to the model", ch, aiprompt.SlotTariffCatalog)
		}
	}
}
