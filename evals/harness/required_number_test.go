package main

import "testing"

func TestRequiredNumberCheck(t *testing.T) {
	tests := []struct {
		name     string
		required []string
		text     string
		wantPass bool
	}{
		{
			name:     "all required numbers present, exact grouping",
			required: []string{"24 500", "2041"},
			text:     "К оплате 24 500 ₸. Заказ #A-2041.",
			wantPass: true,
		},
		{
			name:     "grouping differs but canonicalizes the same — still passes",
			required: []string{"7 777 123-45-67"},
			text:     "Phone: +77771234567",
			wantPass: true,
		},
		{
			name:     "a required number silently dropped fails",
			required: []string{"24 500", "320"},
			text:     "К оплате 24 500 ₸.", // "320" (bonus points) never transcribed
			wantPass: false,
		},
		{
			name:     "empty text with required numbers fails",
			required: []string{"245 600"},
			text:     "",
			wantPass: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := ExtractCase{RequiredNumbers: tc.required}
			r := ExtractionResult{ExtractedText: tc.text}
			got := requiredNumberCheck(c, r)
			if got.Pass != tc.wantPass {
				t.Errorf("requiredNumberCheck() = %+v, want pass=%v", got, tc.wantPass)
			}
		})
	}
}

// TestRunExtractChecks_RequiredNumbersOptedIn proves the check is skipped entirely for a
// case that declares no RequiredNumbers (most cases: not every visible number is
// business-critical), and included when a case does declare them.
func TestRunExtractChecks_RequiredNumbersOptedIn(t *testing.T) {
	withoutRequired := ExtractCase{AllowedNumbers: []string{}}
	checks := runExtractChecks(withoutRequired, ExtractionResult{})
	for _, c := range checks {
		if c.Name == "required_numbers" {
			t.Fatalf("expected no required_numbers check for a case that doesn't declare any, got %+v", checks)
		}
	}

	withRequired := ExtractCase{AllowedNumbers: []string{"1"}, RequiredNumbers: []string{"1"}}
	checks = runExtractChecks(withRequired, ExtractionResult{ExtractedText: "1"})
	found := false
	for _, c := range checks {
		if c.Name == "required_numbers" {
			found = true
			if !c.Pass {
				t.Errorf("expected required_numbers to pass, got %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("expected a required_numbers check to be present for a case that declares RequiredNumbers")
	}
}
