package kbimport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/extractor"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// tinyPNG is the minimal byte sequence http.DetectContentType recognizes
// as image/png.
var tinyPNG = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}

func TestDownloadEmbeddedImages_CapsAndDedupes(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(tinyPNG)
	}))
	defer srv.Close()

	client := &scriptedClient{}
	svc, kb, orgID, _ := newTestService(t, client)
	svc.cfg.MaxImagesPerJob = 2

	images := []extractor.Image{
		{URL: srv.URL + "/a.png"},
		{URL: srv.URL + "/a.png"}, // duplicate — must be fetched once
		{URL: srv.URL + "/b.png"},
		{URL: srv.URL + "/c.png"}, // over the MaxImagesPerJob=2 cap
	}
	runBudget := svc.cfg.MaxRunImageBytes
	runID := uuid.New()
	parentID := uuid.New()

	staged := svc.downloadEmbeddedImages(context.Background(), orgID, runID, parentID, images, &runBudget)

	if len(staged) != 2 {
		t.Fatalf("staged %d images, want exactly MaxImagesPerJob=2", len(staged))
	}
	if requests != 2 {
		t.Fatalf("server received %d requests, want 2 (a.png once, b.png once — c.png never reached, duplicate a.png never re-fetched)", requests)
	}

	materials, err := kb.ImportRunMaterials(context.Background(), orgID, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(materials) != 2 {
		t.Fatalf("materials = %+v, want 2 tagged", materials)
	}
	for _, m := range materials {
		if m.CustomerVisibility != "auto" {
			t.Errorf("material %s CustomerVisibility = %q, want auto", m.ID, m.CustomerVisibility)
		}
		if m.Params.ParentID == nil || *m.Params.ParentID != parentID {
			t.Errorf("material %s ParentID = %v, want %s", m.ID, m.Params.ParentID, parentID)
		}
		if m.Params.RunID != runID {
			t.Errorf("material %s RunID = %s, want %s", m.ID, m.Params.RunID, runID)
		}
	}
}

func TestDownloadEmbeddedImages_RunBudgetExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(tinyPNG)
	}))
	defer srv.Close()

	svc, kb, orgID, _ := newTestService(t, &scriptedClient{})
	svc.cfg.MaxImagesPerJob = 10

	images := []extractor.Image{{URL: srv.URL + "/a.png"}, {URL: srv.URL + "/b.png"}}
	var runBudget int64 = 0 // already exhausted
	staged := svc.downloadEmbeddedImages(context.Background(), orgID, uuid.New(), uuid.New(), images, &runBudget)
	if len(staged) != 0 {
		t.Fatalf("staged %+v, want none — run image budget was already exhausted", staged)
	}
	_ = kb
}

func TestDownloadOneImage_RejectsSVGMasqueradingAsPNG(t *testing.T) {
	svgBody := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png") // lies: this is not a PNG
		_, _ = w.Write(svgBody)
	}))
	defer srv.Close()

	svc, _, orgID, _ := newTestService(t, &scriptedClient{})
	_, _, err := svc.downloadOneImage(context.Background(), orgID, srv.URL+"/x.png", "", svc.cfg.MaxImageBytes)
	if err == nil {
		t.Fatal("expected the mismatched declared/sniffed content type to be rejected")
	}
}

func TestDownloadOneImage_RejectsHTMLMasqueradingAsImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("<html><body><script>alert(1)</script></body></html>"))
	}))
	defer srv.Close()

	svc, _, orgID, _ := newTestService(t, &scriptedClient{})
	_, _, err := svc.downloadOneImage(context.Background(), orgID, srv.URL+"/x.jpg", "", svc.cfg.MaxImageBytes)
	if err == nil {
		t.Fatal("expected HTML content declared as an image to be rejected")
	}
}

func TestDownloadOneImage_RejectsNonImageMimeType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4 not really"))
	}))
	defer srv.Close()

	svc, _, orgID, _ := newTestService(t, &scriptedClient{})
	_, _, err := svc.downloadOneImage(context.Background(), orgID, srv.URL+"/x.pdf", "", svc.cfg.MaxImageBytes)
	if err == nil {
		t.Fatal("expected a non-image mime type to be rejected even though it matches its own declared type")
	}
}

func TestDownloadOneImage_RejectsSSRFTarget(t *testing.T) {
	svc, _, orgID, _ := newTestService(t, &scriptedClient{})
	svc.deps.AllowPrivateFetch = false
	_, _, err := svc.downloadOneImage(context.Background(), orgID, "http://169.254.169.254/latest/meta-data/", "", svc.cfg.MaxImageBytes)
	if err == nil {
		t.Fatal("expected a link-local metadata-endpoint URL to be rejected before any fetch")
	}
}

// TestDownloadOneImage_StagedImagePassesValidateMediaRef proves a staged
// embedded image is immediately usable in a media field (customer_visibility
// "auto" passes validateMediaRef — only "invisible" is rejected there) by
// running it through the exact same MCPUpsertProduct path pass 2 uses.
func TestDownloadOneImage_StagedImagePassesValidateMediaRef(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(tinyPNG)
	}))
	defer srv.Close()

	svc, kb, orgID, userID := newTestService(t, &scriptedClient{})
	id, _, err := svc.downloadOneImage(context.Background(), orgID, srv.URL+"/photo.png", "alt text", svc.cfg.MaxImageBytes)
	if err != nil {
		t.Fatalf("downloadOneImage: %v", err)
	}

	res, err := kb.MCPUpsertProduct(context.Background(), orgID, userID, "widget-1",
		kbstore.ProductChanges{Name: strPtr("Товар"), AvailabilityStatus: strPtr("in_stock"), FeaturedImage: uuidPtrPtr(id)},
		nil, kbstore.MCPProvenance{})
	if err != nil {
		t.Fatalf("MCPUpsertProduct with a staged embedded image: %v", err)
	}
	if res.Key != "widget-1" {
		t.Fatalf("res = %+v", res)
	}
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
func uuidPtrPtr(id uuid.UUID) **uuid.UUID {
	p := &id
	return &p
}
