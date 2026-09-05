package response

import (
	"context"
	"errors"
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/llm"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

type fakeConversationRepo struct {
	ctx ConversationContext
	err error
}

func (f *fakeConversationRepo) LoadForResponse(ctx context.Context, conversationID string) (ConversationContext, error) {
	return f.ctx, f.err
}

type fakeKBRepo struct {
	kb  *aiprompt.KB
	err error
}

func (f *fakeKBRepo) Load(ctx context.Context, organizationID string) (*aiprompt.KB, error) {
	return f.kb, f.err
}

type fakeDraftRepo struct {
	written []DraftToPersist
}

func (f *fakeDraftRepo) ReplaceSuggested(ctx context.Context, draft DraftToPersist) ([]PersistedDraft, error) {
	f.written = append(f.written, draft)
	return []PersistedDraft{{
		ID: "draft-1", ConversationID: draft.ConversationID, TriggerMessageID: draft.TriggerMessageID,
		Text: draft.Text, ReplyLanguage: draft.ReplyLanguage, Confidence: draft.Confidence,
		Escalate: draft.Escalate, EscalationReason: draft.EscalationReason,
	}}, nil
}

func testService(convRepo ConversationRepository, kbRepo KnowledgeBaseRepository, draftRepo DraftRepository, client llm.ChatClient) *Service {
	return &Service{
		Conversations: convRepo,
		KnowledgeBase: kbRepo,
		Drafts:        draftRepo,
		Engine:        testEngine(client),
	}
}

func TestService_Respond_HappyPath(t *testing.T) {
	convRepo := &fakeConversationRepo{ctx: ConversationContext{
		OrganizationID: "org-1", AccountID: "acct-1", Channel: messaging.ChannelWhatsApp,
		CurrentMessage: "Сколько стоит виджет?", TriggerMessageID: "trigger-1",
	}}
	kbRepo := &fakeKBRepo{kb: testKB()}
	client := &fakeClient{responses: []llm.ChatResponse{{Text: responseJSON("Цена: {{product.widget.price}}.", nil, false, "")}}}
	draftRepo := &fakeDraftRepo{}
	svc := testService(convRepo, kbRepo, draftRepo, client)

	out, err := svc.Respond(context.Background(), messaging.ChannelWhatsApp, "conv-1", RespondOptions{})
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if len(out) != 1 || out[0].Text != "Цена: 1 000 ₸." {
		t.Fatalf("unexpected result: %+v", out)
	}
	if len(draftRepo.written) != 1 || draftRepo.written[0].ConversationID != "conv-1" || draftRepo.written[0].TriggerMessageID != "trigger-1" {
		t.Fatalf("unexpected persisted draft: %+v", draftRepo.written)
	}
}

// TestService_Respond_ModelKBGapReachesDraftToPersist proves generate()
// copies GenerateResult.KBGap onto the persisted draft unchanged — the last
// hop before internal/responsestore.DraftRepo turns it into an
// ai_kb_gap_events row.
func TestService_Respond_ModelKBGapReachesDraftToPersist(t *testing.T) {
	convRepo := &fakeConversationRepo{ctx: ConversationContext{Channel: messaging.ChannelWhatsApp}}
	draftRepo := &fakeDraftRepo{}
	raw := `{"reply_text":"Секунду.","reply_language":"ru","media_files_to_send":[],"escalate":true,` +
		`"kb_gap":{"reason_code":"unsupported_request"}}`
	client := &fakeClient{responses: []llm.ChatResponse{{Text: raw}}}
	svc := testService(convRepo, &fakeKBRepo{kb: testKB()}, draftRepo, client)

	if _, err := svc.Respond(context.Background(), messaging.ChannelWhatsApp, "conv-1", RespondOptions{}); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	got := draftRepo.written[0].KBGap
	if got == nil || got.ReasonCode != aiprompt.KBGapReasonUnsupportedRequest {
		t.Fatalf("KBGap = %+v, want reason_code unsupported_request", got)
	}
	if got.Source != aiprompt.KBGapSourceModel {
		t.Errorf("Source = %q, want %q", got.Source, aiprompt.KBGapSourceModel)
	}
}

func TestService_Respond_ChannelMismatchFailsClosed(t *testing.T) {
	convRepo := &fakeConversationRepo{ctx: ConversationContext{Channel: messaging.ChannelWhatsApp}}
	draftRepo := &fakeDraftRepo{}
	svc := testService(convRepo, &fakeKBRepo{kb: testKB()}, draftRepo, &fakeClient{})

	if _, err := svc.Respond(context.Background(), messaging.ChannelSimulator, "conv-1", RespondOptions{}); err == nil {
		t.Fatal("want an error when the requested channel doesn't match the stored conversation")
	}
	if len(draftRepo.written) != 0 {
		t.Fatal("must not persist a draft on channel mismatch")
	}
}

func TestService_Respond_KBLoadFailureProducesHoldingDraft(t *testing.T) {
	convRepo := &fakeConversationRepo{ctx: ConversationContext{Channel: messaging.ChannelWhatsApp, TriggerMessageID: "trigger-1"}}
	draftRepo := &fakeDraftRepo{}
	svc := testService(convRepo, &fakeKBRepo{err: errors.New("kb not configured")}, draftRepo, &fakeClient{})

	out, err := svc.Respond(context.Background(), messaging.ChannelWhatsApp, "conv-1", RespondOptions{})
	if err != nil {
		t.Fatalf("Respond: %v (a KB load failure must produce a holding draft, not a hard error)", err)
	}
	if len(out) != 1 || out[0].Text != HoldingText || !out[0].Escalate {
		t.Fatalf("unexpected result: %+v", out)
	}
	if draftRepo.written[0].TriggerMessageID != "trigger-1" {
		t.Fatalf("holding draft must still tie to the trigger message: %+v", draftRepo.written[0])
	}
	assertEngineErrorKBGap(t, draftRepo.written[0])
}

// assertEngineErrorKBGap checks the KBGap holdingDraft stamps on every hard
// Generate failure: KBGapReasonEngineError/KBGapSourceEngine, never a
// fabricated KB entity or a code the model could have claimed itself.
func assertEngineErrorKBGap(t *testing.T, draft DraftToPersist) {
	t.Helper()
	if draft.KBGap == nil {
		t.Fatal("a holding draft must carry a KBGap diagnostic")
	}
	if draft.KBGap.ReasonCode != aiprompt.KBGapReasonEngineError {
		t.Errorf("KBGap.ReasonCode = %q, want %q", draft.KBGap.ReasonCode, aiprompt.KBGapReasonEngineError)
	}
	if draft.KBGap.Source != aiprompt.KBGapSourceEngine {
		t.Errorf("KBGap.Source = %q, want %q", draft.KBGap.Source, aiprompt.KBGapSourceEngine)
	}
	if draft.KBGap.TargetEntityType != "" || draft.KBGap.TargetEntityRef != "" {
		t.Errorf("a hard engine failure must never claim a specific KB entity: %+v", draft.KBGap)
	}
}

func TestService_Respond_EngineFailureProducesHoldingDraft(t *testing.T) {
	convRepo := &fakeConversationRepo{ctx: ConversationContext{Channel: messaging.ChannelWhatsApp}}
	draftRepo := &fakeDraftRepo{}
	client := &fakeClient{responses: []llm.ChatResponse{{Text: "not json"}, {Text: "still not json"}}}
	svc := testService(convRepo, &fakeKBRepo{kb: testKB()}, draftRepo, client)

	out, err := svc.Respond(context.Background(), messaging.ChannelWhatsApp, "conv-1", RespondOptions{})
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if len(out) != 1 || out[0].Text != HoldingText || !out[0].Escalate {
		t.Fatalf("unexpected result: %+v", out)
	}
	assertEngineErrorKBGap(t, draftRepo.written[0])
}

func TestService_Respond_ConversationLoadFailurePropagates(t *testing.T) {
	convRepo := &fakeConversationRepo{err: errors.New("not found")}
	draftRepo := &fakeDraftRepo{}
	svc := testService(convRepo, &fakeKBRepo{kb: testKB()}, draftRepo, &fakeClient{})

	if _, err := svc.Respond(context.Background(), messaging.ChannelWhatsApp, "conv-1", RespondOptions{}); err == nil {
		t.Fatal("want an error when the conversation can't be loaded")
	}
	if len(draftRepo.written) != 0 {
		t.Fatal("must not persist a draft when the conversation can't even be loaded")
	}
}

// TestService_Respond_KBOverrideBypassesLoad is KB-02's core service-level
// guarantee: when the caller (the simulator's "test against staged draft"
// mode) supplies KBOverride, generate() must use it verbatim instead of
// calling KnowledgeBase.Load — proven here by an override KB whose product
// price differs from fakeKBRepo's, showing up in the generated reply.
func TestService_Respond_KBOverrideBypassesLoad(t *testing.T) {
	convRepo := &fakeConversationRepo{ctx: ConversationContext{Channel: messaging.ChannelSimulator}}
	draftRepo := &fakeDraftRepo{}
	client := &fakeClient{responses: []llm.ChatResponse{{Text: responseJSON("Цена: {{product.widget.price}}.", nil, false, "")}}}
	// fakeKBRepo.err is set: if generate() ever fell through to Load(), the
	// call would produce a holding draft instead of the scripted reply below
	// — this doubles as proof Load was never reached.
	svc := testService(convRepo, &fakeKBRepo{err: errors.New("Load must not be called when KBOverride is set")}, draftRepo, client)

	override := testKB()
	override.Products[0].Price = "2 000 ₸ (черновик)"

	out, err := svc.Respond(context.Background(), messaging.ChannelSimulator, "conv-1", RespondOptions{KBOverride: override})
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if len(out) != 1 || out[0].Text != "Цена: 2 000 ₸ (черновик)." {
		t.Fatalf("unexpected result (KBOverride was not used): %+v", out)
	}
}

func TestService_Respond_ModelOverridePassedToEngine(t *testing.T) {
	convRepo := &fakeConversationRepo{ctx: ConversationContext{Channel: messaging.ChannelSimulator}}
	draftRepo := &fakeDraftRepo{}
	client := &fakeClient{responses: []llm.ChatResponse{{Text: responseJSON("ok", nil, false, "")}}}
	svc := testService(convRepo, &fakeKBRepo{kb: testKB()}, draftRepo, client)

	override := llm.ModelRef{Provider: "openrouter", Model: "override-model"}
	if _, err := svc.Respond(context.Background(), messaging.ChannelSimulator, "conv-1", RespondOptions{ModelOverride: &override}); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if len(client.calls) != 1 || client.calls[0].Model != "override-model" {
		t.Fatalf("model override was not passed through: %+v", client.calls)
	}
}
