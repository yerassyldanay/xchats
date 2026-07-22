package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"xchats-evals-harness/internal/kbfixture"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

func loadSchemaKBJudgeContext(t *testing.T) (*aiprompt.KB, *aiprompt.Catalog) {
	t.Helper()
	dir := t.TempDir()
	fixturePath := filepath.Join(dir, "data-ru.yaml")
	mustWrite(t, fixturePath, testSchemaKBFixture)
	kb, err := kbfixture.Load(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := aiprompt.BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	return kb, cat
}

func schemaKBResponse(t *testing.T, text string, media []string, escalationReason string) string {
	t.Helper()
	b, err := json.Marshal(aiprompt.Response{
		ReplyText:        text,
		ReplyLanguage:    "ru",
		MediaFilesToSend: media,
		Escalate:         escalationReason != "",
		EscalationReason: escalationReason,
		Confidence:       0.9,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func schemaKBPromptfooRow(output string) PromptfooRow {
	row := PromptfooRow{}
	row.Provider.ID = "openrouter:test/model"
	row.TestCase.Description = "schema test"
	row.Response.Output = output
	row.Response.FinishReason = "stop"
	return row
}

func judgeSchemaKBOutput(t *testing.T, kb *aiprompt.KB, cat *aiprompt.Catalog, output string) Verdict {
	t.Helper()
	return judgeOneSchemaKB(
		TestCase{},
		schemaKBPromptfooRow(output),
		kb,
		cat,
		aipromptTokenValues(cat),
		trustedDigitsFromKB(kb),
	)
}

func TestJudgeOneSchemaKB_ValidatesAndInjectsCurrentFact(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContext(t)
	output := schemaKBResponse(t, "Цена — {{product.coffee-machine.price}}.", []string{}, "")
	v := judgeSchemaKBOutput(t, kb, cat, output)
	if !v.ContractPass {
		t.Fatalf("ContractPass=false: %s", v.Reason)
	}
	if v.InjectedText != "Цена — 129 900 ₸." {
		t.Fatalf("InjectedText = %q", v.InjectedText)
	}
	parseOK, contractPass := firstAttemptOutcome(schemaKBPromptfooRow(output), v, func(row PromptfooRow) Verdict {
		return judgeOneSchemaKB(TestCase{}, row, kb, cat, aipromptTokenValues(cat), trustedDigitsFromKB(kb))
	})
	if !parseOK || !contractPass {
		t.Fatalf("first attempt = (%v, %v), want valid contract", parseOK, contractPass)
	}
}

func TestJudgeOneSchemaKB_BlocksFactContractIssues(t *testing.T) {
	tests := []struct {
		name             string
		text             string
		escalationReason string
		mutate           func(*aiprompt.KB)
	}{
		{name: "unknown placeholder", text: "{{product.unknown.price}}"},
		{name: "malformed placeholder", text: "{product.coffee-machine.price}"},
		{name: "literal exact value", text: "Цена — 129 900 ₸."},
		{
			name:             "placeholder outside reply",
			text:             "Уточню.",
			escalationReason: "Нужен {{product.coffee-machine.price}}",
		},
		{
			name: "stale placeholder",
			text: "{{product.coffee-machine.price}}",
			mutate: func(kb *aiprompt.KB) {
				kb.Products[0].Price = ""
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kb, cat := loadSchemaKBJudgeContext(t)
			if tt.mutate != nil {
				tt.mutate(kb)
			}
			output := schemaKBResponse(t, tt.text, []string{}, tt.escalationReason)
			v := judgeSchemaKBOutput(t, kb, cat, output)
			if !v.Blocked || v.ContractPass {
				t.Fatalf("Blocked=%v ContractPass=%v reason=%q", v.Blocked, v.ContractPass, v.Reason)
			}
			if candidates := retryCandidateIndexesSchemaKB([]PromptfooRow{schemaKBPromptfooRow(output)}, kb, cat); len(candidates) != 0 {
				t.Fatalf("fact behavior failure must not auto-retry, got candidates %v", candidates)
			}
		})
	}
}

func TestJudgeOneSchemaKB_UnknownMediaFailsContractAndRetries(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContext(t)
	output := schemaKBResponse(t, "Прикрепляю.", []string{"products.unknown.gallery_images"}, "")
	row := schemaKBPromptfooRow(output)
	v := judgeSchemaKBOutput(t, kb, cat, output)
	if v.ContractPass || len(v.UnknownMedia) != 1 {
		t.Fatalf("ContractPass=%v UnknownMedia=%v", v.ContractPass, v.UnknownMedia)
	}
	candidates := retryCandidateIndexesSchemaKB([]PromptfooRow{row}, kb, cat)
	if len(candidates) != 1 || candidates[0] != 0 {
		t.Fatalf("retry candidates = %v, want [0]", candidates)
	}
}

func TestJudgeOneSchemaKB_RereadsMediaColumn(t *testing.T) {
	kb, cat := loadSchemaKBJudgeContext(t)
	kb.Products[0].FeaturedImage = ""
	output := schemaKBResponse(
		t,
		"Прикрепляю фото.",
		[]string{"products.coffee-machine.featured_image"},
		"",
	)
	v := judgeSchemaKBOutput(t, kb, cat, output)
	if v.MediaResolveOK || v.ContractPass {
		t.Fatalf("MediaResolveOK=%v ContractPass=%v reason=%q", v.MediaResolveOK, v.ContractPass, v.Reason)
	}
}
