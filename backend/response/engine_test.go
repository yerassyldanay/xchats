package response

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/llm"
)

// fakeClient is a scripted llm.ChatClient: call i returns responses[i] (or the
// last response, once exhausted) and records every request it received.
type fakeClient struct {
	responses []llm.ChatResponse
	calls     []llm.ChatRequest
}

func (f *fakeClient) Complete(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	i := len(f.calls)
	f.calls = append(f.calls, req)
	if i < len(f.responses) {
		return f.responses[i], nil
	}
	return f.responses[len(f.responses)-1], nil
}

type fakeRegistry struct{ client llm.ChatClient }

func (r *fakeRegistry) Client(ref llm.ModelRef) (llm.ChatClient, error) { return r.client, nil }

func testKB() *aiprompt.KB {
	return &aiprompt.KB{
		OrganizationID: "org-1",
		Assistant:      &aiprompt.Assistant{Persona: "Тестовый ассистент.", ReplyMaxWords: 120},
		Products: []aiprompt.Product{
			{Ref: "widget", Name: "Виджет", Price: "1 000 ₸", AvailabilityStatus: "in_stock", SalesStatus: "active"},
		},
		Contacts: &aiprompt.Contacts{Phone: "+7 700 000 00 00"},
		Policies: &aiprompt.Policies{DeliveryCost: "1 000 ₸", DeliveryInDays: "1-2"},
	}
}

func responseJSON(replyText string, media []string, escalate bool, reason string) string {
	if media == nil {
		media = []string{}
	}
	b, err := json.Marshal(map[string]any{
		"reply_text": replyText, "reply_language": "ru", "media_files_to_send": media,
		"escalate": escalate, "escalation_reason": reason,
	})
	if err != nil {
		panic(err)
	}
	return string(b)
}

func testParams() LLMParams {
	return LLMParams{
		DefaultModel: llm.ModelRef{Provider: "openrouter", Model: "test-model"},
		MaxTokens:    500,
		Temperature:  0.3,
		RetryEnabled: true,
	}
}

func testEngine(client llm.ChatClient) *Engine {
	return &Engine{
		LLMs:   &fakeRegistry{client: client},
		Params: testParams,
	}
}

func TestEngine_Generate_HappyPath(t *testing.T) {
	client := &fakeClient{responses: []llm.ChatResponse{
		{Text: responseJSON("Цена: {{product.widget.price}}.", nil, false, "")},
	}}
	e := testEngine(client)

	res, err := e.Generate(context.Background(), GenerateRequest{KB: testKB(), IncomingText: "Сколько стоит виджет?"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.FinalText != "Цена: 1 000 ₸." {
		t.Fatalf("FinalText = %q", res.FinalText)
	}
	if res.ReplyLanguage != "ru" || res.Escalate {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(client.calls) != 1 {
		t.Fatalf("want exactly 1 call (no retry needed), got %d", len(client.calls))
	}
	sentPrompt := client.calls[0].Messages[0].Content
	if !strings.Contains(sentPrompt, "Клиент пишет: Сколько стоит виджет?") {
		t.Fatalf("prompt missing conversation tail: %q", sentPrompt)
	}
	if client.calls[0].Model != "test-model" || client.calls[0].Temperature != 0.3 || client.calls[0].MaxTokens != 500 {
		t.Fatalf("unexpected request params: %+v", client.calls[0])
	}
}

func TestEngine_Generate_ContractShapeRetryResendsIdenticalPrompt(t *testing.T) {
	client := &fakeClient{responses: []llm.ChatResponse{
		{Text: "not json"},
		{Text: responseJSON("Хорошо.", nil, false, "")},
	}}
	e := testEngine(client)

	res, err := e.Generate(context.Background(), GenerateRequest{KB: testKB(), IncomingText: "Привет"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.FinalText != "Хорошо." {
		t.Fatalf("FinalText = %q", res.FinalText)
	}
	if len(client.calls) != 2 {
		t.Fatalf("want exactly 2 calls (one retry), got %d", len(client.calls))
	}
	if client.calls[0].Messages[0].Content != client.calls[1].Messages[0].Content {
		t.Fatalf("contract_shape retry must resend a byte-identical prompt:\nfirst:  %q\nsecond: %q",
			client.calls[0].Messages[0].Content, client.calls[1].Messages[0].Content)
	}
}

func TestEngine_Generate_MediaNotFoundRetryAppendsFeedback(t *testing.T) {
	kb := testKB()
	kb.Products[0].FeaturedImage = "m1"
	kb.Materials = []aiprompt.Material{{
		ID: "m1", OrganizationID: "org-1", SourceType: "file", SourceRef: "src.jpg", Filename: "photo.jpg",
		MimeType: "image/jpeg", SizeBytes: 100, StorageBackend: "fake", StorageKey: "fake://x",
		ProcessingStatus: "built", CustomerVisibility: "visible",
	}}
	badReply := responseJSON("Вот фото.", []string{"products.widget.nonexistent"}, false, "")
	goodReply := responseJSON("Вот фото.", []string{"products.widget.featured_image"}, false, "")
	client := &fakeClient{responses: []llm.ChatResponse{{Text: badReply}, {Text: goodReply}}}
	e := testEngine(client)

	res, err := e.Generate(context.Background(), GenerateRequest{KB: kb, IncomingText: "Покажи фото"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.FinalText != "Вот фото." {
		t.Fatalf("FinalText = %q", res.FinalText)
	}
	if len(client.calls) != 2 {
		t.Fatalf("want exactly 2 calls, got %d", len(client.calls))
	}
	secondPrompt := client.calls[1].Messages[0].Content
	if !strings.Contains(secondPrompt, "products.widget.nonexistent") {
		t.Fatalf("retry prompt should name the bad token: %q", secondPrompt)
	}
	if !strings.Contains(secondPrompt, "медиа не найдено") {
		t.Fatalf("retry prompt should carry the media_not_found corrective feedback: %q", secondPrompt)
	}
}

func TestEngine_Generate_RetryCappedAtOne(t *testing.T) {
	client := &fakeClient{responses: []llm.ChatResponse{
		{Text: "not json"}, {Text: "still not json"}, {Text: "definitely not json"},
	}}
	e := testEngine(client)

	if _, err := e.Generate(context.Background(), GenerateRequest{KB: testKB(), IncomingText: "Привет"}); err == nil {
		t.Fatal("want an error — retry is capped at one, so a still-broken response must hard-fail")
	}
	if len(client.calls) != 2 {
		t.Fatalf("want exactly 2 calls total (1 original + 1 capped retry), got %d", len(client.calls))
	}
}

func TestEngine_Generate_EscalationKeepsModelTextVerbatim(t *testing.T) {
	holding := "Сейчас у меня нет этой информации — передаю ваш вопрос менеджеру и вернусь с точным ответом."
	client := &fakeClient{responses: []llm.ChatResponse{
		{Text: responseJSON(holding, nil, true, "нет данных о ремонте")},
	}}
	e := testEngine(client)

	res, err := e.Generate(context.Background(), GenerateRequest{KB: testKB(), IncomingText: "Почините мою кофемашину?"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !res.Escalate {
		t.Fatal("want Escalate=true")
	}
	if res.FinalText != holding {
		t.Fatalf("FinalText = %q, want the model's holding text verbatim: %q", res.FinalText, holding)
	}
	if res.EscalationReason != "нет данных о ремонте" {
		t.Fatalf("EscalationReason = %q", res.EscalationReason)
	}
}

func TestEngine_Generate_NoRetryWhenDisabled(t *testing.T) {
	client := &fakeClient{responses: []llm.ChatResponse{
		{Text: "not json"}, {Text: responseJSON("ok", nil, false, "")},
	}}
	e := testEngine(client)
	e.Params = func() LLMParams {
		p := testParams()
		p.RetryEnabled = false
		return p
	}

	if _, err := e.Generate(context.Background(), GenerateRequest{KB: testKB(), IncomingText: "Привет"}); err == nil {
		t.Fatal("want an error — retry disabled means a bad response fails immediately")
	}
	if len(client.calls) != 1 {
		t.Fatalf("want exactly 1 call when retry is disabled, got %d", len(client.calls))
	}
}

func TestEngine_Generate_RequiresKB(t *testing.T) {
	e := testEngine(&fakeClient{})
	if _, err := e.Generate(context.Background(), GenerateRequest{IncomingText: "x"}); err == nil {
		t.Fatal("want an error when KB is nil")
	}
	if len(e.LLMs.(*fakeRegistry).client.(*fakeClient).calls) != 0 {
		t.Fatal("must not call the LLM when the KB is missing")
	}
}

// erroringClient always fails — the StatusHook tests' failure-path double.
type erroringClient struct{ err error }

func (c *erroringClient) Complete(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{}, c.err
}

func TestEngine_StatusHook_CalledWithProviderAndNilErrOnSuccess(t *testing.T) {
	client := &fakeClient{responses: []llm.ChatResponse{
		{Text: responseJSON("ok", nil, false, "")},
	}}
	e := testEngine(client)
	var gotProvider string
	var gotErr error
	calls := 0
	e.StatusHook = func(provider string, err error) {
		calls++
		gotProvider, gotErr = provider, err
	}

	if _, err := e.Generate(context.Background(), GenerateRequest{KB: testKB(), IncomingText: "hi"}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if calls != 1 {
		t.Fatalf("StatusHook called %d times, want 1 (no retry needed)", calls)
	}
	if gotProvider != "openrouter" || gotErr != nil {
		t.Errorf("StatusHook(provider=%q, err=%v), want (openrouter, nil)", gotProvider, gotErr)
	}
}

func TestEngine_StatusHook_CalledWithErrProviderAuthOnAuthFailure(t *testing.T) {
	authErr := fmt.Errorf("llmprovider: http 401: %w", llm.ErrProviderAuth)
	e := testEngine(&erroringClient{err: authErr})
	var gotErr error
	e.StatusHook = func(provider string, err error) { gotErr = err }

	if _, err := e.Generate(context.Background(), GenerateRequest{KB: testKB(), IncomingText: "hi"}); err == nil {
		t.Fatal("want Generate to fail when the LLM call fails")
	}
	if !errors.Is(gotErr, llm.ErrProviderAuth) {
		t.Errorf("StatusHook's err = %v, want errors.Is(_, llm.ErrProviderAuth)", gotErr)
	}
}

func TestEngine_StatusHook_NilIsSafeToLeaveUnset(t *testing.T) {
	client := &fakeClient{responses: []llm.ChatResponse{{Text: responseJSON("ok", nil, false, "")}}}
	e := testEngine(client) // StatusHook left nil
	if _, err := e.Generate(context.Background(), GenerateRequest{KB: testKB(), IncomingText: "hi"}); err != nil {
		t.Fatalf("Generate with a nil StatusHook: %v", err)
	}
}
