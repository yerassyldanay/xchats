package evaltext

import "testing"

func TestNormalizeText(t *testing.T) {
	tests := []struct{ in, want string }{
		{"  Старт   10 000 ₸/мес ", "старт 10 000 ₸/мес"},
		{"MiXeD\nCase\twith\n\nwhitespace", "mixed case with whitespace"},
		{"", ""},
		// ё/е is a genuine, common Russian orthographic equivalence (diaeresis routinely
		// dropped in casual/printed text) -- a model choosing either spelling of a name
		// like "Пётр"/"Петр" must not be penalized as if it wrote a different name.
		{"Петров Пётр Петрович", "петров петр петрович"},
	}
	for _, tc := range tests {
		if got := NormalizeText(tc.in); got != tc.want {
			t.Errorf("NormalizeText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCanonicalNumber(t *testing.T) {
	tests := []struct{ in, want string }{
		{"25 000", "25000"},
		{"25,000", "25000"},
		{"25-000", "25000"},
		{"25000", "25000"},
	}
	for _, tc := range tests {
		if got := CanonicalNumber(tc.in); got != tc.want {
			t.Errorf("CanonicalNumber(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFoldNBSP_PreservesNewlinesUnlikeNormalizeText(t *testing.T) {
	in := "77/7 line\nsecond line"
	got := FoldNBSP(in)
	want := "77/7 line\nsecond line"
	if got != want {
		t.Errorf("FoldNBSP(%q) = %q, want %q", in, got, want)
	}
}

func TestStripFences(t *testing.T) {
	tests := []struct{ in, want string }{
		{`{"a":1}`, `{"a":1}`},
		{"```json\n{\"a\":1}\n```", `{"a":1}`},
		{"```\n{\"a\":1}\n```", `{"a":1}`},
		{"  ```json  \n{\"a\":1}\n  ```  ", `{"a":1}`},
	}
	for _, tc := range tests {
		if got := StripFences(tc.in); got != tc.want {
			t.Errorf("StripFences(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
