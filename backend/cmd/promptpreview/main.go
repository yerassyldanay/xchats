// Command promptpreview is an eval/analysis tool, not part of the product.
// It builds the REAL drafting prompt from the embedded seed Snapshot
// (brain.BuildSystem / brain.BuildUser), imitates a customer asking about a
// product, and with -call sends it through the production LLM client and
// post-processes the draft exactly like the server does. When LANGFUSE_* is
// enabled in .env the call is exported to Langfuse as a generation span.
//
// Usage (from repo root):
//
//	cd backend && go run ./cmd/promptpreview -config ../config.yaml -env ../.env [-call]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/brain"
	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
	"github.com/yerassyldanay/xchats/backend/internal/brain/llm"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/telemetry"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config.yaml")
	envPath := flag.String("env", ".env", "path to .env")
	call := flag.Bool("call", false, "also call the LLM and post-process the draft")
	flag.Parse()

	snap := brain.SeedSnapshot()

	// Imitated conversation: the customer asks about a product (price, delivery
	// cost, and a photo — the photo probes media selection when no product photo
	// exists in the catalog).
	window := []domain.Message{
		{Role: domain.RoleCustomer, Content: "Здравствуйте! Кофемашина DeLonghi есть в наличии?"},
		{Role: domain.RoleAgent, Content: "Здравствуйте! Да, есть 🙂"},
	}
	current := domain.Message{Role: domain.RoleCustomer,
		Content: "Отлично! Сколько она стоит и сколько будет доставка по Алматы? Пришлите фото, пожалуйста."}

	p := brain.Prompt{System: brain.BuildSystem(snap), User: brain.BuildUser(nil, window, current)}

	fmt.Println("================ SYSTEM PROMPT (cache-stable, per org) ================")
	fmt.Println(p.System)
	fmt.Println("================ USER PROMPT (per chat) ===============================")
	fmt.Println(p.User)
	fmt.Printf("---- size estimate: system ~%d tokens, user ~%d tokens ----\n",
		len(p.System)/4, len(p.User)/4)

	if !*call {
		return
	}

	cfg, err := config.Load(*cfgPath, *envPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if cfg.LangfuseTracingEnabled() {
		if tp, terr := telemetry.NewLangfuseProvider(ctx, cfg, "xchats-eval"); terr != nil {
			fmt.Println("[langfuse] init failed, continuing without it:", terr)
		} else {
			fmt.Println("[langfuse] tracing enabled →", cfg.LangfuseHost)
			defer func() {
				shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = tp.Shutdown(shutCtx)
			}()
		}
	}

	d := llm.New(cfg.LLMResolvedBaseURL(), cfg.LLMAPIKey, cfg.LLMProvider,
		cfg.LLMFastModel, "", cfg.LLMMaxTokens, cfg.LLMTemperature)

	start := time.Now()
	raw, err := d.Draft(ctx, p)
	if err != nil {
		fmt.Fprintln(os.Stderr, "llm draft:", err)
		os.Exit(1)
	}
	fmt.Printf("================ RAW emit_draft (model %s, %.1fs) ====================\n",
		cfg.LLMFastModel, time.Since(start).Seconds())
	rawJSON, _ := json.MarshalIndent(raw, "", "  ")
	fmt.Println(string(rawJSON))

	final := brain.PostProcess(raw, snap, slog.Default())
	fmt.Println("================ FINAL DRAFT (after PostProcess) ======================")
	fmt.Println("reply:   ", final.ReplyText)
	for _, m := range final.Media {
		fmt.Printf("media:    %s (%s) → %s\n", m.Ref, m.Kind, m.URL)
	}
	if len(final.Media) == 0 {
		fmt.Println("media:    (none)")
	}
	fmt.Println("escalate:", final.Escalate, "| confidence:", final.Confidence,
		"| pricing_error:", final.PricingError, "| dropped_refs:", final.DroppedRefs)
}
