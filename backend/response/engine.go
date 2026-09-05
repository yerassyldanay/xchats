// Package response is the channel-neutral response engine and service: given
// an organization's knowledge base and a conversation, it renders the
// evaluated shop-kb-v4 prompt, calls the configured LLM, and returns a
// grounded, validated draft reply. It depends only on backend/aiprompt,
// backend/llm's contracts, backend/messaging's contracts, and its own
// repository interfaces — never on a specific channel provider, PostgreSQL,
// or any concrete LLM provider's wire format. Those live in backend/internal
// and are wired together only at the composition root (cmd/xchats).
package response

import (
	"context"
	"fmt"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/llm"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// LLMParams is the response engine's per-request model configuration:
// which provider/model to call by default, and the sampling/retry knobs.
type LLMParams struct {
	DefaultModel llm.ModelRef
	MaxTokens    int
	Temperature  float64
	RetryEnabled bool
}

// Engine renders the evaluated prompt, calls the configured LLM, and
// validates/grounds its response. It has no database, channel-send, or
// provider-HTTP dependency, and constructs no fake knowledge-base data —
// GenerateRequest.KB must already be a real, loaded knowledge base.
type Engine struct {
	LLMs llm.Registry
	// Params returns the CURRENT LLMParams — called once per Generate,
	// never cached across calls or frozen at construction. This is what
	// lets a composition root's runtime-configurable settings (xchats'
	// Settings UI, internal/settings, wired in at cmd/xchats) take effect
	// on the very next request with no restart; a composition root with no
	// such thing can just close over a fixed LLMParams value. response
	// itself has no internal/settings dependency — see this package's own
	// doc comment on why it stays composition-root-agnostic.
	Params func() LLMParams
	// StatusHook, if set, is called after every LLM Complete attempt (the
	// initial call and, if it runs, the retry) with the provider that was
	// called and the resulting error (nil on success). Best-effort and
	// synchronous but never fatal to Generate — a nil StatusHook is simply
	// not called. This is the composition root's seam for self-healing
	// status reporting (internal/providerhealth) without response itself
	// depending on internal/realtime or any other internal/ package.
	StatusHook func(provider string, err error)
}

// GenerateRequest is one channel-neutral request to produce a draft reply.
type GenerateRequest struct {
	OrganizationID string
	ConversationID string
	Channel        messaging.Channel
	History        []aiprompt.HistoryTurn
	IncomingText   string
	KB             *aiprompt.KB
	// Customer is the CRM profile of the person being answered, or nil when
	// the conversation has no customer yet (an unassigned account, or a chat
	// that predates the CRM layer). Nil renders no block at all, which keeps
	// the assembled prompt byte-identical to the eval harness's — see
	// aiprompt.RenderCustomerContext.
	//
	// It is read-only context: the response contract has no CRM fields, so a
	// generated reply can never change a status, tag, assignee or follow-up.
	Customer *aiprompt.CustomerContext
	// ModelOverride is settable only by the authenticated simulator handler,
	// and only to a registered provider; nil in production (the WhatsApp path
	// never sets it).
	ModelOverride *llm.ModelRef
}

// GenerateResult is the engine's grounded, contract-validated output.
// Escalate=true keeps the model's own eval-validated holding text in
// FinalText — a canned holding string is only ever produced by the caller on
// a hard Generate error, never substituted in here.
type GenerateResult struct {
	FinalText        string
	ReplyLanguage    string
	Escalate         bool
	EscalationReason string
	Confidence       float64
	// KBGap is the optional v7 structured escalation diagnostic
	// (aiprompt.KBGapDiagnostic), already sanitized against the loaded KB by
	// aiprompt.ValidateResponseV7. Nil whenever the model did not send one
	// (including every case sanitizeKBGap drops as untrustworthy) — a nil
	// KBGap alongside Escalate=true is normal and expected, not an error;
	// callers fall back to EscalationReason's free text exactly as before.
	KBGap *aiprompt.KBGapDiagnostic
}

// Generate renders the prompt, calls the LLM (retrying once when the response
// is a contract_shape or media_not_found candidate and RetryEnabled is set),
// and returns the validated, fact-substituted result. Any failure — a KB that
// fails to build a catalog, a leak-gate rejection, an LLM/provider error, or a
// response that still fails contract validation after the retry — is returned
// as a plain error; producing a holding/escalation draft from that error is
// the caller's job (response.Service), not the engine's.
func (e *Engine) Generate(ctx context.Context, req GenerateRequest) (*GenerateResult, error) {
	if req.KB == nil {
		return nil, fmt.Errorf("response: GenerateRequest.KB is required")
	}

	cat, err := aiprompt.BuildCatalog(req.KB)
	if err != nil {
		return nil, fmt.Errorf("response: build catalog: %w", err)
	}
	rendered, err := aiprompt.RenderPromptV7(frameFor(req.Channel), req.KB.PromptInput(), cat)
	if err != nil {
		return nil, fmt.Errorf("response: render prompt: %w", err)
	}
	if err := aiprompt.ValidateNoMaterialLeak(rendered, req.KB.Materials); err != nil {
		return nil, fmt.Errorf("response: %w", err)
	}
	if err := aiprompt.ValidateNoStorageLocatorLeak(rendered); err != nil {
		return nil, fmt.Errorf("response: %w", err)
	}
	prompt := rendered + aiprompt.RenderCustomerContext(req.Customer) +
		aiprompt.ConversationTail(aiprompt.RenderHistory(req.History), req.IncomingText)

	params := e.Params()
	modelRef := params.DefaultModel
	if req.ModelOverride != nil {
		modelRef = *req.ModelOverride
	}
	client, err := e.LLMs.Client(modelRef)
	if err != nil {
		return nil, fmt.Errorf("response: resolve model client: %w", err)
	}

	raw, err := e.complete(ctx, client, modelRef, prompt, params)
	if err != nil {
		return nil, fmt.Errorf("response: llm call: %w", err)
	}

	if reason := aiprompt.ClassifyRetryV7(raw, req.KB, cat); reason != aiprompt.RetryReasonNone && params.RetryEnabled {
		retryPrompt := prompt + aiprompt.RetryFeedbackV7(raw, req.KB, cat)
		retryRaw, err := e.complete(ctx, client, modelRef, retryPrompt, params)
		if err != nil {
			return nil, fmt.Errorf("response: llm retry call: %w", err)
		}
		raw = retryRaw
	}

	ext, ok := aiprompt.ExtractFinalOutput(raw)
	if !ok {
		return nil, fmt.Errorf("response: model output has no extractable final answer")
	}
	resp, issues := aiprompt.ValidateResponseV7(ext.Final, req.KB, cat)
	if resp == nil {
		return nil, fmt.Errorf("response: response fails the contract shape: %+v", issues)
	}
	if len(issues) > 0 {
		return nil, fmt.Errorf("response: response fails validation: %+v", issues)
	}

	finalText, err := aiprompt.SubstituteFactsLang(resp.ReplyText, req.KB, cat, resp.ReplyLanguage)
	if err != nil {
		return nil, fmt.Errorf("response: substitute facts: %w", err)
	}
	if _, err := aiprompt.ResolveSend(resp.MediaFilesToSend, req.KB, cat); err != nil {
		return nil, fmt.Errorf("response: resolve media: %w", err)
	}

	return &GenerateResult{
		FinalText:        finalText,
		ReplyLanguage:    resp.ReplyLanguage,
		Escalate:         resp.Escalate,
		EscalationReason: resp.EscalationReason,
		Confidence:       resp.Confidence,
		KBGap:            resp.KBGap,
	}, nil
}

// frameFor picks the prompt frame for a channel. WhatsApp and the simulator
// keep the byte-identical base frame (the simulator exists to rehearse the
// WhatsApp path, so it must not diverge from it); Telegram gets the variant
// whose only difference is a persona line that does not call the assistant a
// WhatsApp one. An unset channel — a caller that predates GenerateRequest
// carrying it — keeps the base frame rather than guessing.
//
// v7 (not v6) since 2026-09 (0018_kb_gap_telemetry): v6 had no way for an
// escalation to report WHY in any queryable shape — every gap in the
// knowledge base (an unpriced product, a request outside the supported
// scope, an unknown item) landed as the same free-text escalation_reason
// prose, unreportable except by reading chat transcripts one at a time. v7
// is v6 plus the optional "kb_gap" structured diagnostic (rule 9) — see
// aiprompt.PromptRefShopKBV7, which also records that the eval pipeline has
// not been run against it yet. Generate must call ValidateResponseV7 (not
// ValidateResponse) whenever this function returns a v7 frame — the two
// move in lockstep exactly as frameFor and PromptRefFor do.
func frameFor(channel messaging.Channel) string {
	if channel == messaging.ChannelTelegram {
		return aiprompt.FrameShopKBV7TGRU()
	}
	return aiprompt.FrameShopKBV7RU()
}

// PromptRefFor names the frame frameFor would pick, for logs and draft records.
// It must move in lockstep with frameFor: a draft stamped with a ref whose
// frame did not produce it is unreproducible.
func PromptRefFor(channel messaging.Channel) string {
	if channel == messaging.ChannelTelegram {
		return aiprompt.PromptRefShopKBV7TG
	}
	return aiprompt.PromptRefShopKBV7
}

func (e *Engine) complete(ctx context.Context, client llm.ChatClient, modelRef llm.ModelRef, prompt string, params LLMParams) (string, error) {
	resp, err := client.Complete(ctx, llm.ChatRequest{
		Model:       modelRef.Model,
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: params.Temperature,
		MaxTokens:   params.MaxTokens,
	})
	if e.StatusHook != nil {
		e.StatusHook(modelRef.Provider, err)
	}
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}
