package campaign

import "testing"

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		country string
		want    string
		wantOK  bool
	}{
		{"leading 8 + 10 digits", "8 701 123 45 67", "7", "77011234567", true},
		{"already country-prefixed", "+7 701 123 45 67", "7", "77011234567", true},
		{"bare 10 digits gets country code", "701 123 45 67", "7", "77011234567", true},
		{"plausible foreign E.164 passes through", "+1 555 123 4567", "7", "15551234567", true},
		{"too short is invalid", "12345", "7", "", false},
		{"empty is invalid", "", "7", "", false},
		{"non-digit garbage is invalid", "abc", "7", "", false},
		{"too long is invalid", "1234567890123456", "7", "", false},
		{"punctuation stripped", "+7 (701) 111-11-11", "7", "77011111111", true},
		{"leading 8 with different default country still rewrites to that country", "8 701 123 45 67", "1", "17011234567", true},
		{"exactly min plausible length (7 digits, no default-country/10-digit match)", "1234567", "7", "1234567", true},
		{"exactly max plausible length (15 digits)", "123456789012345", "7", "123456789012345", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NormalizePhone(tt.raw, tt.country)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("NormalizePhone(%q, %q) = (%q, %v), want (%q, %v)", tt.raw, tt.country, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestParseRecipients_FlatSingleLinePaste(t *testing.T) {
	r := ParseRecipients("+7 701 111 11 11, +7 702 222 22 22, +7 701 111 11 11", ChannelWhatsApp, "7")
	if r.Total() != 3 {
		t.Fatalf("Total() = %d, want 3", r.Total())
	}
	if r.Valid != 2 {
		t.Errorf("Valid = %d, want 2", r.Valid)
	}
	if r.Duplicate != 1 {
		t.Errorf("Duplicate = %d, want 1", r.Duplicate)
	}
	if r.Invalid != 0 {
		t.Errorf("Invalid = %d, want 0", r.Invalid)
	}
	if r.Rows[2].Status != PreviewDuplicate {
		t.Errorf("Rows[2].Status = %q, want duplicate", r.Rows[2].Status)
	}
}

func TestParseRecipients_NewlineNoHeader(t *testing.T) {
	raw := "77011234567\n77021234567\n77011234567\nnot-a-number"
	r := ParseRecipients(raw, ChannelWhatsApp, "7")
	if r.Total() != 4 {
		t.Fatalf("Total() = %d, want 4", r.Total())
	}
	if r.Valid != 2 || r.Duplicate != 1 || r.Invalid != 1 {
		t.Errorf("counts = valid:%d duplicate:%d invalid:%d, want 2/1/1", r.Valid, r.Duplicate, r.Invalid)
	}
}

func TestParseRecipients_HeaderDetectedAndColumnsMapped(t *testing.T) {
	raw := "name,phone,city\nAigul,77011234567,Almaty\nBota,8 702 111 22 33,Astana"
	r := ParseRecipients(raw, ChannelWhatsApp, "7")
	if r.Total() != 2 {
		t.Fatalf("Total() = %d, want 2 (header row must not be counted as data)", r.Total())
	}
	if r.Valid != 2 {
		t.Fatalf("Valid = %d, want 2", r.Valid)
	}
	row0 := r.Rows[0]
	if row0.Name != "Aigul" {
		t.Errorf("Rows[0].Name = %q, want Aigul", row0.Name)
	}
	if row0.NormalizedIdentity != "77011234567" {
		t.Errorf("Rows[0].NormalizedIdentity = %q, want 77011234567", row0.NormalizedIdentity)
	}
	if row0.Attributes["city"] != "Almaty" {
		t.Errorf("Rows[0].Attributes[city] = %q, want Almaty", row0.Attributes["city"])
	}
	row1 := r.Rows[1]
	if row1.Name != "Bota" || row1.NormalizedIdentity != "77021112233" || row1.Attributes["city"] != "Astana" {
		t.Errorf("Rows[1] = %+v, unexpected", row1)
	}
}

func TestParseRecipients_HeaderColumnOrderDoesNotMatter(t *testing.T) {
	raw := "city,name,phone\nAlmaty,Aigul,77011234567"
	r := ParseRecipients(raw, ChannelWhatsApp, "7")
	if r.Total() != 1 || r.Valid != 1 {
		t.Fatalf("Total/Valid = %d/%d, want 1/1", r.Total(), r.Valid)
	}
	row := r.Rows[0]
	if row.Name != "Aigul" || row.NormalizedIdentity != "77011234567" || row.Attributes["city"] != "Almaty" {
		t.Errorf("row = %+v, unexpected", row)
	}
}

func TestParseRecipients_NoHeaderTwoColumnsIsPhoneThenName(t *testing.T) {
	raw := "77011234567,Aigul\n77021234567,Bota"
	r := ParseRecipients(raw, ChannelWhatsApp, "7")
	if r.Total() != 2 || r.Valid != 2 {
		t.Fatalf("Total/Valid = %d/%d, want 2/2", r.Total(), r.Valid)
	}
	if r.Rows[0].Name != "Aigul" || r.Rows[1].Name != "Bota" {
		t.Errorf("names = %q, %q, want Aigul, Bota", r.Rows[0].Name, r.Rows[1].Name)
	}
}

func TestParseRecipients_BlankLinesIgnored(t *testing.T) {
	raw := "77011234567\n\n\n77021234567\n"
	r := ParseRecipients(raw, ChannelWhatsApp, "7")
	if r.Total() != 2 {
		t.Fatalf("Total() = %d, want 2 (blank lines must not be counted)", r.Total())
	}
}

func TestParseRecipients_EmptyInput(t *testing.T) {
	r := ParseRecipients("", ChannelWhatsApp, "7")
	if r.Total() != 0 {
		t.Fatalf("Total() = %d, want 0", r.Total())
	}
}

func TestParseRecipients_TabAndSemicolonSeparators(t *testing.T) {
	tab := ParseRecipients("77011234567\t77021234567", ChannelWhatsApp, "7")
	if tab.Total() != 2 || tab.Valid != 2 {
		t.Errorf("tab-separated: total/valid = %d/%d, want 2/2", tab.Total(), tab.Valid)
	}
	semi := ParseRecipients("77011234567; 77021234567", ChannelWhatsApp, "7")
	if semi.Total() != 2 || semi.Valid != 2 {
		t.Errorf("semicolon-separated: total/valid = %d/%d, want 2/2", semi.Total(), semi.Valid)
	}
}

func TestParseRecipients_Telegram(t *testing.T) {
	raw := "123456789\n@somebody\n-100200300"
	r := ParseRecipients(raw, ChannelTelegram, "7")
	if r.Total() != 3 {
		t.Fatalf("Total() = %d, want 3", r.Total())
	}
	if r.Valid != 2 || r.Invalid != 1 {
		t.Fatalf("valid/invalid = %d/%d, want 2/1", r.Valid, r.Invalid)
	}
	if r.Rows[0].NormalizedIdentity != "123456789" {
		t.Errorf("Rows[0].NormalizedIdentity = %q, want 123456789", r.Rows[0].NormalizedIdentity)
	}
	if r.Rows[1].Status != PreviewInvalid {
		t.Errorf("Rows[1] (@somebody) status = %q, want invalid", r.Rows[1].Status)
	}
	if r.Rows[1].Reason == "" {
		t.Error("Rows[1] (@somebody) should carry a reason explaining the rejection")
	}
	if r.Rows[2].NormalizedIdentity != "-100200300" {
		t.Errorf("Rows[2] (group id) NormalizedIdentity = %q, want -100200300", r.Rows[2].NormalizedIdentity)
	}
}

func TestParseRecipients_MissingPhoneCellIsInvalidNotSkipped(t *testing.T) {
	raw := "name,phone\nAigul,\nBota,77021234567"
	r := ParseRecipients(raw, ChannelWhatsApp, "7")
	if r.Total() != 2 {
		t.Fatalf("Total() = %d, want 2 (a row with a blank phone cell must still be counted, as invalid)", r.Total())
	}
	if r.Invalid != 1 || r.Valid != 1 {
		t.Errorf("valid/invalid = %d/%d, want 1/1", r.Valid, r.Invalid)
	}
}

func TestApplyUnreachable(t *testing.T) {
	base := ParseRecipients("77011234567\n77021234567\n77031234567", ChannelWhatsApp, "7")
	if base.Valid != 3 {
		t.Fatalf("precondition: base.Valid = %d, want 3", base.Valid)
	}
	got := ApplyUnreachable(base, map[string]bool{"77021234567": true}, "not registered on WhatsApp")
	if got.Valid != 2 {
		t.Errorf("Valid = %d, want 2", got.Valid)
	}
	if got.Invalid != 1 {
		t.Errorf("Invalid = %d, want 1", got.Invalid)
	}
	if got.Rows[1].Status != PreviewInvalid || got.Rows[1].Reason != "not registered on WhatsApp" {
		t.Errorf("Rows[1] = %+v, want invalid with the given reason", got.Rows[1])
	}
	// Original result must be untouched (ApplyUnreachable returns a copy).
	if base.Valid != 3 {
		t.Errorf("original ParseResult mutated: Valid = %d, want 3", base.Valid)
	}
}

func TestApplyUnreachable_NoOpWhenEmpty(t *testing.T) {
	base := ParseRecipients("77011234567", ChannelWhatsApp, "7")
	got := ApplyUnreachable(base, nil, "unused")
	if got.Valid != base.Valid {
		t.Errorf("Valid = %d, want unchanged %d", got.Valid, base.Valid)
	}
}
