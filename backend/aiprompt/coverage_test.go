package aiprompt

import (
	"strings"
	"testing"
)

// TestBuildCatalog_InvalidRef covers validRef's rejection branch: refs must
// be lowercase snake/kebab-ish and must never contain a dot (dots are the
// media-token segment separator).
func TestBuildCatalog_InvalidRef(t *testing.T) {
	cases := []string{"Coffee Machine!", "coffee.machine", "", "UPPER-CASE"}
	for _, ref := range cases {
		t.Run(ref, func(t *testing.T) {
			kb := baseKB()
			kb.Products[0].Ref = ref
			kb.Products[0].FeaturedImage = ""
			kb.Products[0].GalleryImages = nil
			if _, err := BuildCatalog(kb); err == nil {
				t.Fatalf("expected BuildCatalog to reject ref %q", ref)
			}
		})
	}
}

// TestBuildCatalog_MimeMismatchAcrossKinds exercises mimeMatches' video and
// document branches (image is already covered elsewhere).
func TestBuildCatalog_MimeMismatchAcrossKinds(t *testing.T) {
	cases := []struct {
		name   string
		column string
		mime   string
	}{
		{"video column gets an image", "demo_videos", "image/png"},
		{"document column gets a video", "certificate_documents", "video/mp4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kb := baseKB()
			matID := "m-mismatch"
			mat := testMaterial(matID)
			mat.MimeType = tc.mime
			kb.Materials = append(kb.Materials, mat)
			switch tc.column {
			case "demo_videos":
				kb.Products[0].DemoVideos = []string{matID}
			case "certificate_documents":
				kb.Products[0].CertificateDocuments = []string{matID}
			}
			if _, err := BuildCatalog(kb); err == nil {
				t.Fatalf("expected a MIME-mismatch error for %s with %s", tc.column, tc.mime)
			}
		})
	}
}

// TestBuildCatalog_DocumentAndVideoAccepted confirms valid non-image kinds
// (document and video) resolve successfully — the fail-closed tests only
// ever broke image columns; this proves the happy path for the other kinds.
func TestBuildCatalog_DocumentAndVideoAccepted(t *testing.T) {
	kb := baseKB()
	video := testMaterial("m-video")
	video.MimeType = "video/mp4"
	video.Filename = "demo.mp4"
	doc := testDocMaterial("m-cert")
	kb.Materials = append(kb.Materials, video, doc)
	kb.Products[0].DemoVideos = []string{"m-video"}
	kb.Products[0].CertificateDocuments = []string{"m-cert"}

	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	if cat.MediaByToken("products.coffee-machine.demo_videos") == nil {
		t.Error("expected demo_videos token")
	}
	if cat.MediaByToken("products.coffee-machine.certificate_documents") == nil {
		t.Error("expected certificate_documents token")
	}
}

// TestRenderMediaCatalogAndAbsent_EmptyDash: with no topics/products/tariffs/
// contacts/policies at all, both the media catalog and the absence list must
// render the placeholder dash, never an empty string that could be confused
// with a leftover slot.
func TestRenderMediaCatalogAndAbsent_EmptyDash(t *testing.T) {
	kb := &KB{OrganizationID: testOrg}
	prompt, _, err := BuildPrompt(basicFrame, kb)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if got := renderMediaCatalog(nil); got != "—" {
		t.Errorf("renderMediaCatalog(nil) = %q, want —", got)
	}
	if got := renderMediaAbsent(nil); got != "—" {
		t.Errorf("renderMediaAbsent(nil) = %q, want —", got)
	}
	_ = prompt // BuildPrompt succeeding at all (with an entirely empty KB) is itself the interesting assertion
}

// TestValidatePrompt_RegexLeakGates exercises the UUID and file-extension
// defense-in-depth regexes directly against arbitrary text that was never
// backed by a real Material — these gates fire on shape alone.
func TestValidatePrompt_RegexLeakGates(t *testing.T) {
	cat := &Catalog{}
	if err := ValidatePrompt("some text with a uuid 123e4567-e89b-12d3-a456-426614174000 in it", cat); err == nil {
		t.Error("expected the UUID-shaped regex gate to fire")
	}
	if err := ValidatePrompt("here is a file photo.jpg attached", cat); err == nil {
		t.Error("expected the file-extension regex gate to fire")
	}
	if err := ValidatePrompt("perfectly ordinary Russian prompt text", cat); err != nil {
		t.Errorf("clean text must pass, got: %v", err)
	}
}

// TestBuildCatalog_ReturnPeriodUsageNote exercises the KindNumber usage-note
// branch (return_period_in_days is the only KindNumber fact column).
func TestBuildCatalog_ReturnPeriodUsageNote(t *testing.T) {
	kb := baseKB()
	kb.Policies.ReturnPeriodInDays = "14"
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	f := cat.FactByToken("{{policy.main.return_period_in_days}}")
	if f == nil {
		t.Fatal("expected a return_period_in_days fact")
	}
	if f.UsageNote == "" {
		t.Error("expected a non-empty usage note for a number-kind fact")
	}
}

// TestUsageNote_DeliveryFlagDerivedFromWordingMap locks the KB-driven property
// of deliveryUsageGuidance: the note must be built FROM DeliveryWordingRU's map
// values (via lookup), never a wording retyped as a separate string literal —
// a value the eval-design principles require ("prompts are built from KB
// records, never an eval-only representation," applied here to the code-owned
// wording registry instead of a fixture). The note must still carry the real
// outside_zones_note token (the one legitimate brace pair) and both
// DeliveryWordingRU values, via map lookup.
func TestUsageNote_DeliveryFlagDerivedFromWordingMap(t *testing.T) {
	note := usageNote(FactColumn{Kind: KindDeliveryFlag})
	for _, want := range []string{DeliveryWordingRU[true], DeliveryWordingRU[false]} {
		if !strings.Contains(note, want) {
			t.Errorf("delivery usage note = %q, want it to contain wording-map value %q", note, want)
		}
	}
	if !strings.Contains(note, "{{policy.main.outside_zones_note}}") {
		t.Errorf("delivery usage note = %q, want it to still carry the outside_zones_note token", note)
	}
}

// TestMediaColumnSpec_UnknownTableAndColumn exercises both error branches of
// the closed media-column registry lookup used by ResolveSend.
func TestMediaColumnSpec_UnknownTableAndColumn(t *testing.T) {
	if _, err := mediaColumnSpec("not-a-table", "featured_image"); err == nil {
		t.Error("expected an error for an unknown table")
	}
	if _, err := mediaColumnSpec("products", "not-a-column"); err == nil {
		t.Error("expected an error for an unknown column")
	}
}

// TestBuildPrompt_PropagatesCatalogError confirms BuildPrompt surfaces
// BuildCatalog's error directly rather than masking it.
func TestBuildPrompt_PropagatesCatalogError(t *testing.T) {
	kb := baseKB()
	kb.Products[0].FeaturedImage = "does-not-exist"
	if _, _, err := BuildPrompt(basicFrame, kb); err == nil {
		t.Fatal("expected BuildPrompt to fail when BuildCatalog fails")
	}
}
