package whatsapp

import "testing"

func TestSendTargetPrefersKnownLID(t *testing.T) {
	got := SendTarget("521234567890@s.whatsapp.net", "5231387607239@lid")
	if got != "5231387607239@lid" {
		t.Fatalf("SendTarget = %q, want the known LID", got)
	}
}
