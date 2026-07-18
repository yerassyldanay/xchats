package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xchats-evals-harness/internal/provenance"
)

func TestInventedNumberCheck(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		text    string
		wantOK  bool
	}{
		{
			name:    "thousands-grouped number in allow-list passes",
			allowed: []string{"25 000", "250"},
			text:    "Тариф Рост — 25 000 ₸/мес, до 250 платежей",
			wantOK:  true,
		},
		{
			name:    "an unlisted number fails",
			allowed: []string{"25 000"},
			text:    "Тариф Рост — 25 000 ₸/мес, скидка 30%",
			wantOK:  false,
		},
		{
			name:    "empty allow-list means no numbers at all",
			allowed: nil,
			text:    "Просто дрель без цены",
			wantOK:  true,
		},
		{
			name:    "empty allow-list rejects any number",
			allowed: nil,
			text:    "Модель ZT-40H, 1450W",
			wantOK:  false,
		},
		{
			name:    "a multi-group phone number matches regardless of how it's grouped: source style",
			allowed: []string{"7 702 976-65-09"},
			text:    "WhatsApp: +7 702 976-65-09",
			wantOK:  true,
		},
		{
			name:    "same phone number, re-grouped all-hyphens, still matches (same digits)",
			allowed: []string{"7 702 976-65-09"},
			text:    "WhatsApp: +7702-976-65-09",
			wantOK:  true,
		},
		{
			name:    "same phone number, transcribed with no separators at all, still matches",
			allowed: []string{"7 702 976-65-09"},
			text:    "WhatsApp: +77029766509",
			wantOK:  true,
		},
		{
			name:    "a wrong digit in the same shape still fails (grouping tolerance is not a typo tolerance)",
			allowed: []string{"7 702 976-65-09"},
			text:    "WhatsApp: +7 702 976-45-09",
			wantOK:  false,
		},
		{
			// Regression: evaltext.NormalizeText's newline-to-space collapsing used to
			// run BEFORE evaltext.NumberRunRE, destroying a genuine line-break boundary
			// between two UNRELATED numbers and merging them into one bogus "7 2" run.
			// inventedNumberCheck must use evaltext.FoldNBSP (keeps real newlines as hard
			// boundaries) instead.
			name:    "two unrelated numbers on separate lines never merge across the line break",
			allowed: []string{"77", "7", "2"},
			text:    "пр. Аль-Фараби, 77/7\n2 кассира",
			wantOK:  true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := ExtractCase{AllowedNumbers: tc.allowed}
			r := ExtractionResult{ExtractedText: tc.text}
			got := inventedNumberCheck(c, r)
			if got.Pass != tc.wantOK {
				t.Errorf("inventedNumberCheck() = %+v, want pass=%v", got, tc.wantOK)
			}
		})
	}
}

func TestRunExtractChecks_FieldsAndContains(t *testing.T) {
	c := ExtractCase{
		ID: "t",
		Fields: map[string]string{
			"content_kind":          "infographic",
			"visibility_suggestion": "visible",
		},
		TextContainsAll: []string{"старт", "10 000"},
		AllowedNumbers:  []string{"10 000"},
	}
	passing := ExtractionResult{
		ContentKind:          "Infographic", // case-insensitive match expected
		VisibilitySuggestion: "visible",
		ExtractedText:        "Старт — 10 000 ₸/мес",
	}
	checks := runExtractChecks(c, passing)
	if !allChecksPass(checks) {
		t.Fatalf("expected all checks to pass, got %+v", checks)
	}

	failing := ExtractionResult{
		ContentKind:          "product_photo", // wrong
		VisibilitySuggestion: "visible",
		ExtractedText:        "Старт — 25 000 ₸/мес", // missing "10 000", contains invented "25 000"
	}
	checks = runExtractChecks(c, failing)
	if allChecksPass(checks) {
		t.Fatalf("expected failures, got all-pass: %+v", checks)
	}
	names := map[string]bool{}
	for _, ch := range checks {
		if !ch.Pass {
			names[ch.Name] = true
		}
	}
	if !names["field:content_kind"] {
		t.Errorf("expected field:content_kind to fail, got %+v", checks)
	}
	if !names["text_contains_all"] {
		t.Errorf("expected text_contains_all to fail, got %+v", checks)
	}
	if !names["no_invented_numbers"] {
		t.Errorf("expected no_invented_numbers to fail, got %+v", checks)
	}
}

func TestRunExtractChecks_IdentifyAndCurrency(t *testing.T) {
	c := ExtractCase{
		ID:                  "t",
		IdentifyContainsAll: []string{"follis"},
		IdentifyContainsAny: []string{"дрель", "drill"},
		AllowedNumbers:      []string{},
		ForbidCurrency:      true,
	}

	ok := ExtractionResult{Summary: "Photo of a Follis magnetic drill press.", RelatesToHint: "drill"}
	if !allChecksPass(runExtractChecks(c, ok)) {
		t.Fatalf("expected pass, got %+v", runExtractChecks(c, ok))
	}

	badCurrency := ExtractionResult{Summary: "Follis drill, price 5000 ₸"}
	checks := runExtractChecks(c, badCurrency)
	if allChecksPass(checks) {
		t.Fatalf("expected forbid_currency to fail")
	}
	found := false
	for _, ch := range checks {
		if ch.Name == "forbid_currency" && !ch.Pass {
			found = true
		}
	}
	if !found {
		t.Errorf("expected forbid_currency check to fail, got %+v", checks)
	}

	missingIdentify := ExtractionResult{Summary: "A grey machine."}
	checks = runExtractChecks(c, missingIdentify)
	if allChecksPass(checks) {
		t.Fatalf("expected identify checks to fail for %+v", missingIdentify)
	}
}

func TestParseExtractionJSON(t *testing.T) {
	plain := `{"content_kind":"infographic","summary":"s","extracted_text":"t","language":"ru","visibility_suggestion":"visible","media_role_hint":"pricing","relates_to_hint":""}`
	r, err := parseExtractionJSON(plain)
	if err != nil {
		t.Fatalf("parse plain: %v", err)
	}
	if r.ContentKind != "infographic" || r.Language != "ru" {
		t.Errorf("unexpected parse: %+v", r)
	}

	fenced := "```json\n" + plain + "\n```"
	r2, err := parseExtractionJSON(fenced)
	if err != nil {
		t.Fatalf("parse fenced: %v", err)
	}
	if r2 != r {
		t.Errorf("fenced parse %+v != plain parse %+v", r2, r)
	}

	if _, err := parseExtractionJSON("not json at all"); err == nil {
		t.Errorf("expected an error parsing non-JSON, got nil")
	}
}

func TestAllChecksPass(t *testing.T) {
	if allChecksPass(nil) {
		t.Errorf("allChecksPass(nil) should be false (no checks ran = nothing verified)")
	}
	if !allChecksPass([]CheckResult{{Name: "a", Pass: true}}) {
		t.Errorf("expected true for a single passing check")
	}
	if allChecksPass([]CheckResult{{Name: "a", Pass: true}, {Name: "b", Pass: false}}) {
		t.Errorf("expected false when any check fails")
	}
}

func TestEstimateCost(t *testing.T) {
	in, out := 0.15, 0.60
	priced := ModelProvider{InputPerMTok: &in, OutputPerMTok: &out}
	cost, basis := estimateCost(priced, orUsage{PromptTokens: 1_000_000, CompletionTokens: 1_000_000})
	if basis != CostBasisMeasured {
		t.Errorf("want basis %q, got %q", CostBasisMeasured, basis)
	}
	if want := in + out; cost != want {
		t.Errorf("cost = %v, want %v", cost, want)
	}

	unpriced := ModelProvider{}
	_, basis = estimateCost(unpriced, orUsage{PromptTokens: 100})
	if basis != CostBasisUnknownPricing {
		t.Errorf("want basis %q, got %q", CostBasisUnknownPricing, basis)
	}
}

func TestOrModelID(t *testing.T) {
	if got := orModelID("openrouter:google/gemini-2.5-flash"); got != "google/gemini-2.5-flash" {
		t.Errorf("orModelID stripped wrong: %q", got)
	}
	if got := orModelID("google/gemini-2.5-flash"); got != "google/gemini-2.5-flash" {
		t.Errorf("orModelID should be a no-op without the prefix: %q", got)
	}
}

func TestResolveVisionModels_ExplicitFlagFiltersAndAddsUnknown(t *testing.T) {
	dir := t.TempDir()
	modelsPath := filepath.Join(dir, "models.yaml")
	content := `
providers:
  - id: openrouter:google/gemini-2.5-flash
    temperature: 0.3
    max_tokens: 500
    input_per_mtok: 0.30
    output_per_mtok: 2.50
  - id: openrouter:openai/gpt-4o-mini
    temperature: 0.3
    max_tokens: 500
`
	if err := os.WriteFile(modelsPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// No flag/env -> every provider.
	all, err := resolveVisionModels("", modelsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 providers with no filter, got %d", len(all))
	}

	// Explicit flag selects a known id (with or without the openrouter: prefix) and
	// tolerates an unknown one (falls back to harness defaults, unknown_pricing).
	filtered, err := resolveVisionModels("google/gemini-2.5-flash,some/unknown-model", modelsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 2 {
		t.Fatalf("want 2 (1 known + 1 fallback), got %d: %+v", len(filtered), filtered)
	}
	if filtered[0].ID != "google/gemini-2.5-flash" || filtered[0].InputPerMTok == nil {
		t.Errorf("known model should keep its models.yaml pricing: %+v", filtered[0])
	}
	if filtered[1].ID != "some/unknown-model" || filtered[1].InputPerMTok != nil {
		t.Errorf("unknown model should fall back with no pricing: %+v", filtered[1])
	}
}

func TestLoadExtractCases_RealCasesFile(t *testing.T) {
	cf, err := loadExtractCases(filepath.Join("..", "extract", "cases.yaml"))
	if err != nil {
		t.Fatalf("load real cases.yaml: %v", err)
	}
	wantIDs := map[string]bool{
		"infographic": false, "product-photo": false, "screenshot": false,
		"how-to-cachier": false, "how-it-works": false,
	}
	for _, c := range cf.Cases {
		if _, ok := wantIDs[c.ID]; !ok {
			t.Errorf("unexpected case id %q", c.ID)
			continue
		}
		wantIDs[c.ID] = true
		if c.File == "" {
			t.Errorf("case %q has no file", c.ID)
		}
	}
	for id, found := range wantIDs {
		if !found {
			t.Errorf("expected case %q in cases.yaml, not found", id)
		}
	}
}

func TestLoadExtractCases_RejectsMissingID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cases.yaml")
	if err := os.WriteFile(path, []byte("cases:\n  - file: x.png\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadExtractCases(path); err == nil {
		t.Errorf("expected an error for a case missing id, got nil")
	}
}

// TestLoadAndScaleImage_RealAssets exercises real image decode + downscale against the
// actual files this eval's cases reference (not synthetic ones) — the cheapest way to
// catch a codec/path problem before it costs a real API call.
func TestLoadAndScaleImage_RealAssets(t *testing.T) {
	cf, err := loadExtractCases(filepath.Join("..", "extract", "cases.yaml"))
	if err != nil {
		t.Fatalf("load cases.yaml: %v", err)
	}
	for _, c := range cf.Cases {
		path := filepath.Join("..", "extract", c.File)
		data, mimetype, err := loadAndScaleImage(path)
		if err != nil {
			t.Errorf("case %s: loadAndScaleImage(%s): %v", c.ID, path, err)
			continue
		}
		if mimetype != "image/jpeg" {
			t.Errorf("case %s: want image/jpeg, got %s", c.ID, mimetype)
		}
		if len(data) == 0 {
			t.Errorf("case %s: scaled image is empty", c.ID)
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			t.Errorf("case %s: scaled output does not decode: %v", c.ID, err)
			continue
		}
		b := img.Bounds()
		if b.Dx() > maxImageSide || b.Dy() > maxImageSide {
			t.Errorf("case %s: scaled image %dx%d exceeds maxImageSide=%d", c.ID, b.Dx(), b.Dy(), maxImageSide)
		}
	}
}

func TestDownscale_NoopWhenAlreadySmall(t *testing.T) {
	small := image.NewRGBA(image.Rect(0, 0, 100, 50))
	out := downscale(small, maxImageSide)
	if out.Bounds() != small.Bounds() {
		t.Errorf("expected no resize for an already-small image, got %v", out.Bounds())
	}
}

func TestDownscale_ShrinksLongestEdge(t *testing.T) {
	big := image.NewRGBA(image.Rect(0, 0, 4000, 2000))
	for y := 0; y < 2000; y += 50 {
		for x := 0; x < 4000; x += 50 {
			big.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	out := downscale(big, maxImageSide)
	b := out.Bounds()
	if b.Dx() != maxImageSide {
		t.Errorf("want width %d, got %d", maxImageSide, b.Dx())
	}
	if b.Dy() != maxImageSide/2 {
		t.Errorf("want height %d (aspect ratio preserved), got %d", maxImageSide/2, b.Dy())
	}
}

// TestLoadAndScaleImage_SyntheticPNG proves the decode->downscale->JPEG-encode path end
// to end on a file this test fully controls (belt-and-suspenders alongside the real-asset
// test above, which depends on files that could change).
func TestLoadAndScaleImage_SyntheticPNG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.png")
	img := image.NewRGBA(image.Rect(0, 0, 2000, 1000))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	data, mimetype, err := loadAndScaleImage(path)
	if err != nil {
		t.Fatalf("loadAndScaleImage: %v", err)
	}
	if mimetype != "image/jpeg" {
		t.Errorf("want image/jpeg, got %s", mimetype)
	}
	if len(data) == 0 {
		t.Errorf("expected non-empty output")
	}
}

// TestDescribeImage_FakeServer drives the real orClient.describeImage HTTP path against
// an httptest fake OpenRouter server — proves the live-call plumbing (request shape,
// response parsing, usage extraction) without spending anything or needing a real key.
func TestDescribeImage_FakeServer(t *testing.T) {
	var gotReq orRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("server: decode request: %v", err)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("server: want Authorization 'Bearer test-key', got %q", got)
		}
		resp := orResponse{
			Usage: &orUsage{PromptTokens: 123, CompletionTokens: 45},
		}
		resp.Choices = []orChoice{{}}
		resp.Choices[0].Message.Content = `{"content_kind":"product_photo","summary":"a drill","extracted_text":"","language":"none","visibility_suggestion":"visible","media_role_hint":"gallery","relates_to_hint":""}`
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newORClient(srv.URL, "test-key")
	model := ModelProvider{ID: "openrouter:google/gemini-2.5-flash", Temperature: 0.3, MaxTokens: 700}
	raw, reasoning, usage, err := client.describeImage(context.Background(), model, "system prompt", []byte{0xFF, 0xD8}, "image/jpeg")
	if err != nil {
		t.Fatalf("describeImage: %v", err)
	}
	if reasoning != "" {
		t.Errorf("want empty reasoning (fake server never set it), got %q", reasoning)
	}
	if usage.PromptTokens != 123 || usage.CompletionTokens != 45 {
		t.Errorf("unexpected usage: %+v", usage)
	}
	parsed, err := parseExtractionJSON(raw)
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if parsed.ContentKind != "product_photo" {
		t.Errorf("unexpected parsed result: %+v", parsed)
	}

	// The request the fake server actually received must use the de-prefixed model id
	// and carry the image as a data URL content part.
	if gotReq.Model != "google/gemini-2.5-flash" {
		t.Errorf("want de-prefixed model id sent, got %q", gotReq.Model)
	}
	if len(gotReq.Messages) != 1 || len(gotReq.Messages[0].Content) != 2 {
		t.Fatalf("unexpected request shape: %+v", gotReq)
	}
	if gotReq.Messages[0].Content[1].ImageURL == nil {
		t.Errorf("expected an image_url content part")
	}
}

// TestDescribeImage_ProviderAndReasoningPrefsRoundTrip proves both halves of fix
// 4b/5's wiring against a real (fake-server) HTTP round trip, not just struct
// construction: (1) a ModelProvider with Provider/Reasoning config set produces the
// exact OpenRouter wire shape ({"order":...,"allow_fallbacks":...} /
// {"enabled":...,"effort":...,"max_tokens":...}) on the outgoing request, and (2) a
// response's separate message.reasoning field is captured into its OWN return value —
// never appended to, or confused with, raw (which stays exactly message.content).
func TestDescribeImage_ProviderAndReasoningPrefsRoundTrip(t *testing.T) {
	var gotReq orRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("server: decode request: %v", err)
		}
		resp := orResponse{Usage: &orUsage{PromptTokens: 10, CompletionTokens: 5}}
		resp.Choices = []orChoice{{}}
		resp.Choices[0].Message.Content = `{"content_kind":"other","summary":"x","extracted_text":"","language":"none","visibility_suggestion":"visible","media_role_hint":"none","relates_to_hint":""}`
		resp.Choices[0].Message.Reasoning = "internal chain of thought that must never reach the graded output"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	allowFallbacks := false
	model := ModelProvider{
		ID:          "openrouter:google/gemini-2.5-flash",
		Temperature: 0.3,
		MaxTokens:   500,
		Provider:    &ProviderRoute{Order: []string{"Google AI Studio"}, AllowFallbacks: &allowFallbacks},
		Reasoning:   &ReasoningConfig{Enabled: true, Effort: "low"},
	}
	client := newORClient(srv.URL, "test-key")
	raw, reasoning, _, err := client.describeImage(context.Background(), model, "system prompt", []byte{0xFF, 0xD8}, "image/jpeg")
	if err != nil {
		t.Fatalf("describeImage: %v", err)
	}

	if gotReq.Provider == nil {
		t.Fatal("want the request to carry a provider preference block")
	}
	if len(gotReq.Provider.Order) != 1 || gotReq.Provider.Order[0] != "Google AI Studio" {
		t.Errorf("want provider.order=[Google AI Studio], got %+v", gotReq.Provider.Order)
	}
	if gotReq.Provider.AllowFallbacks == nil || *gotReq.Provider.AllowFallbacks != false {
		t.Errorf("want provider.allow_fallbacks=false, got %+v", gotReq.Provider.AllowFallbacks)
	}
	if gotReq.Reasoning == nil || !gotReq.Reasoning.Enabled || gotReq.Reasoning.Effort != "low" {
		t.Errorf("want reasoning={enabled:true, effort:low}, got %+v", gotReq.Reasoning)
	}

	if reasoning != "internal chain of thought that must never reach the graded output" {
		t.Errorf("want the response's reasoning field captured verbatim, got %q", reasoning)
	}
	if strings.Contains(raw, "internal chain of thought") {
		t.Fatal("reasoning content leaked into raw (=message.content) — these must stay separate return values")
	}
}

// TestDescribeImage_NoProviderOrReasoningConfigOmitsFieldsEntirely proves the "unset
// means omitted, not null" contract for a model with neither Provider nor Reasoning
// configured (today, every entry in models.yaml) — the request must not even mention
// these keys, matching this harness's existing "never present a fact more confidently
// than it's known" discipline.
func TestDescribeImage_NoProviderOrReasoningConfigOmitsFieldsEntirely(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rawBody = b
		resp := orResponse{Usage: &orUsage{PromptTokens: 1, CompletionTokens: 1}}
		resp.Choices = []orChoice{{}}
		resp.Choices[0].Message.Content = `{"content_kind":"other","summary":"x","extracted_text":"","language":"none","visibility_suggestion":"visible","media_role_hint":"none","relates_to_hint":""}`
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	model := ModelProvider{ID: "openrouter:openai/gpt-4o-mini", Temperature: 0.3, MaxTokens: 500}
	client := newORClient(srv.URL, "test-key")
	if _, _, _, err := client.describeImage(context.Background(), model, "p", []byte{1}, "image/jpeg"); err != nil {
		t.Fatalf("describeImage: %v", err)
	}

	if strings.Contains(string(rawBody), `"provider"`) {
		t.Errorf("want no \"provider\" key on the wire when unset, got body: %s", rawBody)
	}
	if strings.Contains(string(rawBody), `"reasoning"`) {
		t.Errorf("want no \"reasoning\" key on the wire when unset, got body: %s", rawBody)
	}
}

// TestResolveVisionModels_SameIDDifferentLabelBothSelected mirrors
// TestFilterProviders_SameIDDifferentLabelBothSelected for the extraction eval's own
// model-selection path (a separate byID map from render.go's, with the identical bug
// found independently by review).
func TestResolveVisionModels_SameIDDifferentLabelBothSelected(t *testing.T) {
	dir := t.TempDir()
	modelsYAML := `providers:
  - id: openrouter:google/gemini-2.5-flash
    label: reasoning-off
    temperature: 0.3
    max_tokens: 700
  - id: openrouter:google/gemini-2.5-flash
    label: reasoning-on
    temperature: 0.3
    max_tokens: 700
`
	path := filepath.Join(dir, "models.yaml")
	if err := os.WriteFile(path, []byte(modelsYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveVisionModels("google/gemini-2.5-flash", path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want both labeled entries selected, got %d: %+v", len(got), got)
	}
	labels := map[string]bool{got[0].Label: true, got[1].Label: true}
	if !labels["reasoning-off"] || !labels["reasoning-on"] {
		t.Errorf("want both reasoning-off and reasoning-on present, got labels %+v", labels)
	}
}

// TestDescribeImage_ReasoningEnabledFalseIsSentExplicitly is the regression test for a
// bug a review caught: orReasoningPrefs.Enabled had `,omitempty` on a bool, so an
// explicit Reasoning.Enabled=false was silently dropped from the wire request —
// indistinguishable from Reasoning being unset entirely — while the promptfoo-mediated
// path (render.go's buildPassthrough) always sent it. A config with Reasoning non-nil
// but Enabled=false (e.g. `reasoning: {enabled: false, exclude: true}`) must produce an
// explicit "enabled":false on the wire, matching that other path exactly.
func TestDescribeImage_ReasoningEnabledFalseIsSentExplicitly(t *testing.T) {
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rawBody = b
		resp := orResponse{Usage: &orUsage{PromptTokens: 1, CompletionTokens: 1}}
		resp.Choices = []orChoice{{}}
		resp.Choices[0].Message.Content = `{"content_kind":"other","summary":"x","extracted_text":"","language":"none","visibility_suggestion":"visible","media_role_hint":"none","relates_to_hint":""}`
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	model := ModelProvider{
		ID: "openrouter:google/gemini-2.5-flash", Temperature: 0.3, MaxTokens: 500,
		Reasoning: &ReasoningConfig{Enabled: false, Exclude: true},
	}
	client := newORClient(srv.URL, "test-key")
	if _, _, _, err := client.describeImage(context.Background(), model, "p", []byte{1}, "image/jpeg"); err != nil {
		t.Fatalf("describeImage: %v", err)
	}

	if !strings.Contains(string(rawBody), `"enabled":false`) {
		t.Errorf("want an explicit \"enabled\":false on the wire (Reasoning was set, just disabled) — omitempty must not silently drop it, got body: %s", rawBody)
	}
}

func TestDescribeImage_HTTPErrorSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer srv.Close()

	client := newORClient(srv.URL, "test-key")
	model := ModelProvider{ID: "x/y"}
	_, _, _, err := client.describeImage(context.Background(), model, "p", []byte{1}, "image/jpeg")
	if err == nil {
		t.Fatalf("expected an error for a non-200 response")
	}
}

// TestRunOneExtraction_RetriesOnParseFailure simulates the observed real-world pattern
// (a truncated, unparseable response on the first N attempts, then a clean one) and
// verifies runOneExtraction recovers rather than reporting a failure a plain retry
// would have fixed.
func TestRunOneExtraction_RetriesOnParseFailure(t *testing.T) {
	goodJSON := `{"content_kind":"product_photo","summary":"a drill","extracted_text":"","language":"none","visibility_suggestion":"visible","media_role_hint":"gallery","relates_to_hint":""}`
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		var content string
		if calls <= parseFailureRetries { // truncated/unparseable on every attempt except the last
			content = `{"content_kind": "product_photo", "summary": "cut off mid`
		} else {
			content = goodJSON
		}
		resp := orResponse{}
		resp.Choices = []orChoice{{}}
		resp.Choices[0].Message.Content = content
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newORClient(srv.URL, "test-key")
	c := ExtractCase{ID: "product-photo"}
	m := ModelProvider{ID: "google/gemini-2.5-flash", Temperature: 0.3, MaxTokens: 700}
	p := provenance.LoadedPrompt{Ref: provenance.PromptRef{Name: "extract", Version: 1}, Content: "test prompt"}
	res := runOneExtraction(context.Background(), client, c, m, p, preprocessorImageJPEG85, []byte{0xFF, 0xD8}, "image/jpeg")

	if calls != parseFailureRetries+1 {
		t.Fatalf("want exactly %d calls (retries exhausted then one more), got %d", parseFailureRetries+1, calls)
	}
	if res.ParseError != "" {
		t.Fatalf("expected eventual success, got parse error: %s", res.ParseError)
	}
	if res.Parsed == nil || res.Parsed.ContentKind != "product_photo" {
		t.Fatalf("expected the eventually-successful parse, got %+v", res.Parsed)
	}
}

// TestRunOneExtraction_GivesUpAfterRetries proves a PERSISTENT parse failure (every
// attempt truncated) is still reported honestly, not silently retried forever.
func TestRunOneExtraction_GivesUpAfterRetries(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		resp := orResponse{}
		resp.Choices = []orChoice{{}}
		resp.Choices[0].Message.Content = `{"content_kind": "always cut off`
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := newORClient(srv.URL, "test-key")
	c := ExtractCase{ID: "product-photo"}
	m := ModelProvider{ID: "x/y"}
	p := provenance.LoadedPrompt{Ref: provenance.PromptRef{Name: "extract", Version: 1}, Content: "test prompt"}
	res := runOneExtraction(context.Background(), client, c, m, p, preprocessorImageJPEG85, []byte{0xFF, 0xD8}, "image/jpeg")

	if calls != parseFailureRetries+1 {
		t.Fatalf("want exactly %d calls (no calls beyond the retry budget), got %d", parseFailureRetries+1, calls)
	}
	if res.ParseError == "" {
		t.Fatalf("expected a reported parse error after exhausting retries, got a clean result: %+v", res)
	}
}
