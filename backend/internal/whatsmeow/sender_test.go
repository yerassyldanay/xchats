package whatsmeow

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func TestParseSendTargetPreservesLID(t *testing.T) {
	got, err := parseSendTarget("5231387607239@lid")
	if err != nil {
		t.Fatalf("parse send target: %v", err)
	}
	if got.User != "5231387607239" || got.Server != types.HiddenUserServer {
		t.Fatalf("parsed destination = %s, want the original LID", got)
	}
}
