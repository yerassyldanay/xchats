package campaign

import "testing"

func TestCanTransition(t *testing.T) {
	allowed := map[[2]Status]bool{
		{StatusDraft, StatusScheduled}: true,
		{StatusDraft, StatusRunning}:   true,
		{StatusDraft, StatusCancelled}: true,

		{StatusScheduled, StatusRunning}:   true,
		{StatusScheduled, StatusCancelled}: true,

		{StatusRunning, StatusPaused}:    true,
		{StatusRunning, StatusCancelled}: true,
		{StatusRunning, StatusCompleted}: true,
		{StatusRunning, StatusFailed}:    true,

		{StatusPaused, StatusRunning}:   true,
		{StatusPaused, StatusCancelled}: true,
	}
	all := []Status{StatusDraft, StatusScheduled, StatusRunning, StatusPaused, StatusCompleted, StatusFailed, StatusCancelled}
	for _, from := range all {
		for _, to := range all {
			want := allowed[[2]Status{from, to}]
			got := CanTransition(from, to)
			if got != want {
				t.Errorf("CanTransition(%q, %q) = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestCanTransition_NoSelfLoops(t *testing.T) {
	for _, s := range []Status{StatusDraft, StatusScheduled, StatusRunning, StatusPaused, StatusCompleted, StatusFailed, StatusCancelled} {
		if CanTransition(s, s) {
			t.Errorf("CanTransition(%q, %q) = true, want false (no self-transition)", s, s)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := map[Status]bool{
		StatusDraft: false, StatusScheduled: false, StatusRunning: false, StatusPaused: false,
		StatusCompleted: true, StatusFailed: true, StatusCancelled: true,
	}
	for s, want := range terminal {
		if got := IsTerminal(s); got != want {
			t.Errorf("IsTerminal(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestValidStatus(t *testing.T) {
	for _, s := range []string{"draft", "scheduled", "running", "paused", "completed", "failed", "cancelled"} {
		if !ValidStatus(s) {
			t.Errorf("ValidStatus(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "active", "DRAFT", "pending"} {
		if ValidStatus(s) {
			t.Errorf("ValidStatus(%q) = true, want false", s)
		}
	}
}

func TestValidRecipientStatus(t *testing.T) {
	for _, s := range []string{"pending", "sending", "sent", "failed", "skipped"} {
		if !ValidRecipientStatus(s) {
			t.Errorf("ValidRecipientStatus(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "queued", "SENT"} {
		if ValidRecipientStatus(s) {
			t.Errorf("ValidRecipientStatus(%q) = true, want false", s)
		}
	}
}

func TestIsRecipientTerminal(t *testing.T) {
	terminal := map[RecipientStatus]bool{
		RecipientPending: false, RecipientSending: false,
		RecipientSent: true, RecipientFailed: true, RecipientSkipped: true,
	}
	for s, want := range terminal {
		if got := IsRecipientTerminal(s); got != want {
			t.Errorf("IsRecipientTerminal(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestCanEditContent(t *testing.T) {
	if !CanEditContent(0) {
		t.Error("CanEditContent(0) = false, want true")
	}
	if CanEditContent(1) {
		t.Error("CanEditContent(1) = true, want false")
	}
}

func TestCanEditPacing(t *testing.T) {
	editable := map[Status]bool{
		StatusDraft: true, StatusScheduled: true, StatusPaused: true,
		StatusRunning: false, StatusCompleted: false, StatusFailed: false, StatusCancelled: false,
	}
	for s, want := range editable {
		if got := CanEditPacing(s); got != want {
			t.Errorf("CanEditPacing(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestColdSendCapable(t *testing.T) {
	cold := map[string]bool{
		ChannelWhatsApp: true, ChannelSimulator: true,
		ChannelWhatsAppCloud: false, ChannelTelegram: false, ChannelInstagram: false, ChannelMessenger: false,
	}
	for ch, want := range cold {
		if got := ColdSendCapable(ch); got != want {
			t.Errorf("ColdSendCapable(%q) = %v, want %v", ch, got, want)
		}
	}
}
