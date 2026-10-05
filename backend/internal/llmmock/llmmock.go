// Package llmmock is an in-memory llm.Registry/llm.ChatClient/
// llm.StreamClient for cmd/xchats' mock-externals mode. It never makes an
// HTTP call: Complete sniffs the rendered prompt text to tell apart the
// three purpose-specific contracts this codebase's callers actually expect
// (response.Engine's aiprompt v7 customer-response JSON, internal/kbimport's
// synthesis tool-call JSON, and everything else falls back to plain prose),
// and Stream — used only by internal/chat's KB-chat assistant, the one
// caller that type-asserts for llm.StreamClient — always answers with
// streamed prose, chunked so incremental-rendering callers see more than
// one delta.
package llmmock

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/yerassyldanay/xchats/backend/llm"
)

// maxRecordedCalls bounds Client.Calls() — see metamock's identical
// reasoning (a long profiling run must not grow the fake's own memory).
const maxRecordedCalls = 200

// Call records one Complete/Stream request for test/introspection use.
type Call struct {
	Streamed bool
	Request  llm.ChatRequest
}

// Registry always resolves to the same shared Client regardless of
// ref.Provider/ref.Model — mock mode has no per-provider credentials to
// pick between, and every caller (response.Engine, internal/chat,
// internal/kbimport) just needs *a* working llm.ChatClient.
type Registry struct {
	client *Client
}

// NewRegistry returns a Registry backed by a fresh Client.
func NewRegistry() *Registry {
	return &Registry{client: NewClient()}
}

func (r *Registry) Client(ref llm.ModelRef) (llm.ChatClient, error) {
	return r.client, nil
}

// Underlying returns the shared mock Client, for tests that want to inspect
// recorded calls.
func (r *Registry) Underlying() *Client { return r.client }

var _ llm.Registry = (*Registry)(nil)

// Client is an in-memory llm.ChatClient + llm.StreamClient. Zero value is
// ready to use; NewClient is equivalent, kept for symmetry with metamock.
type Client struct {
	mu    sync.Mutex
	calls []Call
}

// NewClient returns a ready-to-use mock Client.
func NewClient() *Client { return &Client{} }

var (
	_ llm.ChatClient   = (*Client)(nil)
	_ llm.StreamClient = (*Client)(nil)
)

func (c *Client) record(req llm.ChatRequest, streamed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, Call{Streamed: streamed, Request: req})
	if over := len(c.calls) - maxRecordedCalls; over > 0 {
		c.calls = c.calls[over:]
	}
}

// Calls returns a snapshot of every recorded call, oldest first.
func (c *Client) Calls() []Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Call, len(c.calls))
	copy(out, c.calls)
	return out
}

// promptText concatenates every message's own Content plus any text content
// parts — enough to sniff which purpose-specific contract the caller wants;
// an image part's data URI is deliberately excluded (irrelevant to contract
// detection, and needlessly bloats the string on a vision request).
func promptText(req llm.ChatRequest) string {
	var b strings.Builder
	for _, m := range req.Messages {
		b.WriteString(m.Content)
		b.WriteByte('\n')
		for _, p := range m.Parts {
			if p.Kind == llm.PartText {
				b.WriteString(p.Text)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

// Complete implements llm.ChatClient. response.Engine and internal/kbimport
// both call only Complete, never Stream (see their own doc comments) — this
// is the one method that must tell their two distinct JSON contracts apart.
func (c *Client) Complete(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	c.record(req, false)
	text := promptText(req)
	switch {
	// "reply_text" is aiprompt v7's own response-contract field name,
	// spelled out literally in every shop-kb-v7*.txt frame's instructions
	// (see aiprompt/frames) — a reliable, distinctive marker that a kbimport
	// synthesis prompt never contains.
	case strings.Contains(text, "reply_text"):
		return customerResponse(text), nil
	// `"unmapped"` is kbimport's own output contract's third top-level key
	// (internal/kbimport/prompt.go's outputContractDescription) — equally
	// distinctive against the v7 contract above.
	case strings.Contains(text, `"unmapped"`):
		return kbImportResponse(), nil
	default:
		return proseResponse(text), nil
	}
}

// Stream implements llm.StreamClient. Only internal/chat's KB-chat assistant
// ever calls this (it type-asserts for the interface and falls back to
// Complete otherwise) — so this always answers with the same prose Complete
// falls back to, chunked word-by-word so a caller exercising incremental
// rendering (SSE deltas) sees more than one delta, and stops early on
// context cancellation like a real streaming provider would.
func (c *Client) Stream(ctx context.Context, req llm.ChatRequest, onDelta func(string)) (llm.ChatResponse, error) {
	c.record(req, true)
	resp := proseResponse(promptText(req))
	var sent strings.Builder
	for _, chunk := range strings.SplitAfter(resp.Text, " ") {
		if chunk == "" {
			continue
		}
		select {
		case <-ctx.Done():
			return llm.ChatResponse{Text: sent.String(), FinishReason: "cancelled"}, ctx.Err()
		default:
		}
		if onDelta != nil {
			onDelta(chunk)
		}
		sent.WriteString(chunk)
	}
	return resp, nil
}

// customerResponse produces a valid aiprompt v7 Response body (see
// aiprompt.Response / ValidateResponseV7): reply_language "ru", no
// escalation, and — deliberately — no {{table.ref.column}} catalog
// placeholders and an empty media_files_to_send, since a generic mock has
// no real catalog/media tokens to reference correctly and either would fail
// ValidateResponseV7's own catalog cross-check.
func customerResponse(prompt string) llm.ChatResponse {
	lastMsg := prompt
	if idx := strings.LastIndex(prompt, "Клиент пишет: "); idx != -1 {
		lastMsg = prompt[idx+len("Клиент пишет: "):]
	}
	lower := strings.ToLower(lastMsg)
	isZarina := strings.Contains(prompt, "Zarina") || strings.Contains(prompt, "zarina") || strings.Contains(prompt, "перманент") || strings.Contains(prompt, "ботокс") || strings.Contains(prompt, "филлер")
	isLion := strings.Contains(prompt, "Lion Barbershop") || strings.Contains(prompt, "барбер")
	reply := "Здравствуйте! Чем я могу вам помочь?"

	if isZarina {
		switch {
		case strings.Contains(lower, "запис") || strings.Contains(lower, "забронир") || strings.Contains(lower, "подтверд") || strings.Contains(lower, "суббот") || strings.Contains(lower, "время"):
			reply = "Записаться к эксперту Зарине на удобный день и время можно напрямую через наш WhatsApp по номеру +7 702 331 72 69. Мастер согласует свободное окно и подтвердит вашу запись!"
		case strings.Contains(lower, "адрес") || strings.Contains(lower, "где") || strings.Contains(lower, "город") || strings.Contains(lower, "находит"):
			reply = "Наша студия находится по адресу: г. Шымкент, ул. Еримбетова, 32, Салон «Бонита», 2 этаж (Zarina PM Studio). Также ведется регулярная запись на процедуры в г. Астана!"
		case strings.Contains(lower, "ботокс"):
			reply = "Здравствуйте! Ботокс лица (верхняя треть: 4 зоны — лоб, межбровка, глаза, нос) стоит 40 000 ₸. Процедуру проводит эксперт Зарина с 10-летним стажем сертифицированными препаратами."
		case strings.Contains(lower, "филлер") || strings.Contains(lower, "пластик") || strings.Contains(lower, "увеличен"):
			reply = "Здравствуйте! Контурная пластика губ проводится сертифицированными препаратами: Revolax Deep (1 ml) — 45 000 ₸, E.P.T.Q (1 ml) — 45 000 ₸, Belotero (0.6 ml) — 80 000 ₸, Belotero (1 ml) — 85 000 ₸, Stylage (1 ml) — 85 000 ₸, Juvederm (1 ml) — 90 000 ₸. Удаление филлера — 25 000 ₸."
		case strings.Contains(lower, "удал"):
			reply = "Здравствуйте! В нашей студии доступно лазерное удаление старого татуажа (10 000 ₸) и удаление ремувером (20 000 ₸), а также удаление филлера губ (25 000 ₸)."
		case strings.Contains(lower, "губ") || strings.Contains(lower, "бров") || strings.Contains(lower, "стои") || strings.Contains(lower, "цен") || strings.Contains(lower, "техник") || strings.Contains(lower, "перманент"):
			reply = "Здравствуйте! В «Zarina PM Studio» мастер Зарина со стажем 10 лет выполняет перманентный макияж бровей (пудровое напыление 35 000 ₸, волосковая техника 40 000 ₸) и губ (акварельная техника Lifting Effect 35 000 ₸). При записи сразу на 2 зоны действует скидка 5 000 ₸!"
		default:
			reply = "Здравствуйте! Добро пожаловать в студию перманентного макияжа и косметологии «Zarina PM Studio». Чем могу помочь вам сегодня?"
		}
	} else if isLion {
		reply = "Спасибо за обращение в барбершоп «Lion Barbershop»! Вы можете записаться на стрижку или уход онлайн по ссылке: {{contact.main.booking}}"
		switch {
		case strings.Contains(lower, "забронировал") || strings.Contains(lower, "записался") || strings.Contains(lower, "до встречи") || (strings.Contains(lower, "спасибо") && (strings.Contains(lower, "понял") || strings.Contains(lower, "отлично"))):
			reply = "Отлично! Будем рады видеть вас в Lion Barbershop! Если возникнут вопросы, наш телефон: {{contact.main.phone}}. До встречи!"
		case strings.Contains(lower, "нурлан") || strings.Contains(lower, "касымов"):
			if strings.Contains(lower, "вторник") || strings.Contains(lower, "15:00") || strings.Contains(lower, "запишите") || strings.Contains(lower, "забронир") {
				reply = "Во вторник топ-барбер Нурлан Касымов работает: {{specialist.nurlan-kasymov.schedule_tue}}. Обратите внимание: точное время бронируется клиентами онлайн в реальном времени. Пожалуйста, откройте ссылку и выберите удобный свободный слот: {{specialist.nurlan-kasymov.booking}}"
			} else {
				reply = "Топ-барбер Нурлан Касымов принимает по графику: {{specialist.nurlan-kasymov.schedule}}. Записаться к нему можно по ссылке: {{specialist.nurlan-kasymov.booking}}"
			}
		case strings.Contains(lower, "арман"):
			reply = "Барбер Арман Ибраев принимает по графику: {{specialist.arman-ibraev.schedule}}. Записаться к нему можно по ссылке: {{specialist.arman-ibraev.booking}}"
		case strings.Contains(lower, "данияр"):
			reply = "Барбер-стилист Данияр Алиев принимает по графику: {{specialist.daniyar-aliev.schedule}}. Записаться к нему можно по ссылке: {{specialist.daniyar-aliev.booking}}"
		case strings.Contains(lower, "мастер") || strings.Contains(lower, "барбер") || strings.Contains(lower, "специалист"):
			reply = "У нас работают 3 барбера: топ-барбер Нурлан Касымов (график: {{specialist.nurlan-kasymov.schedule}}), Арман Ибраев и Данияр Алиев. Выбрать мастера и удобное время можно по ссылке: {{contact.main.booking}}"
		case strings.Contains(lower, "окн") || strings.Contains(lower, "свободн") || strings.Contains(lower, "занят") || strings.Contains(lower, "слот"):
			reply = "Все актуальные свободные слоты и окна отображаются в реальном времени на странице онлайн-записи. Если выбранное время занято, система сразу покажет соседние доступные окна: {{contact.main.booking}}"
		case strings.Contains(lower, "отец") || strings.Contains(lower, "сын") || strings.Contains(lower, "детск") || strings.Contains(lower, "мальчик"):
			reply = "Здравствуйте! Для отцов с сыновьями у нас действует комплекс «Отец + сын» {{service.haircut-father-son.price}} ({{service.haircut-father-son.duration}}). Отдельно детская стрижка для мальчика стоит {{service.haircut-kids.price}} ({{service.haircut-kids.duration}}). Онлайн-запись: {{contact.main.booking}}"
		case strings.Contains(lower, "королевск") || strings.Contains(lower, "брить") || strings.Contains(lower, "бород"):
			reply = "Здравствуйте! Стрижка + стрижка бороды стоит {{service.haircut-combo-beard.price}} ({{service.haircut-combo-beard.duration}}), королевское бритьё — {{service.beard-royal.price}} ({{service.beard-royal.duration}}), а стрижка бороды — {{service.beard-trim.price}} ({{service.beard-trim.duration}}). Онлайн-запись: {{contact.main.booking}}"
		case strings.Contains(lower, "мужск") || strings.Contains(lower, "стрижк") || strings.Contains(lower, "машинк") || strings.Contains(lower, "окантовк"):
			reply = "Здравствуйте! Мужская стрижка в Lion Barbershop стоит {{service.haircut-men.price}} ({{service.haircut-men.duration}}). Стрижка под машинку — {{service.haircut-clipper.price}} ({{service.haircut-clipper.duration}}), окантовка — {{service.hair-edging.price}} ({{service.hair-edging.duration}}). Онлайн-запись: {{contact.main.booking}}"
		case strings.Contains(lower, "маск") || strings.Contains(lower, "пилинг") || strings.Contains(lower, "скраб") || strings.Contains(lower, "воск") || strings.Contains(lower, "уход"):
			reply = "Здравствуйте! Из уходовых процедур мы предлагаем: Black mask/Чёрная маска {{service.black-mask.price}} ({{service.black-mask.duration}}), скраб лица/пилинг {{service.face-scrub.price}} ({{service.face-scrub.duration}}) и воск {{service.wax-barber.price}} ({{service.wax-barber.duration}}). Онлайн-запись: {{contact.main.booking}}"
		case strings.Contains(lower, "седин") || strings.Contains(lower, "окрашиван") || strings.Contains(lower, "камуфляж"):
			reply = "Здравствуйте! Камуфляж седины волос стоит {{service.hair-camo-grey.price}} ({{service.hair-camo-grey.duration}}), а камуфляж бороды — {{service.beard-camo.price}} ({{service.beard-camo.duration}}). Онлайн-запись: {{contact.main.booking}}"
		case strings.Contains(lower, "запис") || strings.Contains(lower, "онлайн") || strings.Contains(lower, "где") || strings.Contains(lower, "адрес") || strings.Contains(lower, "график"):
			reply = "Мы работаем по графику: {{contact.main.schedule}}. Записаться к мастерам онлайн можно по ссылке: {{contact.main.booking}}"
		}
	}

	raw, _ := json.Marshal(map[string]any{
		"reply_text":          reply,
		"reply_language":      "ru",
		"media_files_to_send": []string{},
		"escalate":            false,
		"escalation_reason":   "",
		"confidence":          0.95,
	})
	return llm.ChatResponse{
		Text: string(raw), FinishReason: "stop",
		PromptTokens: estimateTokens(prompt), CompletionTokens: estimateTokens(string(raw)),
	}
}

// kbImportResponse produces a syntactically valid kbimport modelOutput body
// (internal/kbimport/synth.go's modelOutput: Calls/Notes/Unmapped) — zero
// calls is a legitimate, terminal answer kbimport's own retry logic already
// handles (see internal/kbimport's own doc comment on the corrective re-ask
// loop), so this never open-loops the mock.
func kbImportResponse() llm.ChatResponse {
	raw, _ := json.Marshal(map[string]any{
		"calls":    []any{},
		"notes":    "mock-externals: no changes synthesized",
		"unmapped": []string{},
	})
	return llm.ChatResponse{Text: string(raw), FinishReason: "stop", CompletionTokens: estimateTokens(string(raw))}
}

// proseResponse is the KB-chat assistant's answer shape: plain prose, no
// JSON contract expected.
func proseResponse(prompt string) llm.ChatResponse {
	const text = "Это тестовый ответ ассистента базы знаний в режиме mock-externals."
	return llm.ChatResponse{Text: text, FinishReason: "stop", PromptTokens: estimateTokens(prompt), CompletionTokens: estimateTokens(text)}
}

// estimateTokens is a rough, deterministic stand-in for a real tokenizer —
// good enough for the load harness's own token-count reporting, which only
// needs plausible non-zero numbers, never exact ones.
func estimateTokens(s string) int {
	if n := len(s) / 4; n > 0 {
		return n
	}
	return 1
}
