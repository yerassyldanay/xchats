package provenance

import "testing"

func TestParsePromptSpec(t *testing.T) {
	tests := []struct {
		spec        string
		wantName    string
		wantVersion int
		wantErr     bool
	}{
		{"extract@v1", "extract", 1, false},
		{"extract@1", "extract", 1, false},
		{"extract@v12", "extract", 12, false},
		{"draft-schema@v3", "draft-schema", 3, false},
		{"extract", "", 0, true},
		{"extract@", "", 0, true},
		{"extract@vX", "", 0, true},
		{"", "", 0, true},
	}
	for _, tc := range tests {
		name, version, err := ParsePromptSpec(tc.spec)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParsePromptSpec(%q): want error, got name=%q version=%d", tc.spec, name, version)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParsePromptSpec(%q): unexpected error: %v", tc.spec, err)
			continue
		}
		if name != tc.wantName || version != tc.wantVersion {
			t.Errorf("ParsePromptSpec(%q) = (%q, %d), want (%q, %d)", tc.spec, name, version, tc.wantName, tc.wantVersion)
		}
	}
}

func TestLoadPrompt_MissingFileIsHardError(t *testing.T) {
	_, _, err := LoadPrompt(t.TempDir(), "does-not-exist", 1)
	if err == nil {
		t.Fatal("want error for missing prompt file, got nil (fail-closed requirement)")
	}
}
