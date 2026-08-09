package kbimport_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/kbimport"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
	"github.com/yerassyldanay/xchats/backend/llm"
)

type noopRegistry struct{}

func (noopRegistry) Client(llm.ModelRef) (llm.ChatClient, error) {
	return nil, errors.New("not configured in this test")
}

// newSubmitTestService builds a Service with no credentials configured
// (native works; firecrawl/llamaparse are always ErrProviderNotConfigured)
// unless creds is supplied. allowPrivate is threaded to Deps.AllowPrivateFetch
// — most tests need it true to reach an httptest.Server (a loopback
// address); the SSRF-rejection test needs it false to actually exercise the
// guard.
func newSubmitTestService(t *testing.T, creds *credentials.Chain, allowPrivate bool) (*kbimport.Service, uuid.UUID) {
	t.Helper()
	kb, st, _ := dbtest.NewKB(t)
	org, err := st.SeedOrganization(context.Background(), "xchats")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	diskStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob.NewDisk: %v", err)
	}
	settingsStore := settings.NewStore(t.TempDir())

	svc := kbimport.New(kbimport.Deps{
		KB: kb, Blob: diskStore, Settings: settingsStore, Credentials: creds, LLM: noopRegistry{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), AllowPrivateFetch: allowPrivate,
	}, kbimport.DefaultConfig())
	return svc, org.ID
}

func newFileCredChain(t *testing.T) *credentials.Chain {
	t.Helper()
	c, err := credentials.Open(credentials.OpenOptions{AllowFile: true, DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("credentials.Open: %v", err)
	}
	return c
}

func TestSubmit_RejectsUnknownTargetType(t *testing.T) {
	svc, orgID := newSubmitTestService(t, nil, true)
	_, err := svc.Submit(context.Background(), orgID, uuid.New(), kbimport.SubmitInput{
		Provider: "native", TargetType: "not-a-real-type", URLs: []string{"https://example.com/x"},
	})
	if !errors.Is(err, kbimport.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestSubmit_RejectsUnknownProvider(t *testing.T) {
	svc, orgID := newSubmitTestService(t, nil, true)
	_, err := svc.Submit(context.Background(), orgID, uuid.New(), kbimport.SubmitInput{
		Provider: "not-a-real-provider", TargetType: "auto", URLs: []string{"https://example.com/x"},
	})
	if !errors.Is(err, kbimport.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestSubmit_RejectsNoInput(t *testing.T) {
	svc, orgID := newSubmitTestService(t, nil, true)
	_, err := svc.Submit(context.Background(), orgID, uuid.New(), kbimport.SubmitInput{Provider: "native", TargetType: "auto"})
	if !errors.Is(err, kbimport.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestSubmit_RejectsTooManyURLs(t *testing.T) {
	svc, orgID := newSubmitTestService(t, nil, true)
	urls := make([]string, 11)
	for i := range urls {
		urls[i] = "https://example.com/x"
	}
	_, err := svc.Submit(context.Background(), orgID, uuid.New(), kbimport.SubmitInput{Provider: "native", TargetType: "auto", URLs: urls})
	if !errors.Is(err, kbimport.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestSubmit_RejectsSSRFTarget(t *testing.T) {
	svc, orgID := newSubmitTestService(t, nil, false)
	_, err := svc.Submit(context.Background(), orgID, uuid.New(), kbimport.SubmitInput{
		Provider: "native", TargetType: "auto", URLs: []string{"http://169.254.169.254/latest/meta-data/"},
	})
	if !errors.Is(err, kbimport.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestSubmit_RejectsPDFWithFirecrawlAndHintsLlamaParse(t *testing.T) {
	creds := newFileCredChain(t)
	if err := creds.Set(context.Background(), "firecrawl.api_key", "fc-key"); err != nil {
		t.Fatalf("seed credential: %v", err)
	}
	svc, orgID := newSubmitTestService(t, creds, true)
	_, err := svc.Submit(context.Background(), orgID, uuid.New(), kbimport.SubmitInput{
		Provider: "firecrawl", TargetType: "auto",
		Files: []kbimport.SubmitFile{{Filename: "price-list.pdf", MimeType: "application/pdf", Data: []byte("%PDF-1.4")}},
	})
	if !errors.Is(err, kbimport.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
	if err == nil || !strings.Contains(err.Error(), "LlamaParse") {
		t.Fatalf("err = %v, want a hint pointing at LlamaParse", err)
	}
}

func TestSubmit_MissingCredentialIs422(t *testing.T) {
	svc, orgID := newSubmitTestService(t, newFileCredChain(t), true) // credential store present but empty
	_, err := svc.Submit(context.Background(), orgID, uuid.New(), kbimport.SubmitInput{
		Provider: "firecrawl", TargetType: "auto", URLs: []string{"https://vendor.example/x"},
	})
	if !errors.Is(err, kbimport.ErrProviderNotConfigured) {
		t.Fatalf("err = %v, want ErrProviderNotConfigured", err)
	}
}

func TestSubmit_NativeNeedsNoCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<html><body>hi</body></html>")) }))
	defer srv.Close()
	svc, orgID := newSubmitTestService(t, nil, true)
	run, err := svc.Submit(context.Background(), orgID, uuid.New(), kbimport.SubmitInput{
		Provider: "native", TargetType: "auto", URLs: []string{srv.URL},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(run.Materials) != 1 {
		t.Fatalf("run.Materials = %+v, want 1", run.Materials)
	}
}

func TestSubmit_SecondConcurrentRunIs409(t *testing.T) {
	svc, orgID := newSubmitTestService(t, nil, true)
	userID := uuid.New()
	if _, err := svc.Submit(context.Background(), orgID, userID, kbimport.SubmitInput{
		Provider: "native", TargetType: "auto", URLs: []string{"https://example.com/first"},
	}); err != nil {
		t.Fatalf("first Submit: %v", err)
	}
	_, err := svc.Submit(context.Background(), orgID, userID, kbimport.SubmitInput{
		Provider: "native", TargetType: "auto", URLs: []string{"https://example.com/second"},
	})
	if !errors.Is(err, kbimport.ErrRunActive) {
		t.Fatalf("second Submit err = %v, want ErrRunActive", err)
	}
}

func TestSubmit_202Shape(t *testing.T) {
	svc, orgID := newSubmitTestService(t, nil, true)
	run, err := svc.Submit(context.Background(), orgID, uuid.New(), kbimport.SubmitInput{
		Provider: "native", TargetType: "products", Guidance: "Возьми название и цену",
		URLs: []string{"https://example.com/a", "https://example.com/b"},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if run.RunID == uuid.Nil {
		t.Error("expected a non-nil run id")
	}
	if run.Status != "extracting" {
		t.Errorf("Status = %q, want extracting", run.Status)
	}
	if len(run.Materials) != 2 {
		t.Fatalf("Materials = %+v, want 2", run.Materials)
	}
	for _, m := range run.Materials {
		if m.ProcessingStatus != kbstore.ImportStatusQueued {
			t.Errorf("material %s ProcessingStatus = %q, want queued", m.ID, m.ProcessingStatus)
		}
	}
}

func TestRunStatus_CrossOrgIs404(t *testing.T) {
	svc, orgID := newSubmitTestService(t, nil, true)
	run, err := svc.Submit(context.Background(), orgID, uuid.New(), kbimport.SubmitInput{
		Provider: "native", TargetType: "auto", URLs: []string{"https://example.com/a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.RunStatus(context.Background(), uuid.New(), run.RunID)
	if !errors.Is(err, kbimport.ErrNotFound) {
		t.Fatalf("cross-org RunStatus err = %v, want ErrNotFound", err)
	}
}

func TestRunStatus_UnknownRunIs404(t *testing.T) {
	svc, orgID := newSubmitTestService(t, nil, true)
	_, err := svc.RunStatus(context.Background(), orgID, uuid.New())
	if !errors.Is(err, kbimport.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
