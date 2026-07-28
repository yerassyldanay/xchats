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
