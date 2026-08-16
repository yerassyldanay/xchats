package extractor

import (
	"net/http"
	"strings"
	"time"
)

// NewLlamaParseForTest builds the LlamaParse provider with a caller-chosen
// poll interval, so a test double's job can "complete" after one or two
// polls instead of the real default's 2s cadence.
func NewLlamaParseForTest(apiKey, baseURL string, client *http.Client, pollInterval time.Duration) Provider {
	if baseURL == "" {
		baseURL = "https://api.cloud.llamaindex.ai"
	}
	return &llamaparseProvider{
		apiKey: apiKey, baseURL: strings.TrimRight(baseURL, "/"), client: client,
		pollInterval: pollInterval,
	}
}
