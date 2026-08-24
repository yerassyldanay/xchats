package campaign

import "testing"

func TestValidateName(t *testing.T) {
	if err := ValidateName("Summer promo"); err != nil {
		t.Errorf("ValidateName(non-empty) = %v, want nil", err)
	}
	for _, bad := range []string{"", "   "} {
		if err := ValidateName(bad); err == nil {
			t.Errorf("ValidateName(%q) = nil, want an error", bad)
		}
	}
}

func TestValidateMessageBody(t *testing.T) {
	if err := ValidateMessageBody("Hello {{name}}"); err != nil {
		t.Errorf("ValidateMessageBody(non-empty) = %v, want nil", err)
	}
	for _, bad := range []string{"", "  \n "} {
		if err := ValidateMessageBody(bad); err == nil {
			t.Errorf("ValidateMessageBody(%q) = nil, want an error", bad)
		}
	}
}

func TestValidateCountryCode(t *testing.T) {
	for _, ok := range []string{"7", "1", "994", "44"} {
		if err := ValidateCountryCode(ok); err != nil {
			t.Errorf("ValidateCountryCode(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "+7", "7a", "12345"} {
		if err := ValidateCountryCode(bad); err == nil {
			t.Errorf("ValidateCountryCode(%q) = nil, want an error", bad)
		}
	}
}

func TestValidatePace(t *testing.T) {
	tests := []struct {
		name             string
		interval, jitter int
		wantErr          bool
	}{
		{"defaults are valid", DefaultMinIntervalSeconds, DefaultJitterSeconds, false},
		{"zero jitter is valid", 90, 0, false},
		{"jitter equal to interval is valid", 90, 90, false},
		{"interval below the floor", 0, 0, true},
		{"interval above the ceiling", MaxIntervalSecondsBounds + 1, 0, true},
		{"negative jitter", 90, -1, true},
		{"jitter exceeds interval", 90, 91, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePace(tt.interval, tt.jitter)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePace(%d, %d) = %v, wantErr %v", tt.interval, tt.jitter, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTier(t *testing.T) {
	tests := []struct {
		name    string
		tier    Tier
		wantErr bool
	}{
		{"a default tier is valid", Tier{WindowSeconds: 3600, MaxSends: 5}, false},
		{"window too short", Tier{WindowSeconds: 1, MaxSends: 5}, true},
		{"window too long", Tier{WindowSeconds: MaxTierWindowSeconds + 1, MaxSends: 5}, true},
		{"zero max sends", Tier{WindowSeconds: 3600, MaxSends: 0}, true},
		{"negative max sends", Tier{WindowSeconds: 3600, MaxSends: -1}, true},
		{"max sends too large", Tier{WindowSeconds: 3600, MaxSends: MaxTierMaxSends + 1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTier(tt.tier)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTier(%+v) = %v, wantErr %v", tt.tier, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTiers(t *testing.T) {
	if err := ValidateTiers(DefaultTiers); err != nil {
		t.Errorf("ValidateTiers(DefaultTiers) = %v, want nil", err)
	}
	if err := ValidateTiers(nil); err != nil {
		t.Errorf("ValidateTiers(nil) = %v, want nil (the simulator has zero tiers)", err)
	}
	dup := []Tier{{WindowSeconds: 3600, MaxSends: 5}, {WindowSeconds: 3600, MaxSends: 10}}
	if err := ValidateTiers(dup); err == nil {
		t.Error("ValidateTiers(duplicate window_seconds) = nil, want an error")
	}
	bad := []Tier{{WindowSeconds: 3600, MaxSends: 5}, {WindowSeconds: 0, MaxSends: 5}}
	if err := ValidateTiers(bad); err == nil {
		t.Error("ValidateTiers(one invalid tier) = nil, want an error")
	}
}
