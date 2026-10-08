package response

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/llm"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// withTemplate returns a shallow copy of kb carrying template id/text.
func withTemplate(kb *aiprompt.KB, id, text string, updated time.Time) *aiprompt.KB {
	c := *kb
	c.PromptTemplate = &aiprompt.PromptTemplate{ID: id, Instructions: text, UpdatedAt: updated}
	return &c
}

// Channels are handled automatically by a channel fact line: there is no
// per-channel (or per-vertical) frame to select and no Telegram profile.
func TestFrameFor_ChannelLine(t *testing.T) {
	kb := withTemplate(testKB(), "general", "ПРАВИЛА-МАРКЕР", time.Time{})
	cases := []struct {
		channel messaging.Channel
		want    string // "" = no channel line at all
	}{
		{messaging.ChannelWhatsApp, "WhatsApp"},
		{messaging.ChannelWhatsAppCloud, "WhatsApp"},
		{messaging.ChannelSimulator, "WhatsApp"}, // the simulator rehearses the WhatsApp path
		{messaging.ChannelTelegram, "Telegram"},
		{messaging.ChannelInstagram, "Instagram"},
		{messaging.ChannelMessenger, "Facebook Messenger"},
		{messaging.Channel(""), ""},
		{messaging.Channel("something-else"), ""},
	}
	for _, tc := range cases {
		t.Run(string(tc.channel)+"/"+tc.want, func(t *testing.T) {
			frame := FrameFor(kb, tc.channel)
			if !strings.HasPrefix(frame, "ПРАВИЛА-МАРКЕР\n\n") {
				t.Fatalf("the template instructions must lead the frame: %.60q", frame)
			}
			if tc.want == "" {
				if strings.Contains(frame, aiprompt.ChannelLinePrefix) {
					t.Errorf("no channel line expected for %q", tc.channel)
				}
				return
			}
			if !strings.Contains(frame, aiprompt.ChannelLinePrefix+tc.want+".") {
				t.Errorf("channel line for %q missing: want %q", tc.channel, tc.want)
			}
		})
	}
	// Frames for different channels differ ONLY in the channel line.
	wa := strings.Replace(FrameFor(kb, messaging.ChannelWhatsApp), "WhatsApp", "X", 1)
	tg := strings.Replace(FrameFor(kb, messaging.ChannelTelegram), "Telegram", "X", 1)
	if wa != tg {
		t.Error("channel must be the only difference between frames")
	}
}

func TestFrameFor_NoTemplateNoFrame(t *testing.T) {
	if FrameFor(nil, messaging.ChannelWhatsApp) != "" || FrameFor(&aiprompt.KB{}, messaging.ChannelWhatsApp) != "" {
		t.Error("a KB without a template has no frame")
	}
	if got := PromptRefFor(nil); got != "template:none" {
		t.Errorf("PromptRefFor(nil) = %q", got)
	}
}

func TestPromptRefFor(t *testing.T) {
	at := time.UnixMilli(1791417612345)
	if got := PromptRefFor(withTemplate(testKB(), "online-shop", "x", at)); got != "template:online-shop@1791417612345" {
		t.Errorf("ref = %q", got)
	}
	if got := PromptRefFor(withTemplate(testKB(), "general", "x", time.Time{})); got != "template:general@default" {
		t.Errorf("ref of a never-saved template = %q", got)
	}
}

// Products, tariffs, services and specialists appear together whenever present,
// whichever profile is active — the old salon-vs-shop frame switch is gone.
func TestFrameFor_SectionsFollowDataNotProfile(t *testing.T) {
	kb := testKB()
	kb.Services = []aiprompt.Service{{Ref: "haircut", ServiceType: "base", Name: "Стрижка", SalesStatus: "active"}}
	kb.Specialists = []aiprompt.Specialist{{Ref: "alina", FullName: "Алина", SalesStatus: "active"}}
	for _, id := range aiprompt.PromptTemplateIDs() {
		frame := FrameFor(withTemplate(kb, id, "x", time.Time{}), messaging.ChannelWhatsApp)
		for _, label := range []string{aiprompt.LabelProductsAvailable, aiprompt.LabelServices, aiprompt.LabelSpecialists} {
			if !strings.Contains(frame, label) {
				t.Errorf("profile %s: frame lacks %q although the KB has that data", id, label)
			}
		}
	}
}

// RenderSystemPrompt is the single builder both the preview and Generate use.
func TestRenderSystemPrompt(t *testing.T) {
	kb := withTemplate(testKB(), "general", "ПРАВИЛА-МАРКЕР", time.UnixMilli(1791417612345))
	sys, cat, err := RenderSystemPrompt(kb, messaging.ChannelTelegram)
	if err != nil {
		t.Fatal(err)
	}
	if cat == nil || sys.TemplateID != "general" || sys.Ref != "template:general@1791417612345" {
		t.Errorf("metadata = %+v", sys)
	}
	if !strings.Contains(sys.Text, "ПРАВИЛА-МАРКЕР") || !strings.Contains(sys.Text, "product: widget") ||
		!strings.Contains(sys.Text, aiprompt.ChannelLinePrefix+"Telegram.") || strings.Contains(sys.Text, "%%") {
		t.Errorf("rendered prompt incomplete:\n%s", sys.Text)
	}
	if sys.Frame != FrameFor(kb, messaging.ChannelTelegram) {
		t.Error("Frame must be exactly FrameFor's output")
	}

	if _, _, err := RenderSystemPrompt(nil, messaging.ChannelWhatsApp); err == nil {
		t.Error("nil KB must error")
	}
	noTpl := testKB()
	noTpl.PromptTemplate = nil
	if _, _, err := RenderSystemPrompt(noTpl, messaging.ChannelWhatsApp); !errors.Is(err, ErrNoPromptTemplate) {
		t.Errorf("a KB without a template must fail with ErrNoPromptTemplate, got %v", err)
	}
}

// Generate must send exactly the prompt the preview shows, built from the KB's
// template and its CURRENT data.
func TestGenerate_UsesTemplateAndCurrentKB(t *testing.T) {
	client := &fakeClient{responses: []llm.ChatResponse{{Text: responseJSON("Здравствуйте", nil, false, "")}}}
	e := testEngine(client)
	lastPrompt := func() string { return client.calls[len(client.calls)-1].Messages[0].Content }

	kb := withTemplate(testKB(), "online-shop", "ПРАВИЛА-ОТ-ОПЕРАТОРА", time.UnixMilli(1791417612345))
	if _, err := e.Generate(t.Context(), GenerateRequest{KB: kb, Channel: messaging.ChannelWhatsApp, IncomingText: "Привет"}); err != nil {
		t.Fatal(err)
	}
	sys, _, _ := RenderSystemPrompt(kb, messaging.ChannelWhatsApp)
	if !strings.HasPrefix(lastPrompt(), sys.Text) {
		t.Fatal("the model prompt must begin with exactly the preview text")
	}
	if !strings.Contains(lastPrompt(), "ПРАВИЛА-ОТ-ОПЕРАТОРА") {
		t.Error("the operator's template text did not reach the model")
	}

	// New KB data shows up on the very next call, and an edited template text too.
	kb2 := withTemplate(kb, "online-shop", "ДРУГИЕ-ПРАВИЛА", time.UnixMilli(1791417699999))
	kb2.Products = append(append([]aiprompt.Product{}, kb.Products...),
		aiprompt.Product{Ref: "gadget", Name: "Гаджет", Price: "2 000 ₸", AvailabilityStatus: "in_stock", SalesStatus: "active"})
	if _, err := e.Generate(t.Context(), GenerateRequest{KB: kb2, Channel: messaging.ChannelWhatsApp, IncomingText: "Привет"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(lastPrompt(), "ДРУГИЕ-ПРАВИЛА") || strings.Contains(lastPrompt(), "ПРАВИЛА-ОТ-ОПЕРАТОРА") ||
		!strings.Contains(lastPrompt(), "product: gadget") {
		t.Error("the second call must use the edited template and the new product")
	}
}
