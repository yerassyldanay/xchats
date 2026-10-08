// Package response is the channel-neutral response engine and service: given
// an organization's knowledge base and a conversation, it renders the
// system prompt from the business's active editable template, calls the configured LLM, and returns a
// grounded, validated draft reply. It depends only on backend/aiprompt,
// backend/llm's contracts, backend/messaging's contracts, and its own
// repository interfaces — never on a specific channel provider, PostgreSQL,
// or any concrete LLM provider's wire format. Those live in backend/internal
// and are wired together only at the composition root (cmd/xchats).
package response

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/llm"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// LLMParams is the response engine's per-request model configuration:
// which provider/model to call by default, and the sampling/retry knobs.
type LLMParams struct {
	DefaultModel llm.ModelRef
	// VisionModel is the model Generate routes to instead of DefaultModel
	// when the request carries an image attachment — a zero ModelRef (Model
	// == "") means vision is not configured, so an image attachment is
	// described to the model as text only (see GenerateRequest.Attachments
	// and attachmentTailSuffix). It always shares DefaultModel's Provider:
	// the Settings UI offers one model string, not a second provider
	// selector, since every provider this engine calls is the same
	// OpenAI-wire-compatible surface for both chat and vision.
	VisionModel  llm.ModelRef
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
	// Attachments are the trigger message's own media (images, transcribed
	// audio) — see IncomingAttachment. An image attachment routes the call
	// to LLMParams.VisionModel (when configured) and is attached to the
	// model's user message as an extra content part; an audio attachment's
	// transcript is folded into the conversation tail as ordinary text.
	Attachments []IncomingAttachment
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

	sys, cat, err := RenderSystemPrompt(req.KB, req.Channel)
	if err != nil {
		return nil, err
	}
	rendered := sys.Text
	incomingText := req.IncomingText
	if suffix := attachmentTailSuffix(req.Attachments, incomingText); suffix != "" {
		if incomingText != "" {
			incomingText += "\n" + suffix
		} else {
			incomingText = suffix
		}
	}
	prompt := rendered + aiprompt.RenderCustomerContext(req.Customer) +
		aiprompt.ConversationTail(aiprompt.RenderHistory(req.History), incomingText)

	params := e.Params()
	// attachVision gates whether an image is actually attached as vision
	// input, not just whether one is present: with no VisionModel
	// configured, DefaultModel may not even be multimodal-capable, so an
	// image attachment stays a text-only note (attachmentTailSuffix above)
	// rather than being sent as an image part it might reject outright.
	attachVision := hasImageAttachment(req.Attachments) && params.VisionModel.Model != ""
	modelRef := params.DefaultModel
	switch {
	case req.ModelOverride != nil:
		modelRef = *req.ModelOverride
	case attachVision:
		modelRef = params.VisionModel
	}
	client, err := e.LLMs.Client(modelRef)
	if err != nil {
		return nil, fmt.Errorf("response: resolve model client: %w", err)
	}

	var visionAttachments []IncomingAttachment
	if attachVision {
		visionAttachments = req.Attachments
	}

	raw, err := e.complete(ctx, client, modelRef, prompt, visionAttachments, params)
	if err != nil {
		return nil, fmt.Errorf("response: llm call: %w", err)
	}

	if reason := aiprompt.ClassifyRetryV7(raw, req.KB, cat); reason != aiprompt.RetryReasonNone && params.RetryEnabled {
		retryPrompt := prompt + aiprompt.RetryFeedbackV7(raw, req.KB, cat)
		retryRaw, err := e.complete(ctx, client, modelRef, retryPrompt, visionAttachments, params)
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

// ErrNoPromptTemplate means the knowledge base reached the prompt builder
// without its active template: the repository (responsestore.Load) always
// attaches one, so this is a wiring bug, not a user-facing condition.
var ErrNoPromptTemplate = errors.New("response: knowledge base has no prompt template loaded")

// SystemPrompt is a fully rendered system prompt and how it was produced.
type SystemPrompt struct {
	// Text is the prompt sent to the model (before the customer/conversation tail).
	Text string
	// Frame is the composed frame it was rendered from — the operator's
	// template instructions plus the protected structure, slot markers intact.
	Frame string
	// Ref identifies the template revision that produced it (PromptRefFor).
	Ref string
	// TemplateID is the active template profile.
	TemplateID string
}

// RenderSystemPrompt is THE prompt builder: Engine.Generate (every customer
// reply) and the Final Template preview (GET /kb/prompt) both call it, so what
// an operator previews is exactly what the model receives. It builds the
// catalog, composes the frame from the KB's active template (aiprompt.ComposeFrame,
// which includes a data section for every kind of data the KB holds, regardless
// of profile), renders it, and runs the material and storage-locator leak
// gates. The prompt is rebuilt from the latest KB on each call — it is never
// stored.
func RenderSystemPrompt(kb *aiprompt.KB, channel messaging.Channel) (*SystemPrompt, *aiprompt.Catalog, error) {
	if kb == nil {
		return nil, nil, fmt.Errorf("response: knowledge base is required")
	}
	if kb.PromptTemplate == nil {
		return nil, nil, ErrNoPromptTemplate
	}
	cat, err := aiprompt.BuildCatalog(kb)
	if err != nil {
		return nil, nil, fmt.Errorf("response: build catalog: %w", err)
	}
	frame := FrameFor(kb, channel)
	rendered, err := aiprompt.RenderPromptV7(frame, kb.PromptInput(), cat)
	if err != nil {
		return nil, nil, fmt.Errorf("response: render prompt: %w", err)
	}
	if err := aiprompt.ValidateNoMaterialLeak(rendered, kb.Materials); err != nil {
		return nil, nil, fmt.Errorf("response: %w", err)
	}
	if err := aiprompt.ValidateNoStorageLocatorLeak(rendered); err != nil {
		return nil, nil, fmt.Errorf("response: %w", err)
	}
	return &SystemPrompt{Text: rendered, Frame: frame, Ref: PromptRefFor(kb), TemplateID: kb.PromptTemplate.ID}, cat, nil
}

// FrameFor composes the frame for an organization's KB and channel from the
// KB's active template: the operator-editable instructions, then the protected
// structure (channel line, strict-JSON contract, schema, assistant block and a
// bare-labelled data section for each kind of data the KB actually holds).
// There is no per-channel or per-vertical frame choice any more — channels are
// handled automatically by the channel line, and products, tariffs, services,
// specialists and zones appear together whenever present. A KB without a
// template yields "".
func FrameFor(kb *aiprompt.KB, channel messaging.Channel) string {
	if kb == nil || kb.PromptTemplate == nil {
		return ""
	}
	return aiprompt.ComposeFrame(kb.PromptTemplate.Instructions, kb, channelLabel(channel))
}

// PromptRefFor names the template revision FrameFor would use, for logs and
// draft provenance: "template:<id>@<updated_at ms>". A template served from the
// shipped default (never saved) reads "@default".
func PromptRefFor(kb *aiprompt.KB) string {
	if kb == nil || kb.PromptTemplate == nil {
		return "template:none"
	}
	t := kb.PromptTemplate
	if t.UpdatedAt.IsZero() {
		return "template:" + t.ID + "@default"
	}
	return fmt.Sprintf("template:%s@%d", t.ID, t.UpdatedAt.UnixMilli())
}

// channelLabel is the human name of the conversation channel, told to the model
// as a fact. The simulator rehearses the WhatsApp path. An unset or unknown
// channel adds no channel line.
func channelLabel(ch messaging.Channel) string {
	switch ch {
	case messaging.ChannelWhatsApp, messaging.ChannelWhatsAppCloud, messaging.ChannelSimulator:
		return "WhatsApp"
	case messaging.ChannelTelegram:
		return "Telegram"
	case messaging.ChannelInstagram:
		return "Instagram"
	case messaging.ChannelMessenger:
		return "Facebook Messenger"
	default:
		return ""
	}
}

func (e *Engine) complete(ctx context.Context, client llm.ChatClient, modelRef llm.ModelRef, prompt string, attachments []IncomingAttachment, params LLMParams) (string, error) {
	resp, err := client.Complete(ctx, llm.ChatRequest{
		Model:       modelRef.Model,
		Messages:    []llm.Message{userMessage(prompt, attachments)},
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

// hasImageAttachment reports whether atts contains at least one image
// resolved to a data URI and therefore ready to attach to a vision call.
func hasImageAttachment(atts []IncomingAttachment) bool {
	for _, a := range atts {
		if a.Kind == AttachmentImage && a.DataURI != "" {
			return true
		}
	}
	return false
}

// userMessage builds the model-facing user turn: plain text for the common
// (no-image) case, or a multi-part message — the rendered prompt as one
// text part, followed by one image part per attachment — once any image is
// present. See llm.Message.Parts' own doc comment for why Parts, not
// Content, carries a multimodal turn.
func userMessage(prompt string, attachments []IncomingAttachment) llm.Message {
	var images []llm.ContentPart
	for _, a := range attachments {
		if a.Kind == AttachmentImage && a.DataURI != "" {
			images = append(images, llm.ContentPart{Kind: llm.PartImage, ImageURL: a.DataURI})
		}
	}
	if len(images) == 0 {
		return llm.Message{Role: "user", Content: prompt}
	}
	parts := append([]llm.ContentPart{{Kind: llm.PartText, Text: prompt}}, images...)
	return llm.Message{Role: "user", Parts: parts}
}

// attachmentTailSuffix renders the trigger message's non-text attachments as
// bracketed labels appended to the customer's own text, so the conversation
// tail the model reads never silently drops that the customer sent media —
// a transcribed voice note becomes "[Голосовое сообщение]: <transcript>"
// and an image becomes "[Прикреплено фото]" (plus its caption, if any).
// This runs regardless of whether an image is ALSO attached as vision
// input (hasImageAttachment/userMessage) — a model with no VisionModel
// configured still needs to know a photo arrived, even though it cannot see
// it.
//
// incomingText is the caller's OWN incoming text (before this suffix is
// appended to it) — an image's Caption is the trigger message's own body
// (see responsestore.resolveAttachments), which is already incomingText
// itself in the overwhelmingly common case of one photo with one caption.
// Appending it again here would put the customer's words in the prompt
// twice; it is only worth restating next to "[Прикреплено фото]" when
// incomingText is empty (an image with no other text at all), so the model
// still sees it once.
func attachmentTailSuffix(atts []IncomingAttachment, incomingText string) string {
	var lines []string
	for _, a := range atts {
		switch a.Kind {
		case AttachmentAudio:
			if a.Transcript != "" {
				lines = append(lines, "[Голосовое сообщение]: "+a.Transcript)
			}
		case AttachmentImage:
			label := "[Прикреплено фото]"
			if a.Caption != "" && incomingText == "" {
				label += " " + a.Caption
			}
			lines = append(lines, label)
		}
	}
	return strings.Join(lines, "\n")
}
