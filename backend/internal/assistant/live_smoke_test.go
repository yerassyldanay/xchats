package assistant

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/brain"
	"github.com/yerassyldanay/xchats/backend/internal/brain/llm"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// resolveBaseURL mirrors config.LLMResolvedBaseURL for the standalone live test.
func resolveBaseURL() string {
	if b := os.Getenv("LLM_BASE_URL"); b != "" {
		return strings.TrimRight(b, "/")
	}
	switch strings.ToLower(os.Getenv("LLM_PROVIDER")) {
	case "openai":
		return "https://api.openai.com/v1"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta/openai"
	default:
		return "https://openrouter.ai/api/v1"
	}
}

func liveWindow(text string) fakeWindow {
	ts := time.Now()
	return fakeWindow{msgs: []store.Message{
		{ID: uuid.New(), Direction: "in", Body: text, MessageTS: &ts},
	}}
}

// TestLive_RealBrain hits the configured live provider through the exact
// production path (llm.Drafter → RealDrafter → SeedSnapshot post-process).
// Skipped unless LLM_API_KEY is set, so normal CI stays offline.
//
//	go test ./internal/assistant -run TestLive -v   (with LLM_* exported)
func TestLive_RealBrain(t *testing.T) {
	key := os.Getenv("LLM_API_KEY")
	if key == "" {
		t.Skip("LLM_API_KEY unset; skipping live provider test")
	}
	fast := os.Getenv("LLM_FAST_MODEL")
	if fast == "" {
		fast = "openai/gpt-4o-mini"
	}
	maxTok, _ := strconv.Atoi(os.Getenv("LLM_MAX_TOKENS"))
	if maxTok == 0 {
		maxTok = 1024
	}
	temp, _ := strconv.ParseFloat(os.Getenv("LLM_TEMPERATURE"), 64)

	client := llm.New(resolveBaseURL(), key, os.Getenv("LLM_PROVIDER"), fast, os.Getenv("LLM_THINKING_MODEL"), maxTok, temp)

	t.Run("grounded pricing question renders a real price", func(t *testing.T) {
		r := NewReal(liveWindow("Сколько стоит тариф Стандарт?"), client, brain.SeedSnapshot(), nil)
		opts, err := r.Draft(context.Background(), Input{ChatID: uuid.New().String()})
		if err != nil {
			t.Fatalf("live draft: %v", err)
		}
		o := opts[0]
		t.Logf("REPLY: %q | escalate=%v | conf=%v | media=%d", o.Text, o.Escalate, deref(o.Confidence), len(o.Media))
		if o.Escalate {
			t.Fatalf("pricing question should be grounded, not escalated: %q", o.Text)
		}
		if strings.Contains(o.Text, "{{") {
			t.Fatalf("price token left unrendered: %q", o.Text)
		}
		if !strings.Contains(o.Text, "₸") {
			t.Fatalf("expected a rendered tenge price in the reply: %q", o.Text)
		}
	})

	t.Run("off-KB refund question escalates", func(t *testing.T) {
		r := NewReal(liveWindow("Вы возвращаете деньги, если мне не понравится?"), client, brain.SeedSnapshot(), nil)
		opts, err := r.Draft(context.Background(), Input{ChatID: uuid.New().String()})
		if err != nil {
			t.Fatalf("live draft: %v", err)
		}
		o := opts[0]
		t.Logf("REPLY: %q | escalate=%v | reason=%q", o.Text, o.Escalate, o.Reason)
		if !o.Escalate {
			t.Errorf("refund/returns question is outside the KB guardrails; expected escalation, got: %q", o.Text)
		}
	})
}

func deref(p *float64) float64 {
	if p == nil {
		return -1
	}
	return *p
}
