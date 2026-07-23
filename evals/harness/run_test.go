package main

import "testing"

func TestResolveExpectedCalls(t *testing.T) {
	tests := []struct {
		name                           string
		totalTests, numModels, repeats int
		want                           int
	}{
		{"no repeats (default)", 20, 4, 1, 80},
		{"screening: 3 uncached repetitions", 6, 4, 3, 72},
		{"finalist: 15 intents x 5 repetitions, one model", 15, 1, 5, 75},
		{"zero tests", 0, 4, 3, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveExpectedCalls(tt.totalTests, tt.numModels, tt.repeats); got != tt.want {
				t.Errorf("resolveExpectedCalls(%d,%d,%d) = %d, want %d", tt.totalTests, tt.numModels, tt.repeats, got, tt.want)
			}
		})
	}
}

func TestValidateResolvedRunSize(t *testing.T) {
	tests := []struct {
		name                   string
		totalTests, totalCalls int
		wantErr                bool
	}{
		{"normal run", 31, 93, false},
		{"zero tests", 0, 0, true},
		{"zero calls", 31, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateResolvedRunSize(tt.totalTests, tt.totalCalls)
			if tt.wantErr && err == nil {
				t.Fatal("want error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("want no error, got %v", err)
			}
		})
	}
}

func TestValidateRepeats(t *testing.T) {
	tests := []struct {
		name    string
		repeats int
		noCache bool
		wantErr bool
	}{
		{"default repeats=1, cache allowed", 1, false, false},
		{"repeats=1 with -no-cache is also fine", 1, true, false},
		{"repeats>1 WITHOUT -no-cache is rejected", 5, false, true},
		{"repeats>1 WITH -no-cache is fine", 5, true, false},
		{"repeats=0 is rejected outright", 0, true, true},
		{"negative repeats is rejected outright", -1, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepeats(tt.repeats, tt.noCache)
			if tt.wantErr && err == nil {
				t.Errorf("validateRepeats(%d, noCache=%v): want an error, got nil", tt.repeats, tt.noCache)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateRepeats(%d, noCache=%v): want no error, got %v", tt.repeats, tt.noCache, err)
			}
		})
	}
}
