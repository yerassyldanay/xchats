package main

import "xchats-evals-harness/internal/provenance"

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
//     list. An empty (but present) list means "no numbers may appear at all". This is
//     a PRECISION check only — it says nothing about whether a real visible number was
//     dropped.
//   - RequiredNumbers: the RECALL complement to AllowedNumbers (Phase 0.4 of the
//     language/extraction plan) — every entry here must appear somewhere in the
//     model's text fields, or the case fails. Optional (nil/empty skips the check);
//     use it for the business-relevant figures a multi-panel image must not let a
//     model silently omit (a price, an order number, a phone number) — not every
//     visible number needs to be required, only the ones that matter if dropped.
//   - ForbidCurrency: an extra check that no currency symbol/word appears — for photos
//     that visibly show no price.
type ExtractCase struct {
	ID   string `yaml:"id"`
	File string `yaml:"file"`
	// Kind selects the preprocess() pipeline (see preprocess.go): "" and "image" both
	// mean the current image path (downscale + JPEG re-encode). This is the seam a
	// future pdf/audio/url case type plugs into — an unrecognized kind is a hard error,
	// never a silent fall-through to the image path.
	Kind                string            `yaml:"kind"`
	Fields              map[string]string `yaml:"fields"`
	TextContainsAll     []string          `yaml:"text_contains_all"`
	IdentifyContainsAll []string          `yaml:"identify_contains_all"`
	IdentifyContainsAny []string          `yaml:"identify_contains_any"`
	AllowedNumbers      []string          `yaml:"allowed_numbers"`
	RequiredNumbers     []string          `yaml:"required_numbers"`
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

// extractRunResult is one (case, model, prompt) attempt — saved to disk verbatim
// (JSON) so a run is fully inspectable after the fact, and folded into EXTRACT.md's
// summary table. Prompt/Preprocessor are additive fields (new runs only) — a legacy
// extractRunResult JSON without them still parses fine, with both left at zero value.
type extractRunResult struct {
	CaseID       string               `json:"case_id"`
	Model        string               `json:"model"`
	Prompt       provenance.PromptRef `json:"prompt"`
	Preprocessor string               `json:"preprocessor,omitempty"`
	Raw          string               `json:"raw,omitempty"`
	Error        string               `json:"error,omitempty"`
	ParseError   string               `json:"parse_error,omitempty"`
	Parsed       *ExtractionResult    `json:"parsed,omitempty"`
	Checks       []CheckResult        `json:"checks,omitempty"`
	Usage        orUsage              `json:"usage"`
	Cost         float64              `json:"cost_usd_estimate"`
	CostBasis    string               `json:"cost_basis"`
}

// extractMaxTokens is the default completion budget for an extraction call — generous
// on purpose. models.yaml's max_tokens (500) is tuned for short customer-chat replies;
// a full-text transcription JSON needs much more room, and a truncated response is a
// parse failure with no useful signal (verified: several models cut off mid-sentence
// at exactly 500 completion tokens against these test images).
const extractMaxTokens = 2000
