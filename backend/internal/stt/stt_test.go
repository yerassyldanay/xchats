package stt

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// parsedRequest is what each test extracts from the multipart body a
// Transcribe call sent, so assertions read as plain field comparisons
// instead of re-parsing multipart.Reader boilerplate per test.
type parsedRequest struct {
	model, language, prompt, filename, fileContent string
}

func parseMultipart(t *testing.T, r *http.Request) parsedRequest {
	t.Helper()
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content-type: %v", err)
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	var got parsedRequest
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		data, _ := io.ReadAll(part)
		switch part.FormName() {
		case "model":
			got.model = string(data)
		case "language":
			got.language = string(data)
		case "prompt":
			got.prompt = string(data)
		case "file":
			got.filename = part.FileName()
			got.fileContent = string(data)
		}
	}
	return got
}

// TestTranscribe_WireShape pins the multipart request fields every
// OpenAI-compatible /audio/transcriptions endpoint (OpenAI itself, Groq)
// expects: model, language, prompt, and the audio bytes under "file".
func TestTranscribe_WireShape(t *testing.T) {
	var got parsedRequest
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		got = parseMultipart(t, r)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"text":"здравствуйте, сколько стоит доставка"}`)
	}))
	defer srv.Close()

	c := NewOpenAITranscriber(srv.URL, "test-key", "whisper-1", 5*time.Second)
	text, err := c.Transcribe(context.Background(), []byte("fake-audio-bytes"), "voice.ogg", "audio/ogg", TranscribeOptions{
		Language: "ru",
		Prompt:   "iPhone, Kaspi, Kolesa",
	})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if text != "здравствуйте, сколько стоит доставка" {
		t.Errorf("text = %q", text)
	}
	if gotPath != "/audio/transcriptions" {
		t.Errorf("path = %q, want /audio/transcriptions", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("authorization = %q", gotAuth)
	}
	if got.model != "whisper-1" {
		t.Errorf("model = %q, want whisper-1", got.model)
	}
	if got.language != "ru" {
		t.Errorf("language = %q, want ru", got.language)
	}
	if got.prompt != "iPhone, Kaspi, Kolesa" {
		t.Errorf("prompt = %q", got.prompt)
	}
	if got.filename != "voice.ogg" || got.fileContent != "fake-audio-bytes" {
		t.Errorf("file = %q/%q, want voice.ogg/fake-audio-bytes", got.filename, got.fileContent)
	}
}

// TestTranscribe_OmitsEmptyLanguageAndPrompt asserts an unset language hint
// or vocabulary prompt is left off the request entirely (auto-detect),
// rather than sent as an empty field some providers might reject.
func TestTranscribe_OmitsEmptyLanguageAndPrompt(t *testing.T) {
	var sawLanguage, sawPrompt bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			switch part.FormName() {
			case "language":
				sawLanguage = true
			case "prompt":
				sawPrompt = true
			}
		}
		io.WriteString(w, `{"text":"ok"}`)
	}))
	defer srv.Close()

	c := NewOpenAITranscriber(srv.URL, "k", "whisper-1", 5*time.Second)
	if _, err := c.Transcribe(context.Background(), []byte("x"), "a.ogg", "audio/ogg", TranscribeOptions{}); err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if sawLanguage {
		t.Error("must not send a language field when Language is unset")
	}
	if sawPrompt {
		t.Error("must not send a prompt field when Prompt is unset")
	}
}

// TestTranscribe_DefaultsFilenameFromMime covers a voice note whose channel
// supplied no filename at all (common for WhatsApp/Telegram voice notes) —
// the upload must still carry a recognizable extension.
func TestTranscribe_DefaultsFilenameFromMime(t *testing.T) {
	var gotFilename string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := parseMultipart(t, r)
		gotFilename = got.filename
		io.WriteString(w, `{"text":"ok"}`)
	}))
	defer srv.Close()

	c := NewOpenAITranscriber(srv.URL, "k", "whisper-1", 5*time.Second)
	if _, err := c.Transcribe(context.Background(), []byte("x"), "", "audio/ogg", TranscribeOptions{}); err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if !strings.HasSuffix(gotFilename, ".ogg") {
		t.Errorf("filename = %q, want an .ogg extension", gotFilename)
	}
}

func TestTranscribe_ClassifiesUnauthorizedAsErrProviderAuth(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			io.WriteString(w, `{"error":{"message":"invalid api key"}}`)
		}))
		c := NewOpenAITranscriber(srv.URL, "k", "whisper-1", 5*time.Second)
		_, err := c.Transcribe(context.Background(), []byte("x"), "a.ogg", "audio/ogg", TranscribeOptions{})
		if !errors.Is(err, ErrProviderAuth) {
			t.Errorf("status %d: err = %v, want errors.Is(err, ErrProviderAuth)", status, err)
		}
		srv.Close()
	}
}

func TestTranscribe_PropagatesProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"error":{"message":"file too large"}}`)
	}))
	defer srv.Close()

	c := NewOpenAITranscriber(srv.URL, "k", "whisper-1", 5*time.Second)
	_, err := c.Transcribe(context.Background(), []byte("x"), "a.ogg", "audio/ogg", TranscribeOptions{})
	if err == nil {
		t.Fatal("want an error for a provider error field")
	}
}
