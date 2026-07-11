package main

import (
	"strings"
	"testing"
)

func TestBuildScaleComparison(t *testing.T) {
	scaleStats := map[string]map[string]*modelStats{
		"shop-scale-10": {
			"modelA": {model: "modelA", total: 10, modelBehaviorPass: 9, tokensSum: 15000},
		},
		"shop-scale-20": {
			"modelA": {model: "modelA", total: 10, modelBehaviorPass: 7, tokensSum: 20000},
		},
	}

	got := buildScaleComparison(scaleStats)
	if got == "" {
		t.Fatal("expected a non-empty table for 2+ shop-scale-N scenarios")
	}

	// shop-scale-10 must appear before shop-scale-20 (numeric size order, not string order).
	idx10 := indexOf(got, "shop-scale-10")
	idx20 := indexOf(got, "shop-scale-20")
	if idx10 == -1 || idx20 == -1 || idx10 > idx20 {
		t.Fatalf("expected shop-scale-10 column before shop-scale-20, got: %s", got)
	}

	if got := buildScaleComparison(map[string]map[string]*modelStats{"shop-scale-10": {}}); got != "" {
		t.Fatal("a single shop-scale scenario should not produce a comparison table")
	}
}

func TestScaleSizeOf(t *testing.T) {
	if got := scaleSizeOf("shop-scale-10"); got != 10 {
		t.Fatalf("scaleSizeOf(shop-scale-10) = %d, want 10", got)
	}
	if got := scaleSizeOf("shop-scale-30"); got != 30 {
		t.Fatalf("scaleSizeOf(shop-scale-30) = %d, want 30", got)
	}
}

func TestFormatCostCell(t *testing.T) {
	tests := []struct {
		name string
		ms   modelStats
		want string
	}{
		{
			name: "all unknown pricing",
			ms:   modelStats{total: 12, unknownPricingN: 12},
			want: "unknown pricing",
		},
		{
			name: "all cached with nothing to borrow from",
			ms:   modelStats{total: 12, unpricedN: 12},
			want: "unpriceable (cached, no split to borrow)",
		},
		{
			name: "measured rows priced",
			ms:   modelStats{total: 2, measuredN: 2, costEstimate: 0.0012},
			want: "$0.0012 est. (2 measured)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatCostCell(&tt.ms); got != tt.want {
				t.Fatalf("formatCostCell() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBuildSummary_WilsonIntervalIsPooledPerModelNeverAcrossModels proves both halves of
// fix 3's sample-size reporting: the summary table carries a Wilson interval computed
// from each model's OWN pooled total (not a per-intent number, and not shared across
// rows), and two different models in the same scenario get two DIFFERENTLY-valued
// intervals — the concrete guarantee behind "never pool results across different
// models; a specific prompt+model combination is the experimental unit throughout".
func TestBuildSummary_WilsonIntervalIsPooledPerModelNeverAcrossModels(t *testing.T) {
	// modelA: 8/10 pass. modelB: 3/10 pass. Deliberately different pass rates so a bug
	// that accidentally pooled the two models together would produce IDENTICAL (wrong)
	// intervals for both rows instead of two distinct ones.
	var verdicts []Verdict
	for i := 0; i < 10; i++ {
		verdicts = append(verdicts, Verdict{TestID: "t", Model: "modelA", ModelBehaviorPass: i < 8, ContractPass: true})
		verdicts = append(verdicts, Verdict{TestID: "t", Model: "modelB", ModelBehaviorPass: i < 3, ContractPass: true})
	}
	runs := []JudgedRun{{Scenario: "fixture-scenario", Verdicts: verdicts}}
	models := &ModelsFile{PricingSource: "test", PricingCheckedAt: "2026-01-01"}

	summary := buildSummary("fixture-run", runs, models)

	wantA := "[49%, 94%]" // wilsonInterval(8,10) — see wilson_test.go's reference value
	wantB := "[11%, 60%]" // wilsonInterval(3,10)
	if !strings.Contains(summary, wantA) {
		t.Errorf("want modelA's pooled 8/10 Wilson interval %s in the summary, got:\n%s", wantA, summary)
	}
	if !strings.Contains(summary, wantB) {
		t.Errorf("want modelB's pooled 3/10 Wilson interval %s in the summary, got:\n%s", wantB, summary)
	}
	if !strings.Contains(summary, "95% CI (Wilson, pooled)") {
		t.Error("want the CI column explicitly labeled as pooled, not per-intent")
	}
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
