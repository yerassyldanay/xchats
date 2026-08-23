package campaign

import "testing"

func TestNextRetry(t *testing.T) {
	cases := []struct {
		attempt  int
		wantOK   bool
		wantWait string
	}{
		{0, false, ""},
		{1, true, "1m0s"},
		{2, true, "5m0s"},
		{3, true, "25m0s"},
		{4, false, ""},
		{5, false, ""},
	}
	for _, tc := range cases {
		wait, ok := NextRetry(tc.attempt)
		if ok != tc.wantOK {
			t.Fatalf("NextRetry(%d) ok = %v, want %v", tc.attempt, ok, tc.wantOK)
		}
		if ok && wait.String() != tc.wantWait {
			t.Fatalf("NextRetry(%d) wait = %v, want %v", tc.attempt, wait, tc.wantWait)
		}
	}
}
