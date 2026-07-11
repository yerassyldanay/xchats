package main

// ExtractCasesFile is extract/cases.yaml — one entry per test file, each declaring what
// a correct extraction must contain. Requirements are DATA (not Go code) on purpose: this
// is how eval cases are meant to grow (audio, pdf, more images) without touching the
// runner, matching this playground's existing data.yaml-drives-everything convention.
type ExtractCasesFile struct {
	Cases []ExtractCase `yaml:"cases"`
}

// ExtractCase is one graded extraction case. File is relative to cases.yaml's own
// directory. Checks are grouped by what they test against:
//   - Fields: exact-match checks against named top-level ExtractionResult fields.
//   - TextContainsAll: substrings that must all appear in ExtractedText (normalized).
//   - IdentifyContainsAll / IdentifyContainsAny: substrings checked against
//     Summary+" "+RelatesToHint (normalized) — for files with no exact facts to OCR,
//     where "did the model understand what this is" is the only meaningful check.
//   - AllowedNumbers: the whitelist for the invented-number check (below) — every
//     number-like run found anywhere in the model's JSON text fields must be in this
//     list. An empty (but present) list means "no numbers may appear at all".
//   - ForbidCurrency: an extra check that no currency symbol/word appears — for photos
//     that visibly show no price.
type ExtractCase struct {
	ID                  string            `yaml:"id"`
	File                string            `yaml:"file"`
	Fields              map[string]string `yaml:"fields"`
	TextContainsAll     []string          `yaml:"text_contains_all"`
	IdentifyContainsAll []string          `yaml:"identify_contains_all"`
	IdentifyContainsAny []string          `yaml:"identify_contains_any"`
	AllowedNumbers      []string          `yaml:"allowed_numbers"`
	ForbidCurrency      bool              `yaml:"forbid_currency"`
}

// ExtractionResult is the JSON shape every vision call must return. This is the pass-1
// contract eval 2 (extracted info -> ai_* draft schema) will consume — general TEXT, not
// a structured facts array, because a fixed facts schema can't flex across arbitrary
// product/tariff/contact shapes (that structuring happens in eval 2, with KB context).
type ExtractionResult struct {
	ContentKind          string `json:"content_kind"`          // infographic | product_photo | screenshot | document | other
	Summary              string `json:"summary"`               // what this file is, 1-2 sentences
	ExtractedText        string `json:"extracted_text"`        // ALL visible text/numbers, transcribed exactly
	Language             string `json:"language"`              // ru | kk | en | mixed | none
	VisibilitySuggestion string `json:"visibility_suggestion"` // visible | invisible
	MediaRoleHint        string `json:"media_role_hint"`       // gallery | certificate | pricing | instruction | spec_plate | document | none
	RelatesToHint        string `json:"relates_to_hint"`       // free text, or empty
}

// CheckResult is one named pass/fail with enough detail to debug a failure without
// re-running the call (the raw output is saved alongside anyway).
type CheckResult struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail,omitempty"` // populated on failure
}

func allChecksPass(checks []CheckResult) bool {
	for _, c := range checks {
		if !c.Pass {
			return false
		}
	}
	return len(checks) > 0
}

// extractRunResult is one (case, model) attempt — saved to disk verbatim (JSON) so a run
// is fully inspectable after the fact, and folded into EXTRACT.md's summary table.
type extractRunResult struct {
	CaseID     string            `json:"case_id"`
	Model      string            `json:"model"`
	Raw        string            `json:"raw,omitempty"`
	Error      string            `json:"error,omitempty"`
	ParseError string            `json:"parse_error,omitempty"`
	Parsed     *ExtractionResult `json:"parsed,omitempty"`
	Checks     []CheckResult     `json:"checks,omitempty"`
	Usage      orUsage           `json:"usage"`
	Cost       float64           `json:"cost_usd_estimate"`
	CostBasis  string            `json:"cost_basis"`
}

// extractMaxTokens is the default completion budget for an extraction call — generous
// on purpose. models.yaml's max_tokens (500) is tuned for short customer-chat replies;
// a full-text transcription JSON needs much more room, and a truncated response is a
// parse failure with no useful signal (verified: several models cut off mid-sentence
// at exactly 500 completion tokens against these test images).
const extractMaxTokens = 2000

// extractSystemPrompt is the pass-1 extraction prompt under test. It intentionally
// mirrors DECISIONS.md's pass-1 contract: describe the file, transcribe ALL visible
// text/numbers into one text field (never invent structured facts that can't
// generalize across product/tariff/contact shapes), and never guess a number that
// isn't visible. Revised after a live eval round surfaced three concrete failure
// modes: a self-check pass + "remove rather than guess" instruction (models were
// fabricating plausible specs / drifting a repeated digit); a Cyrillic/Latin
// look-alike caution (several models misread Cyrillic Р/С/О/Т as Latin P/C/O/T);
// and a more mechanical visibility_suggestion rule (models disagreed on whether a
// raw website screenshot counts as "visible" — now a flat rule, not a judgment call).
// "tutorial" was added to content_kind for designed multi-step how-to graphics, which
// don't fit "infographic" (no data/pricing) or "screenshot" (not raw/single-page).
const extractSystemPrompt = `You are the first-pass extractor in a knowledge-base builder for a small business's WhatsApp sales assistant. You are shown ONE file (image). Your only job is to get everything a human would see out of it, as text — you do NOT decide how it becomes structured business data (that happens later, with full business context you don't have here).

Rules:
- Transcribe ALL visible text and numbers into extracted_text, EXACTLY as shown (keep the original language, spacing of thousands, currency symbols, punctuation). Do not summarize or translate it away — extracted_text is meant to be complete. If the same value (a price, an order number) appears more than once, transcribe it the same way every time — do not let it drift on a later occurrence.
- NEVER invent, guess, or "helpfully fill in" a number, price, or spec that is not visibly printed in the file. If nothing exact is visible, extracted_text should simply have no numbers in it. Before you finalize your answer, re-check every single digit you wrote against the image one more time, character by character. If you cannot re-verify a specific digit with confidence, remove that whole value rather than keep a guess — a missing value is safe, a wrong one is not.
- Cyrillic and Latin share several look-alike letters at small sizes (Cyrillic Р/С/О/Т/Н/В/А/Е/К/М/Х look like Latin P/C/O/T/H/B/A/E/K/M/X). Judge each word by its LANGUAGE CONTEXT, not by which alphabet a single glyph resembles — a word sitting in a Russian sentence is Russian, even if a letter looks Latin-shaped.
- summary: 1-2 plain sentences saying what kind of file this is and what it shows.
- content_kind: one of "infographic" (a designed graphic showing data/pricing), "tutorial" (a designed graphic showing numbered steps or a how-to flow, often with UI screenshots inside it), "product_photo" (a photo of a physical item, no data card), "screenshot" (a single, raw captured web page or app screen — not redesigned into panels/steps), "document" (a scanned page of a document), "other".
- language: the dominant language of the visible text — "ru", "kk", "en", "mixed", or "none" if there is no readable text.
- visibility_suggestion: "invisible" for ANY raw, single screen-capture of a website or app (yours or anyone else's) — a customer should never receive a raw browser/app screenshot as a file, even if its content is accurate. "visible" for content specifically produced to be shared: a clean product photo, a designed pricing card, a designed tutorial/how-it-works graphic (even one that shows UI mockups INSIDE a designed layout — that's still "visible", since the graphic itself was produced to be sent, unlike a raw screenshot).
- media_role_hint: one of "gallery", "certificate", "pricing", "instruction", "spec_plate", "document", "none".
- relates_to_hint: a short free-text guess at what product/topic/tariff this file is about, or empty if you cannot tell.

Answer with ONLY a JSON object, no markdown fence, with exactly these fields: content_kind, summary, extracted_text, language, visibility_suggestion, media_role_hint, relates_to_hint.`
