// Package stt defines the provider-neutral speech-to-text contract the
// worker's audio-transcription step calls through, mirroring backend/llm's
// own split between a dependency-free contract and a concrete provider
// adapter: OpenAITranscriber is the one implementation this package needs,
// since OpenAI and Groq both serve the identical multipart
// /audio/transcriptions endpoint, differing only in base URL, key, and
// model. It has no dependency on backend/llm — an LLM chat completion and an
// audio transcription are different provider capabilities (a provider can
// serve one without the other, e.g. OpenRouter and Gemini have no
// OpenAI-wire-compatible transcription endpoint at all), so the two
// contracts are kept independent rather than sharing a Registry.
package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// ErrProviderAuth is the sentinel a Transcriber wraps into its returned
// error when a provider rejects a call as unauthorized (an invalid or
// revoked API key) — the stt-package counterpart to llm.ErrProviderAuth.
var ErrProviderAuth = errors.New("stt: provider rejected the request as unauthorized")

// TranscribeOptions customizes one transcription call.
type TranscribeOptions struct {
	// Language is an ISO-639-1 hint ("kk", "ru", "en") or "" for
	// auto-detect. A hint measurably improves accuracy on a short voice
	// note where the model would otherwise have too little audio to
	// confidently detect the language itself.
	Language string
	// Prompt biases transcription toward domain jargon the model would
	// otherwise mis-hear: product names, brand names, local terms — see
	// BuildPrompt. Whisper-family models only look at roughly the last 224
	// tokens of this field, so callers should keep it short and relevant
	// rather than dumping an entire catalog.
	Prompt string
}

// Transcriber turns one audio attachment's bytes into text.
type Transcriber interface {
	Transcribe(ctx context.Context, audio []byte, filename, mime string, opts TranscribeOptions) (string, error)
}

// Params is the live, per-attempt STT configuration a caller resolves fresh
// on every transcription attempt — mirrors response.Engine.Params, so a
// Settings UI change (provider, model, language hint, vocabulary) takes
// effect on the very next voice note with no restart.
type Params struct {
	// Transcriber is nil when STT is not configured at all (no provider or
	// model selected, or no credential resolves for the selected provider)
	// — the caller then skips transcription entirely rather than erroring.
	Transcriber Transcriber
	Language    string
	Vocabulary  string
}

// DefaultBaseURL returns the standard base URL for a named STT provider.
// Only "openai" and "groq" genuinely serve the OpenAI-wire-compatible
// /audio/transcriptions multipart endpoint this package's client calls —
// OpenRouter is a text-completion router with no audio endpoint at all, and
// Gemini's multimodal audio input uses a different (non-multipart) wire
// shape, so neither belongs in this switch. An unrecognized provider name
// falls back to OpenAI's URL, exactly as DefaultBaseURL(provider) is only
// ever called after settings validation has already rejected anything but
// those two ids (see internal/httpapi/settings.go).
func DefaultBaseURL(provider string) string {
	if strings.ToLower(provider) == "groq" {
		return "https://api.groq.com/openai/v1"
	}
	return "https://api.openai.com/v1"
}

// OpenAITranscriber calls one OpenAI-wire-compatible /audio/transcriptions
// endpoint (OpenAI itself, or Groq's identical surface).
type OpenAITranscriber struct {
	httpc   *http.Client
	baseURL string
	apiKey  string
	model   string
}

// NewOpenAITranscriber builds a client for one provider's transcription
// endpoint. timeout bounds each Transcribe call — a voice note is a small
// upload and a short synchronous wait, so one deadline covering the whole
// request (unlike llmprovider's ctx-plus-per-attempt-timeout split, which
// exists to bound a SEPARATE retry attempt) is enough here; there is no
// transcription retry.
func NewOpenAITranscriber(baseURL, apiKey, model string, timeout time.Duration) *OpenAITranscriber {
	return &OpenAITranscriber{
		httpc:   &http.Client{Timeout: timeout},
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
	}
}

// maxTranscriptionResponseBody caps how much of a transcription response
// this client ever reads — generous for even a long voice note's text, never
// enough for a misbehaving endpoint to exhaust memory (mirrors
// llmprovider.OpenAICompatible's identical 4<<20 cap on a chat response).
const maxTranscriptionResponseBody = 4 << 20

type transcriptionResponse struct {
	Text  string `json:"text"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Transcribe implements Transcriber: it posts the audio bytes as a
// multipart/form-data upload (field "file"), the same shape every
// OpenAI-compatible transcription endpoint expects.
func (c *OpenAITranscriber) Transcribe(ctx context.Context, audio []byte, filename, mimetype string, opts TranscribeOptions) (string, error) {
	if filename == "" {
		filename = "audio" + extensionForMime(mimetype)
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("stt: build request: %w", err)
	}
	if _, err := part.Write(audio); err != nil {
		return "", fmt.Errorf("stt: build request: %w", err)
	}
	if err := w.WriteField("model", c.model); err != nil {
		return "", fmt.Errorf("stt: build request: %w", err)
	}
	if opts.Language != "" {
		if err := w.WriteField("language", opts.Language); err != nil {
			return "", fmt.Errorf("stt: build request: %w", err)
		}
	}
	if opts.Prompt != "" {
		if err := w.WriteField("prompt", opts.Prompt); err != nil {
			return "", fmt.Errorf("stt: build request: %w", err)
		}
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("stt: build request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/transcriptions", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpc.Do(req)
	if err != nil {
		return "", fmt.Errorf("stt: request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxTranscriptionResponseBody))
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("stt: http %d: %w: %s", resp.StatusCode, ErrProviderAuth, string(respBody))
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("stt: http %d: %s", resp.StatusCode, string(respBody))
	}

	var out transcriptionResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("stt: unparseable response: %w (%s)", err, string(respBody))
	}
	if out.Error != nil {
		return "", fmt.Errorf("stt: %s", out.Error.Message)
	}
	return strings.TrimSpace(out.Text), nil
}

// extensionForMime gives a multipart upload a filename extension a
// transcription endpoint can recognize when the source channel didn't
// supply one — common for a WhatsApp/Telegram voice note, which is often
// named generically or not at all.
func extensionForMime(mimetype string) string {
	switch strings.ToLower(strings.SplitN(mimetype, ";", 2)[0]) {
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/mp4", "audio/m4a", "audio/x-m4a":
		return ".m4a"
	case "audio/wav", "audio/x-wav", "audio/wave":
		return ".wav"
	case "audio/webm":
		return ".webm"
	case "audio/amr":
		return ".amr"
	case "audio/ogg", "audio/opus":
		return ".ogg"
	default:
		return ".ogg" // the common case: Telegram/WhatsApp voice notes are Opus-in-Ogg
	}
}
