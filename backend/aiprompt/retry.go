package aiprompt

import (
	"fmt"
	"strings"
)

// RetryReason names why a raw model output is a retry candidate, or that it is
// not one at all (""). Mirrors the eval harness's retryReason vocabulary
// (evals/harness/retry.go:29-46), carried into production so the one-retry
// policy stays identical to what schema_kb_v1 evaluated.
type RetryReason string

const (
	// RetryReasonNone: the response needs no retry.
	RetryReasonNone RetryReason = ""
	// RetryReasonContractShape: unparseable JSON, or JSON failing the strict
	// contract shape (missing/unknown property, wrong type, bad language, or
	// missing validation context). The only lever a retry has here is an
	// unchanged re-roll of the same prompt — see RetryFeedback.
	RetryReasonContractShape RetryReason = "contract_shape"
	// RetryReasonMediaNotFound: the model named a media_files_to_send token
	// outside the catalog, or one that no longer resolves through the current KB.
	RetryReasonMediaNotFound RetryReason = "media_not_found"
)

// ClassifyRetry decides whether raw model output is a retry candidate and why,
// mirroring the eval harness's retryCandidateIndexesSchemaKB
// (evals/harness/retry.go:90-105): contract_shape ALWAYS wins over
// media_not_found, so a malformed response is retried on its shape problem even
// when it also names a bad media token.
func ClassifyRetry(raw string, kb *KB, cat *Catalog) RetryReason {
	ext, ok := ExtractFinalOutput(raw)
	if !ok {
		return RetryReasonContractShape
	}
	resp, issues := ValidateResponse(ext.Final, kb, cat)
	if resp == nil {
		return RetryReasonContractShape
	}
	for _, issue := range issues {
		switch issue.Code {
		case "missing_property", "unknown_property", "validation_context", "bad_language":
			return RetryReasonContractShape
		}
	}
	for _, issue := range issues {
		if issue.Code == "unknown_media_token" {
			return RetryReasonMediaNotFound
		}
	}
	if len(resp.MediaFilesToSend) > 0 {
		if _, err := ResolveSend(resp.MediaFilesToSend, kb, cat); err != nil {
			return RetryReasonMediaNotFound
		}
	}
	return RetryReasonNone
}

// ClassifyRetryV7 is ClassifyRetry for the v7+ contract: identical
// classification, using ValidateResponseV7 instead of ValidateResponse so a
// legitimate "kb_gap" property is never misread as unknown_property. This
// distinction matters in practice, not just in principle: ValidateResponse
// treats any "kb_gap" key as unknown_property (contract_shape, v6's correct
// behavior), which would otherwise force a wasted retry on every v7 response
// that includes a valid diagnostic — i.e. on most escalations. Engine.Generate
// must call this (never ClassifyRetry) whenever it rendered a v7 frame.
func ClassifyRetryV7(raw string, kb *KB, cat *Catalog) RetryReason {
	ext, ok := ExtractFinalOutput(raw)
	if !ok {
		return RetryReasonContractShape
	}
	resp, issues := ValidateResponseV7(ext.Final, kb, cat)
	if resp == nil {
		return RetryReasonContractShape
	}
	for _, issue := range issues {
		switch issue.Code {
		case "missing_property", "unknown_property", "validation_context", "bad_language":
			return RetryReasonContractShape
		}
	}
	for _, issue := range issues {
		if issue.Code == "unknown_media_token" {
			return RetryReasonMediaNotFound
		}
	}
	if len(resp.MediaFilesToSend) > 0 {
		if _, err := ResolveSend(resp.MediaFilesToSend, kb, cat); err != nil {
			return RetryReasonMediaNotFound
		}
	}
	return RetryReasonNone
}

// RetryFeedbackV7 is RetryFeedback for the v7+ contract — see
// ClassifyRetryV7 for why this must be the sibling Engine.Generate calls
// instead of RetryFeedback whenever it rendered a v7 frame.
func RetryFeedbackV7(raw string, kb *KB, cat *Catalog) string {
	if ClassifyRetryV7(raw, kb, cat) != RetryReasonMediaNotFound {
		return ""
	}
	ext, ok := ExtractFinalOutput(raw)
	if !ok {
		return ""
	}
	resp, _ := ValidateResponseV7(ext.Final, kb, cat)
	if resp == nil {
		return ""
	}
	var badTokens []string
	for _, tok := range resp.MediaFilesToSend {
		if cat.MediaByToken(tok) == nil {
			badTokens = append(badTokens, tok)
			continue
		}
		if _, err := ResolveSend([]string{tok}, kb, cat); err != nil {
			badTokens = append(badTokens, tok)
		}
	}
	if len(badTokens) == 0 {
		return ""
	}
	return fmt.Sprintf(
		"\n\nВАЖНО — медиа не найдено, исправь и попробуй снова: в предыдущем ответе поле %q "+
			"содержало значения, для которых сейчас нет медиафайла: %s. Скопируй нужный токен "+
			"ТОЧНО из блока товара или темы выше — не изменяй, не сокращай и не придумывай "+
			"новый; если подходящего медиа действительно нет, верни пустой список.",
		"media_files_to_send", strings.Join(badTokens, ", "),
	)
}

// RetryFeedback returns the Russian corrective feedback text to append to a
// retry's prompt, verbatim-matching the eval harness's
// correctiveFeedbackTextSchemaKB (evals/harness/retry.go:148-178): it names
// every media_files_to_send token that is either outside the catalog or no
// longer resolves through the current KB, and instructs the model to copy an
// existing token exactly. Empty for a contract_shape retry — there is no
// specific problem to point at, so the same prompt is sent unchanged and left
// to self-correct on the second attempt.
func RetryFeedback(raw string, kb *KB, cat *Catalog) string {
	if ClassifyRetry(raw, kb, cat) != RetryReasonMediaNotFound {
		return ""
	}
	ext, ok := ExtractFinalOutput(raw)
	if !ok {
		return ""
	}
	resp, _ := ValidateResponse(ext.Final, kb, cat)
	if resp == nil {
		return ""
	}
	var badTokens []string
	for _, tok := range resp.MediaFilesToSend {
		if cat.MediaByToken(tok) == nil {
			badTokens = append(badTokens, tok)
			continue
		}
		if _, err := ResolveSend([]string{tok}, kb, cat); err != nil {
			badTokens = append(badTokens, tok)
		}
	}
	if len(badTokens) == 0 {
		return ""
	}
	return fmt.Sprintf(
		"\n\nВАЖНО — медиа не найдено, исправь и попробуй снова: в предыдущем ответе поле %q "+
			"содержало значения, для которых сейчас нет медиафайла: %s. Скопируй нужный токен "+
			"ТОЧНО из блока товара или темы выше — не изменяй, не сокращай и не придумывай "+
			"новый; если подходящего медиа действительно нет, верни пустой список.",
		"media_files_to_send", strings.Join(badTokens, ", "),
	)
}
