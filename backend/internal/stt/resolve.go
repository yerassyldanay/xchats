package stt

import (
	"context"
	"strings"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
)

// ResolveParams reads the audio-transcription engine's CURRENT
// configuration from settings + credentials. Both cmd/xchats' Worker.STT
// resolver (the automatic run right after a voice note downloads) and
// internal/httpapi's manual "re-transcribe" endpoint call this exact
// function, so the two paths can never resolve a different
// provider/model/key/base-URL than each other — mirrors
// cmd/xchats/main.go's populateLLMRegistry/resolveLLMParams pair, kept
// dependency-light the same way (creds first, else nothing; settingsStore's
// own ProviderSettings.BaseURL override, else DefaultBaseURL).
//
// An empty STTProvider/STTModel, or a provider with no resolvable
// credential, yields a zero Params — callers treat a nil Transcriber as
// "not configured," never as an error. timeoutSeconds <= 0 falls back to
// 60s; callers resolve it from their own config (e.g.
// config.Config.LLMDraftTimeoutSeconds) rather than this package importing
// internal/config for one integer.
func ResolveParams(ctx context.Context, creds *credentials.Chain, settingsStore *settings.Store, timeoutSeconds int) Params {
	st, err := settingsStore.Load()
	if err != nil || st.LLM.STTProvider == "" || st.LLM.STTModel == "" {
		return Params{}
	}
	var key string
	if creds != nil {
		if v, err := creds.Get(ctx, credentials.Key(st.LLM.STTProvider+".api_key")); err == nil {
			key = v
		}
	}
	if strings.TrimSpace(key) == "" {
		return Params{}
	}
	baseURL := DefaultBaseURL(st.LLM.STTProvider)
	if ps, ok := st.Providers[st.LLM.STTProvider]; ok && ps.BaseURL != "" {
		baseURL = ps.BaseURL
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	language := st.LLM.STTLanguage
	if language == "auto" {
		language = "" // TranscribeOptions' own "" == auto-detect
	}
	return Params{
		Transcriber: NewOpenAITranscriber(baseURL, key, st.LLM.STTModel, time.Duration(timeoutSeconds)*time.Second),
		Language:    language,
		Vocabulary:  st.LLM.STTVocabulary,
	}
}
