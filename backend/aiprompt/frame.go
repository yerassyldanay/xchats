package aiprompt

import _ "embed"

// PromptRefShopKBV4 identifies the frame graded in the schema_kb_v1 eval pipeline
// (evals/scenarios/shop-kb-v1) up to 2026-08. SUPERSEDED IN PRODUCTION by
// PromptRefShopKBV5 below, which adds the tariff block v4 structurally could not
// carry — but kept, embedded and pinned, because it is the ref stamped on every
// draft record produced before that switch: reproducing one means rendering the
// frame it actually used, not today's.
const PromptRefShopKBV4 = "shop-kb@v4"

// frameShopKBV4RU is the v4 frame text. frame_test.go pins its SHA256 so any
// accidental drift fails loudly rather than silently reinterpreting historical
// drafts.
//
// It was a byte-exact copy of evals/scenarios/shop-kb-v1/frame-ru.txt while v4
// was the shipped frame; that eval copy now tracks v5 (see the sync test in
// evals/harness/aiprompt_sync_test.go, which follows whichever frame production
// actually renders). v4's own graded result is a historical fact about these
// bytes, which the hash pin is what preserves.
//
//go:embed frames/shop-kb-v4-ru.txt
var frameShopKBV4RU string

// FrameShopKBV4RU returns the v4 frame text, ready for RenderPrompt.
func FrameShopKBV4RU() string {
	return frameShopKBV4RU
}

// PromptRefShopKBV4TG identifies the Telegram variant of the same frame.
const PromptRefShopKBV4TG = "shop-kb@v4-tg"

// frameShopKBV4TGRU is the Telegram-channel variant: byte-identical to
// frameShopKBV4RU except its first line, which no longer calls the assistant a
// «WhatsApp-ассистент». Every rule, block contract and output shape is the
// same, so the schema_kb_v1 eval grades the shared body once — a divergence in
// anything but that persona line is a bug, and frame_test.go pins both hashes
// so it cannot happen silently.
//
//go:embed frames/shop-kb-v4-tg-ru.txt
var frameShopKBV4TGRU string

// FrameShopKBV4TGRU returns the Telegram frame text, ready for RenderPrompt.
func FrameShopKBV4TGRU() string {
	return frameShopKBV4TGRU
}

// PromptRefShopKBV5 identifies the v5 frame — v4 plus the ТАРИФЫ block
// (%%TARIFFS%%, rendered by renderTariffs) and the rule edits that block needs.
// Cut as a NEW version rather than an edit to v4 because frame_test.go's hash
// pin says so in as many words: an in-place edit would silently reinterpret
// every draft already stamped "shop-kb@v4".
//
// Why it exists: until v5 the whole ai_tariffs table was unreachable by the
// model. BuildCatalog built tariff facts and media all along, but no renderer
// emitted them and no v4 slot could carry them, so an operator who priced a
// tariff in its typed columns — the exact thing the publish gate pushes them
// toward — watched the assistant escalate "нет этой информации" instead of
// quoting the price it had been given.
//
// v4 stays embedded and pinned directly below this: it is the frame every
// existing draft record names, and reproducing one means rendering the frame
// it actually used.
//
// GRADING STATUS: v5 carries v4's whole rule body unchanged, and
// evals/scenarios/shop-kb-v1 tracks it (aiprompt_sync_test.go pins the two
// byte-identical) with tariff fixtures added — but the schema_kb_v1 pipeline
// has not been RUN against it yet. Until it has, the tariff block's behavior
// is covered by unit tests and the frame's own rules, not by a graded eval.
const PromptRefShopKBV5 = "shop-kb@v5"

//go:embed frames/shop-kb-v5-ru.txt
var frameShopKBV5RU string

// FrameShopKBV5RU returns the v5 frame text, ready for RenderPrompt.
func FrameShopKBV5RU() string {
	return frameShopKBV5RU
}

// PromptRefShopKBV5TG identifies the Telegram variant of the v5 frame, related
// to it exactly as V4TG is to V4: byte-identical but for the persona line.
const PromptRefShopKBV5TG = "shop-kb@v5-tg"

//go:embed frames/shop-kb-v5-tg-ru.txt
var frameShopKBV5TGRU string

// FrameShopKBV5TGRU returns the v5 Telegram frame text, ready for RenderPrompt.
func FrameShopKBV5TGRU() string {
	return frameShopKBV5TGRU
}

// PromptRefShopKBV6 identifies the v6 frame — virtual fact columns
// (0017_kb_virtual_facts): v5's whole rule body plus availability_status
// (in_stock/preorder/on_demand fully visible, unavailable name-only,
// replacing the old in_stock boolean split), product/tariff prose
// (brand/advantages/disadvantages/best_for/not_for/availability_note/
// installation_terms/warranty_terms), the organization-wide tariff_info
// singleton, and every entity's seller-authored virtual facts rendered
// beside their instruction (new rule 8) — see prompt.go's
// SlotProductsAvailable/SlotProductsUnavailable/SlotTariffCatalog/
// SlotTariffInfo doc comment for the renderer side of this.
//
// Cut as a NEW version, never an edit to v5, for the same reason v5 was
// cut instead of editing v4: frame_test.go's hash pin says so in as many
// words, and v5 is the frame every draft record stamped "shop-kb@v5"
// actually used — reproducing one means rendering the frame it actually
// used, not today's. v5 (and v4) stay embedded and pinned exactly as
// before this file.
//
// GRADING STATUS: like v5 was at the time it shipped, v6 is covered by
// unit tests and the frame's own rules, not yet by a graded eval run.
const PromptRefShopKBV6 = "shop-kb@v6"

//go:embed frames/shop-kb-v6-ru.txt
var frameShopKBV6RU string

// FrameShopKBV6RU returns the v6 frame text, ready for RenderPrompt.
func FrameShopKBV6RU() string {
	return frameShopKBV6RU
}

// PromptRefShopKBV6TG identifies the Telegram variant of the v6 frame,
// related to it exactly as V5TG is to V5: byte-identical but for the
// persona line.
const PromptRefShopKBV6TG = "shop-kb@v6-tg"

//go:embed frames/shop-kb-v6-tg-ru.txt
var frameShopKBV6TGRU string

// FrameShopKBV6TGRU returns the v6 Telegram frame text, ready for RenderPrompt.
func FrameShopKBV6TGRU() string {
	return frameShopKBV6TGRU
}
