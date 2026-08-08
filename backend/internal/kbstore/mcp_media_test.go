package kbstore_test

import (
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// TestMediaColumnKind_ClosedList pins the exact field->kind registry
// MediaFieldKinds serves (mcp_media.go's mediaColumnKind, built from
// mediaAttachmentFields + derivedMediaColumnKind) — a closed list, not a
// subset check, so adding, removing, or reclassifying a media column here
// without mirroring the change in the frontend registry
// (frontend/src/components/kb/records/shared.ts's KB_MEDIA_FIELDS) fails
// loudly right here, at the source of truth, instead of the frontend
// silently rendering nothing (or the wrong icon) for the new field.
func TestMediaColumnKind_ClosedList(t *testing.T) {
	const mirrorHint = "mirror this change in frontend/src/components/kb/records/shared.ts's KB_MEDIA_FIELDS"

	want := map[string]string{
		"featured_image":            "image",
		"illustration_images":       "image",
		"explainer_videos":          "video",
		"reference_documents":       "document",
		"gallery_images":            "image",
		"demo_videos":               "video",
		"certificate_documents":     "document",
		"guarantee_documents":       "document",
		"pricing_images":            "image",
		"terms_documents":           "document",
		"contact_card_image":        "image",
		"location_map_image":        "image",
		"company_legal_documents":   "document",
		"commerce_policy_documents": "document",
	}

	got := kbstore.MediaFieldKinds()
	for field, wantKind := range want {
		gotKind, ok := got[field]
		if !ok {
			t.Errorf("kbstore.MediaFieldKinds() is missing field %q (want kind %q) — %s", field, wantKind, mirrorHint)
			continue
		}
		if gotKind != wantKind {
			t.Errorf("kbstore.MediaFieldKinds()[%q] = %q, want %q — %s", field, gotKind, wantKind, mirrorHint)
		}
	}
	for field, gotKind := range got {
		if _, ok := want[field]; !ok {
			t.Errorf("kbstore.MediaFieldKinds() has an unpinned field %q (kind %q) — %s, then add it to this test too", field, gotKind, mirrorHint)
		}
	}
}
