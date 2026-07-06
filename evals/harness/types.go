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
	Name  string `yaml:"name"`  // column name, e.g. "delivery_in_days" -> token {{policy.main.delivery_in_days}}
	Label string `yaml:"label"` // e.g. "срок доставки, в днях"
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
}

// MediaExpect describes what a test's answer must attach, checked against whichever of
// group/ref actually exists in the scenario's contract.
type MediaExpect struct {
	AnyOfGroups []string `yaml:"any_of_groups"` // e.g. ["coffee-machine.images"]
	AnyOfRefs   []string `yaml:"any_of_refs"`   // e.g. ["coffee-photo-1", "coffee-photo-2"]
}

// ModelsFile is models.yaml — the provider list + params shared by every scenario run.
type ModelsFile struct {
	Providers []ModelProvider `yaml:"providers"`
}

type ModelProvider struct {
	ID          string  `yaml:"id"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

// Catalog is generated/catalog.json — the ground truth judge.go validates answers
// against. Tokens is ordered (not a map) so two renders of unchanged data produce a
// byte-identical file, which makes diffing runs meaningful.
type Catalog struct {
	Contract    string        `json:"contract"` // "asset_refs" | "attach_groups"
	Tokens      []CatalogFact `json:"tokens"`
	MediaRefs   []string      `json:"media_refs"`   // valid individual refs (asset_refs contract)
	MediaGroups []string      `json:"media_groups"` // valid group names (attach_groups contract)
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
