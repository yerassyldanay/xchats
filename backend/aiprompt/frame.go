package aiprompt

import _ "embed"

// PromptRefShopKBV4 identifies the evaluated shop-kb-v1 frame this package embeds —
// the exact frame graded in the schema_kb_v1 eval pipeline (evals/scenarios/shop-kb-v1),
// carried into production so a boot log or draft record can name precisely which
// prompt version produced it.
const PromptRefShopKBV4 = "shop-kb@v4"

// frameShopKBV4RU is a byte-exact copy of evals/scenarios/shop-kb-v1/frame-ru.txt — the
// frame the schema_kb_v1 eval pipeline actually graded. frame_test.go pins its SHA256 so
// any accidental drift (a stray edit here, or the eval copy changing without this one)
// fails loudly instead of silently shipping an unevaluated prompt; evals/harness's own
// sync test (canary_sync_test.go doctrine) pins the reverse direction — that the eval
// copy stays byte-identical to this one.
//
//go:embed frames/shop-kb-v4-ru.txt
var frameShopKBV4RU string

// FrameShopKBV4RU returns the evaluated shop-kb-v1 frame text, ready for RenderPrompt.
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
