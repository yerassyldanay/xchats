package main

import "testing"

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

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
