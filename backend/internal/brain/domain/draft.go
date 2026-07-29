package domain

// RawDraft is the model's JSON output (the emit_draft tool arguments), parsed
// defensively before post-processing. See docs/02 · JSON output contract.
type RawDraft struct {
	ReplyText         string         `json:"reply_text"`
	ReplyLanguage     string         `json:"reply_language"`
	ProfilePatch      map[string]any `json:"profile_patch"`
	SuggestedCallback *Callback      `json:"suggested_callback"`
	SuggestedStatus   *StageWrapper  `json:"suggested_status"`
	Confidence        float64        `json:"confidence"`
	Escalate          bool           `json:"escalate"`
	EscalationReason  string         `json:"escalation_reason"`
}

// StageWrapper matches the contract's suggested_status object {stage}; the Draft
// flattens it to a string (docs/02 cosmetic note).
type StageWrapper struct {
	Stage string `json:"stage"`
}

// Callback is an optional follow-up suggestion.
type Callback struct {
	DueAt string `json:"due_at"`
	Note  string `json:"note"`
}

// Draft is the fully post-processed result: final reply text (prices injected)
// and structured side-data. The caller maps it to the channel's draft rows
// (docs/02 · the contract).
type Draft struct {
	ReplyText         string
	ProfilePatch      map[string]any
	SuggestedStatus   string
	SuggestedCallback *Callback
	Confidence        float64
	Escalate          bool
	EscalationReason  string

	// PricingError is set when price-token rendering failed; the caller posts a
	// "check pricing manually" note rather than shipping a half-rendered price.
	PricingError bool
}
