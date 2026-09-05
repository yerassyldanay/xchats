package settings

import (
	"os"
	"strconv"
)

// defaultsFromEnv seeds LLMSettings from the exact env var names
// internal/config's LLMDefaultProvider/LLMDefaultModel/LLMDraft* fields
// already used (LLM_DEFAULT_PROVIDER, LLM_DEFAULT_MODEL,
// LLM_DRAFT_MAX_TOKENS, LLM_DRAFT_TEMPERATURE, LLM_DRAFT_TIMEOUT_SECONDS,
// LLM_DRAFT_RETRY) — so a deployment that already configured its default
// provider/model via those env vars keeps using them as Settings' own
// initial values the first time Store.Load creates settings.json, instead
// of silently reverting to this package's own hardcoded defaults. Every
// fallback below matches config.go's defaults() exactly.
//
// internal/settings deliberately does not import internal/config for this —
// reading the four env vars directly keeps this package dependency-free,
// the same reasoning internal/credentials' EnvStore already applies.
func defaultsFromEnv() LLMSettings {
	return LLMSettings{
		DefaultProvider: envOr("LLM_DEFAULT_PROVIDER", "openrouter"),
		DefaultModel:    envOr("LLM_DEFAULT_MODEL", "google/gemini-2.5-flash"),
		VisionModel:     envOr("LLM_VISION_MODEL", ""),
		// STT has no prior env-var-only existence (unlike the four fields
		// above, which predate the Settings UI) — these three env vars exist
		// only so a headless/scripted deployment can preconfigure it the
		// same way it already can for the chat model, not to preserve any
		// legacy behavior. Unset means "not configured" (see
		// LLMSettings.STTProvider's own doc comment), so the fallback is "",
		// not a guessed provider.
		STTProvider:    envOr("STT_PROVIDER", ""),
		STTModel:       envOr("STT_MODEL", ""),
		STTLanguage:    envOr("STT_LANGUAGE", "auto"),
		STTVocabulary:  envOr("STT_VOCABULARY", ""),
		MaxTokens:      envIntOr("LLM_DRAFT_MAX_TOKENS", 500),
		Temperature:    envFloatOr("LLM_DRAFT_TEMPERATURE", 0.3),
		TimeoutSeconds: envIntOr("LLM_DRAFT_TIMEOUT_SECONDS", 60),
		Retry:          envBoolOr("LLM_DRAFT_RETRY", true),
	}
}

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func envIntOr(name string, def int) int {
	v, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return def
	}
	return v
}

func envFloatOr(name string, def float64) float64 {
	v, err := strconv.ParseFloat(os.Getenv(name), 64)
	if err != nil {
		return def
	}
	return v
}

func envBoolOr(name string, def bool) bool {
	v, err := strconv.ParseBool(os.Getenv(name))
	if err != nil {
		return def
	}
	return v
}
