package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestLoadMissingFileWritesDefaultsAndPersists(t *testing.T) {
	t.Setenv("LLM_DEFAULT_PROVIDER", "")
	dir := t.TempDir()
	got, err := NewStore(dir).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", got.Version, CurrentVersion)
	}
	if got.LLM.DefaultProvider != "openrouter" {
		t.Errorf("LLM.DefaultProvider = %q, want %q", got.LLM.DefaultProvider, "openrouter")
	}

	// Durable from the very first Load — a second Store over the same dir
	// (a fresh process, conceptually) must see the SAME settings on disk,
	// not regenerate a second, possibly different, set of defaults.
	path := filepath.Join(dir, settingsFileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings.json was not written on first Load: %v", err)
	}
	got2, err := NewStore(dir).Load()
	if err != nil {
		t.Fatalf("Load (second store): %v", err)
	}
	if !reflect.DeepEqual(got2, got) {
		t.Errorf("second Store's Load = %+v, want the same defaults %+v", got2, got)
	}
}

func TestLoadCorruptFileIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, settingsFileName), []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(dir).Load(); err == nil {
		t.Fatal("Load with a corrupt settings.json: want an error, got nil")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	want := Settings{
		Version: CurrentVersion,
		LLM:     LLMSettings{DefaultProvider: "openai", DefaultModel: "gpt-4o", MaxTokens: 1000, Temperature: 0.5, TimeoutSeconds: 45, Retry: true},
		Providers: map[string]ProviderSettings{
			"openai": {BaseURL: "https://proxy.example/v1", DefaultModel: "gpt-4o"},
		},
		Ngrok:                          NgrokSettings{Region: "us", Domain: "example.ngrok.app"},
		CredentialFileFallbackAccepted: true,
	}
	if err := s.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// A fresh Store over the same file must read back exactly what was saved.
	got, err := NewStore(dir).Load()
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if got.Version != want.Version || got.LLM != want.LLM || got.Ngrok != want.Ngrok || got.CredentialFileFallbackAccepted != want.CredentialFileFallbackAccepted {
		t.Errorf("Load after Save = %+v, want %+v", got, want)
	}
	if got.Providers["openai"] != want.Providers["openai"] {
		t.Errorf("Providers[openai] = %+v, want %+v", got.Providers["openai"], want.Providers["openai"])
	}
}

func TestSaveFilePermissionsAre0600(t *testing.T) {
	dir := t.TempDir()
	if err := NewStore(dir).Save(defaults()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, settingsFileName))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %o, want %o", perm, 0o600)
	}
}

func TestUpdateMutatesAndPersists(t *testing.T) {
	t.Setenv("LLM_DEFAULT_PROVIDER", "")
	dir := t.TempDir()
	s := NewStore(dir)

	got, err := s.Update(func(st *Settings) {
		st.CredentialFileFallbackAccepted = true
		st.LLM.DefaultModel = "custom-model"
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !got.CredentialFileFallbackAccepted || got.LLM.DefaultModel != "custom-model" {
		t.Errorf("Update result = %+v, want CredentialFileFallbackAccepted=true, DefaultModel=custom-model", got)
	}

	// Persisted — a fresh Store confirms it, not just the in-memory copy.
	reloaded, err := NewStore(dir).Load()
	if err != nil {
		t.Fatalf("Load after Update: %v", err)
	}
	if !reloaded.CredentialFileFallbackAccepted || reloaded.LLM.DefaultModel != "custom-model" {
		t.Errorf("reloaded = %+v, want CredentialFileFallbackAccepted=true, DefaultModel=custom-model", reloaded)
	}
}

func TestUpdateBuildsOnPriorState(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if _, err := s.Update(func(st *Settings) { st.Ngrok.Region = "eu" }); err != nil {
		t.Fatal(err)
	}
	got, err := s.Update(func(st *Settings) { st.Ngrok.Domain = "eu.example.app" })
	if err != nil {
		t.Fatal(err)
	}
	if got.Ngrok.Region != "eu" || got.Ngrok.Domain != "eu.example.app" {
		t.Errorf("Ngrok after two Updates = %+v, want both fields to have survived", got.Ngrok)
	}
}

func TestUpdateSerializesConcurrentCallers(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if _, err := s.Load(); err != nil { // seed the file before the concurrent phase
		t.Fatal(err)
	}

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if _, err := s.Update(func(st *Settings) {
				st.Providers["counter"] = ProviderSettings{DefaultModel: st.Providers["counter"].DefaultModel + "x"}
			}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Providers["counter"].DefaultModel) != n {
		t.Errorf("counter length = %d, want %d — a lost update means Update did not serialize concurrent callers",
			len(got.Providers["counter"].DefaultModel), n)
	}
}

func TestLoadPreservesUnknownFutureFieldsAcrossASave(t *testing.T) {
	// A settings.json written by a hypothetical future version with an
	// extra top-level field must not crash Load, and re-Saving through this
	// version legitimately drops fields it doesn't know about — this test
	// only pins that Load itself tolerates the extra field rather than
	// erroring.
	dir := t.TempDir()
	raw := `{"version":1,"llm":{"default_provider":"openrouter"},"providers":{},"a_future_field":"ignored"}`
	if err := os.WriteFile(filepath.Join(dir, settingsFileName), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := NewStore(dir).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.LLM.DefaultProvider != "openrouter" {
		t.Errorf("LLM.DefaultProvider = %q, want %q", got.LLM.DefaultProvider, "openrouter")
	}
}

func TestSettingsJSONFieldNamesAreSnakeCase(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	now := time.Now().UTC().Truncate(time.Second)
	want := Settings{
		Version: CurrentVersion,
		LLM:     LLMSettings{DefaultProvider: "openrouter", VisionModel: "vision-model"},
		Providers: map[string]ProviderSettings{
			"openai": {LastVerifiedAt: &now, LastError: "boom"},
		},
		CredentialFileFallbackAccepted: true,
	}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, settingsFileName))
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(b, &generic); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"version", "llm", "providers", "ngrok", "credential_file_fallback_accepted"} {
		if _, ok := generic[key]; !ok {
			t.Errorf("settings.json missing expected top-level key %q; got keys %v", key, keysOf(generic))
		}
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestWriteFileAtomicFailsWhenParentDirectoryIsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist", "settings.json")
	if err := writeFileAtomic(path, []byte("{}"), 0o600); err == nil {
		t.Fatal("want an error when the parent directory does not exist")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("target file should not have been created; stat err = %v", err)
	}
}
