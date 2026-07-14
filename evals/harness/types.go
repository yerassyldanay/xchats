package main

// ScenarioConfig is scenario.yaml — the meta file naming which other files make up one
// imitated product version and which real response contract (if any) it matches.
//
// TopicFormat controls how each KNOWLEDGE BASE entry is written; substituted placeholders
// are {ref} {lang} {title} {keywords} {body} — e.g. "# topic: {ref} ({lang})\nkeywords:
// {keywords}\n{body}" or "# {ref} — {title} ({keywords})\n{body}". Different scenarios in
// this playground use different house styles on purpose (that IS one of the things being
// imitated), so this is data, not a hardcoded format.
type ScenarioConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Frame       string `yaml:"frame"`    // path to frame.txt, relative to the scenario dir
	Data        string `yaml:"data"`     // path to data.yaml (may point into another scenario dir)
	Tests       string `yaml:"tests"`    // path to tests.yaml
	Contract    string `yaml:"contract"` // "asset_refs" | "attach_groups" — which media field the model must return
	TopicFormat string `yaml:"topic_format"`
	// Limits caps how many rows of a named fact_tables table (keyed by Table, e.g.
	// "product") this scenario renders — everything else about data.yaml is used as-is.
	// Lets several scenario.yaml files point at ONE shared, large data.yaml and each
	// imitate a different catalog size (10 vs 20 vs 30 products) without duplicating the
	// pool or hand-truncating it out of sync with the grading catalog.
	Limits map[string]int `yaml:"limits"`

	// Setup, PromptRef, and Experiment are the eval-comparison-UI metadata (all
	// optional; every field empty is a legacy/unannotated scenario, handled by
	// fallbacks in viewmodel.go's enrichScenarioExecutions — nothing regresses for
	// data that predates these fields).
	//
	// Setup is the comparison COLUMN in the results matrix — a named strategy, not
	// necessarily one prompt file. A routed strategy that dispatches to a Kazakh frame
	// for Kazakh customers and a Russian frame for everyone else is ONE setup
	// ("lang-v4-routed") realized by TWO scenario dirs (lang-canary-v4-kk and
	// lang-canary-v4-ru), each with a DIFFERENT PromptRef — the matrix must show one
	// column for the strategy, not silently split it into two. Falls back to the
	// scenario's own Name when empty, so an unannotated scenario still gets its own
	// column (today's behavior, unchanged).
	Setup string `yaml:"setup,omitempty"`
	// PromptRef identifies the ACTUAL frame file this scenario renders
	// (ParsePromptSpec-valid: "<name>@v<N>", e.g. "lang-kk@v4") — distinct from Setup
	// specifically so the drill-down can show "this execution used lang-kk@v4" even
	// when its Setup column is the shared "lang-v4-routed". Falls back to the
	// scenario's own Name (unversioned) when empty.
	PromptRef string `yaml:"prompt_ref,omitempty"`
	// Experiment is the comparison GROUP — only scenarios sharing one Experiment value
	// may be pooled into the same matrix. Left empty, a scenario is never
	// auto-compared against anything (safer default than accidentally merging an
	// unrelated shop-scale run into a language bake-off's matrix).
	Experiment string `yaml:"experiment,omitempty"`
}

// Data is data.yaml — the single source of truth a scenario's prompt is rendered from.
// Nothing in render.go ever hand-types a FACTS line or a media group name; both are
// always derived from these rows, so a scenario's prompt can never silently drift from
// its own data (the exact bug class that motivated this rewrite).
type Data struct {
	Topics     []Topic     `yaml:"topics"`
	FactTables []FactTable `yaml:"fact_tables"`
	Assets     []Asset     `yaml:"assets"` // old-style: one row per media file, individual refs
}

// Topic is one KNOWLEDGE BASE entry. Media (if any) becomes a group named "{ref}.{field}".
type Topic struct {
	Ref      string       `yaml:"ref"`
	Title    string       `yaml:"title"`
	Lang     string       `yaml:"lang"`
	Keywords string       `yaml:"keywords"`
	Body     string       `yaml:"body"`
	Media    []MediaGroup `yaml:"media"`
}

// FactTable is one typed fact table (product, tariff, policy, contact, ...).
//
// LabelFormat controls how a FACTS row's "meaning" column reads; placeholders are
// {display_name} and {field_label} — e.g. "Товар «{display_name}» — {field_label}" or
// just "{field_label}" (some scenarios show no per-row prefix at all). Defaults to
// "{display_name} — {field_label}" if empty.
//
// DescriptionsLabel names the optional prose block (e.g. "ТОВАРЫ") this table's rows'
// Description fields render into; leave empty to skip the block for this table entirely
// (some scenarios don't surface product prose to the model at all).
type FactTable struct {
	Table             string      `yaml:"table"`
	LabelFormat       string      `yaml:"label_format"`
	Fields            []FieldSpec `yaml:"fields"` // ORDERED — controls FACTS line order
	DescriptionsLabel string      `yaml:"descriptions_label"`
	Rows              []FactRow   `yaml:"rows"`
}

// FieldSpec names one fact field and the human label shown for it in the FACTS block.
type FieldSpec struct {
	Name      string `yaml:"name"`       // column name, e.g. "delivery_in_days" -> token {{policy.main.delivery_in_days}}
	Label     string `yaml:"label"`      // e.g. "срок доставки, в днях"
	ValueKind string `yaml:"value_kind"` // e.g. "money_display", "number_range", "percent_number"
	UnitRU    string `yaml:"unit_ru"`    // model-added unit for Russian, if the value is unitless
	UnitKK    string `yaml:"unit_kk"`    // model-added unit for Kazakh, if the value is unitless
}

// FactRow is one row: Values holds this row's fact values keyed by field name (tokenized
// per the table's Fields list); Media holds grouped files (tokenized as a group, not a
// single value).
type FactRow struct {
	Ref         string            `yaml:"ref"`
	DisplayName string            `yaml:"display_name"`
	Description string            `yaml:"description"` // word-bearing prose, NEVER tokenized
	Values      map[string]string `yaml:"values"`
	Media       []MediaGroup      `yaml:"media"`
}

// MediaGroup is one group of files attached to a row or topic, becoming the token
// "{owner_ref}.{field}". Description is the selection cue shown in the MEDIA block —
// required, since it's the model's only signal for which group fits a request (mirrors
// the real product's asset-description rule).
type MediaGroup struct {
	Field       string   `yaml:"field"` // "images" | "videos" | "certificates" | "documents" | ...
	Files       []string `yaml:"files"`
	Description string   `yaml:"description"`
}

// Asset is one row of an OLD-style, per-file media table (no grouping — the model
// selects an individual ref by reading its description).
type Asset struct {
	Ref         string `yaml:"ref"`
	Kind        string `yaml:"kind"`
	Topic       string `yaml:"topic"`
	Description string `yaml:"description"`
}

// TestsFile is tests.yaml — an include of shared question banks plus scenario-only tests.
type TestsFile struct {
	Include []string   `yaml:"include"` // paths to common/*.yaml banks, relative to evals/
	Tests   []TestCase `yaml:"tests"`   // scenario-only, appended after includes
}

// TestCase is one customer message plus the per-test checks its answer must pass. These
// run ON TOP OF the universal, always-on checks in judge.go (every token must resolve or
// the draft is BLOCKED; every media entry must exist; no invented digits pre-injection).
//
// Requires is AND-of-OR: the outer list is every requirement that must be met; each inner
// list is satisfied if the model used ANY ONE of its tokens. Example — "must state the
// delivery cost, AND must state a duration (either schema's field name for it)":
//
//	requires:
//	  - ["policy.main.delivery_cost"]
//	  - ["policy.main.delivery_time", "policy.main.delivery_in_days"]
type TestCase struct {
	ID       string       `yaml:"id"`
	Message  string       `yaml:"message"`
	Requires [][]string   `yaml:"requires"`
	Media    *MediaExpect `yaml:"media"`    // expected media behavior, if this test checks media
	Escalate *bool        `yaml:"escalate"` // expected escalate value, if this test checks escalation
	Language string       `yaml:"language"` // expected reply language ("kk"), checked on the INJECTED text
	// MustNotContain is a case-insensitive substring blocklist, checked against
	// reply_text. Mainly for escalation traps: escalate=true alone doesn't stop a model
	// from ALSO writing a confident, invented answer in the same reply (e.g. "we don't
	// deliver to Astana" — a claim the KB never actually makes) — seen for real in a run.
	MustNotContain []string `yaml:"must_not_contain"`
	// History is prior conversation turns rendered above {{message}} in the prompt, so a
	// test can check multi-turn behavior (a follow-up that only makes sense given
	// context, or a model asked to re-state a fact it already gave — must re-use the
	// token, not copy the literal value it wrote earlier in History). Absent or empty
	// means a fresh, no-history conversation, same as every test before this existed.
	History []HistoryTurn `yaml:"history"`
}

// HistoryTurn is one prior message in a test's simulated conversation. Text is authored
// prose (never a {{token}} — history represents what was ALREADY sent to the customer,
// i.e. already-injected text), rendered verbatim into the prompt's history block.
type HistoryTurn struct {
	Role string `yaml:"role" json:"role"` // "client" | "assistant"
	Text string `yaml:"text" json:"text"`
}

// MediaExpect describes what a test's answer must attach, checked against whichever of
// group/ref actually exists in the scenario's contract.
type MediaExpect struct {
	AnyOfGroups []string `yaml:"any_of_groups"` // e.g. ["coffee-machine.images"]
	AnyOfRefs   []string `yaml:"any_of_refs"`   // e.g. ["coffee-photo-1", "coffee-photo-2"]
}

// ModelsFile is models.yaml — the provider list + params shared by every scenario run, plus
// pricing provenance (PricingSource/PricingCheckedAt) so a report can say WHEN a price was
// last verified instead of presenting a hand-typed number as if it were always current.
type ModelsFile struct {
	PricingSource    string          `yaml:"pricing_source"`
	PricingCheckedAt string          `yaml:"pricing_checked_at"`
	Providers        []ModelProvider `yaml:"providers"`
}

// ModelProvider is one provider entry. InputPerMTok/OutputPerMTok are pointers, not plain
// floats: a model with no pricing entry (both nil) must report as "unknown_pricing", never
// silently default to $0 — a nil is a fact ("we haven't verified this price"), a 0.0 isn't.
type ModelProvider struct {
	ID            string   `yaml:"id"`
	Temperature   float64  `yaml:"temperature"`
	MaxTokens     int      `yaml:"max_tokens"`
	InputPerMTok  *float64 `yaml:"input_per_mtok"`
	OutputPerMTok *float64 `yaml:"output_per_mtok"`
	// Label disambiguates two provider entries that share the same ID (same underlying
	// model) but differ in config — e.g. a reasoning-on/reasoning-off comparison pair
	// (see ReasoningConfig below). Threaded through everywhere a model gets grouped
	// (judge.go's providerModelKey, report.go, viewmodel.go) specifically so two such
	// entries never silently merge into one bucket. Empty for every model that doesn't
	// need disambiguating — today, every entry in models.yaml.
	Label string `yaml:"label,omitempty"`
	// Provider pins the exact OpenRouter upstream route for a comparison run, instead of
	// letting OpenRouter's own load-balanced routing pick a possibly-different upstream
	// provider (and therefore possibly different quantization/latency/behavior) between
	// two calls meant to be directly comparable. nil means "unpinned" — OpenRouter's
	// default routing, today's behavior for every existing models.yaml entry. Forwarded
	// via promptfoo's confirmed `passthrough.provider` config key (render.go) and
	// directly on the extraction eval's own OpenRouter calls (openrouter.go).
	Provider *ProviderRoute `yaml:"provider,omitempty"`
	// Reasoning opts a model into OpenRouter's unified reasoning/extended-thinking API.
	// nil means reasoning is left at the provider's own default (today, off for every
	// existing models.yaml entry) — see ReasoningConfig's doc comment for the fields.
	Reasoning *ReasoningConfig `yaml:"reasoning,omitempty"`
}

// ProviderRoute is OpenRouter's provider-routing preferences object — see
// https://openrouter.ai/docs/guides/routing/provider-selection. Order lists upstream
// providers to try, in order (e.g. ["Google AI Studio"]); AllowFallbacks is a pointer
// because "unset" (let OpenRouter decide) and "explicitly false" (fail rather than
// silently fall back to a different, unpinned upstream) are different facts — the whole
// point of pinning a route for a comparison is that a silent fallback would defeat it.
type ProviderRoute struct {
	Order          []string `yaml:"order,omitempty"`
	AllowFallbacks *bool    `yaml:"allow_fallbacks,omitempty"`
}

// ReasoningConfig opts a model into OpenRouter's reasoning API (`reasoning: {...}` on the
// request — see OpenRouter's reasoning-tokens docs). Effort and MaxTokens are mutually
// exclusive per OpenRouter's own API (effort is a ratio of the model's own MaxTokens;
// MaxTokens here is an explicit, separate reasoning budget) — this harness doesn't
// enforce that exclusivity itself, since it's just forwarding config, not validating it.
//
// BilledSeparately is a HAND-DOCUMENTED fact about this specific provider, never inferred
// from a response: OpenRouter's response usage object reports reasoning token counts
// (when it reports them at all — see openrouter.go's defensive parsing) but not which
// dollar rate they billed at. Confirmed example: Gemini 2.5 Flash bills reasoning tokens
// at the standard output rate. Do not assume that generalizes to another provider without
// checking — leave nil (unknown) rather than guess.
type ReasoningConfig struct {
	Enabled bool `yaml:"enabled"`
	// Effort is one of OpenRouter's named levels: "none" | "minimal" | "low" | "medium" |
	// "high" | "xhigh" | "max". Mutually exclusive with MaxTokens.
	Effort string `yaml:"effort,omitempty"`
	// MaxTokens is an EXPLICIT reasoning token budget, separate from this same
	// ModelProvider's own MaxTokens (which bounds the final-answer completion only) — for
	// a provider that shares one completion budget across reasoning and the final answer,
	// leaving this at 0 (unset) risks the model's reasoning alone consuming the whole
	// MaxTokens budget before it ever writes reply_text.
	MaxTokens int `yaml:"max_tokens,omitempty"`
	// Exclude asks the provider to reason internally without returning reasoning content
	// in the response — still billed (if the provider bills reasoning at all), just not
	// returned. False (include it) is the default so this harness can actually inspect
	// and test for reasoning-content leakage (see judge.go's ReasoningLeak check).
	Exclude          bool  `yaml:"exclude,omitempty"`
	BilledSeparately *bool `yaml:"billed_separately,omitempty"`
}

// Catalog is generated/catalog.json — the ground truth judge.go validates answers
// against. Tokens is ordered (not a map) so two renders of unchanged data produce a
// byte-identical file, which makes diffing runs meaningful.
type Catalog struct {
	Contract    string        `json:"contract"` // "asset_refs" | "attach_groups"
	Tokens      []CatalogFact `json:"tokens"`
	MediaRefs   []string      `json:"media_refs"`   // valid individual refs (asset_refs contract)
	MediaGroups []string      `json:"media_groups"` // valid group names (attach_groups contract)
	// TrustedDigits is every digit run found in a FactRow's Description — the ONE field
	// this playground's own doctrine says is trusted prose a model may paraphrase, not a
	// tokenized fact (see FactRow.Description's comment). A model repeating "1.7 л" or "7
	// режимов" straight from a description is not inventing a number; judge.go's invented-
	// digit check excludes anything in this list. Confirmed against a real run: without
	// this, a model correctly quoting a description's spec got flagged as if it had
	// hallucinated a number.
	TrustedDigits []string `json:"trusted_digits"`
}

// CatalogFact is one resolvable token and the value injecting it must produce.
type CatalogFact struct {
	Token string `json:"token"` // "{{product.coffee-machine.price}}"
	Value string `json:"value"` // "129 900 ₸"
}

// ResolvedTests is generated/resolved_tests.json — the exact, ordered test list render.go
// used to build promptfooconfig.yaml. judge.go reads this instead of re-parsing
// tests.yaml + common/*.yaml, so there is only one place test resolution happens.
type ResolvedTests struct {
	Tests []TestCase `json:"tests"`
}
