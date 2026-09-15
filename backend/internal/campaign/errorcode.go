package campaign

import (
	"errors"
	"regexp"
	"strings"

	"github.com/yerassyldanay/xchats/backend/messaging"
)

// ErrorCode is a structured, closed-ish reason a send attempt did not reach
// RecipientSent — see migrations/sqlite/0020_campaign_diagnostics.up.sql's
// own doc comment for why campaign_send_attempts.error_code has no SQL
// CHECK against this vocabulary (validated in Go instead, so it can grow
// without a migration, same as aiprompt's KB-gap reason codes).
type ErrorCode string

const (
	// ErrorCodeWhatsAppDisconnected: the sending account had no live
	// provider connection at send time.
	ErrorCodeWhatsAppDisconnected ErrorCode = "whatsapp_disconnected"
	// ErrorCodeSenderUnavailable: no channel adapter is registered/wired for
	// this campaign's channel at all — a configuration problem, not a
	// connection's own state.
	ErrorCodeSenderUnavailable ErrorCode = "sender_unavailable"
	// ErrorCodeInvalidRecipient: the destination is malformed, or the
	// provider has permanently rejected it, or (warm-only channels) no
	// existing conversation could be found for it.
	ErrorCodeInvalidRecipient ErrorCode = "invalid_recipient"
	// ErrorCodeServiceWindow: the provider's free-form messaging window has
	// closed for this recipient.
	ErrorCodeServiceWindow ErrorCode = "service_window_closed"
	// ErrorCodeProviderRejected: the provider returned an explicit rejection
	// for this specific request that isn't covered by a more specific code
	// above.
	ErrorCodeProviderRejected ErrorCode = "provider_rejected"
	// ErrorCodeNetwork: a network-level failure with no indication the
	// provider ever saw the request.
	ErrorCodeNetwork ErrorCode = "network_error"
	// ErrorCodeTimeout: a send request was made but no response arrived in
	// time — AMBIGUOUS: the provider may have processed it. See
	// Runner.finalize's own doc comment for how this differs from every
	// other transient code.
	ErrorCodeTimeout ErrorCode = "timeout"
	// ErrorCodeMessageCreationFailed: building or persisting the outbound
	// message itself failed, before any provider round trip.
	ErrorCodeMessageCreationFailed ErrorCode = "message_creation_failed"
	// ErrorCodeInternalError: a local store/database failure unrelated to
	// the provider (e.g. the chat lookup itself failed).
	ErrorCodeInternalError ErrorCode = "internal_error"
	// ErrorCodeUnknown: the failure could not be attributed to any of the
	// above from the evidence available. Never guessed from free text — see
	// classifySendErr's own doc comment.
	ErrorCodeUnknown ErrorCode = "unknown"
)

// classification is classifySendError's result: the structured code, a
// human-sanitized detail string safe to persist/return (the same one stored
// in campaign_send_attempts.error_detail and campaign_recipients.
// failure_reason), and whether the SAME error, on its own, is safe to retry
// automatically. ambiguous additionally marks the one case (a timeout) where
// "retryable" is deliberately false even though the underlying cause is
// transient — see Runner.finalize.
type classification struct {
	Code      ErrorCode
	Detail    string
	Retryable bool
	Ambiguous bool
}

// classifySendError maps sendErr — the error internal/outbound.Deliver (or
// this package's own resolveChat/InsertCampaignOutbound calls) returned —
// into a classification, using errors.Is against the channel-neutral
// messaging.Err* sentinels an adapter wraps its own failures in. An error
// this function does not recognize (whatsmeow's own internals leaked
// unwrapped, a bare I/O error, ...) becomes ErrorCodeUnknown — this
// function never inspects err.Error()'s text to guess a code.
func classifySendError(sendErr error) classification {
	cls := classifySendErrorRaw(sendErr)
	cls.Detail = sanitizeDetail(cls.Detail)
	return cls
}

func classifySendErrorRaw(sendErr error) classification {
	switch {
	case sendErr == nil:
		return classification{Code: "", Retryable: false}
	case errors.Is(sendErr, messaging.ErrOutsideServiceWindow):
		return classification{Code: ErrorCodeServiceWindow, Detail: sendErr.Error(), Retryable: false}
	case errors.Is(sendErr, messaging.ErrRecipientUnreachable), errors.Is(sendErr, messaging.ErrInvalidRecipient), errors.Is(sendErr, errNoExistingChat):
		return classification{Code: ErrorCodeInvalidRecipient, Detail: sendErr.Error(), Retryable: false}
	case errors.Is(sendErr, messaging.ErrAccountDisconnected):
		return classification{Code: ErrorCodeWhatsAppDisconnected, Detail: sendErr.Error(), Retryable: true}
	case errors.Is(sendErr, messaging.ErrChannelUnavailable):
		return classification{Code: ErrorCodeSenderUnavailable, Detail: sendErr.Error(), Retryable: true}
	case errors.Is(sendErr, messaging.ErrMessageBuildFailed):
		return classification{Code: ErrorCodeMessageCreationFailed, Detail: sendErr.Error(), Retryable: true}
	case errors.Is(sendErr, messaging.ErrSendTimeout):
		// Retryable is false here on purpose: the normal transient ladder
		// assumes a failed attempt never reached the provider, which a
		// timeout cannot promise. See Runner.finalize's own doc comment for
		// what happens to the recipient instead (stopped, not retried).
		return classification{Code: ErrorCodeTimeout, Detail: sendErr.Error(), Retryable: false, Ambiguous: true}
	case errors.Is(sendErr, messaging.ErrNetwork):
		return classification{Code: ErrorCodeNetwork, Detail: sendErr.Error(), Retryable: true}
	default:
		return classification{Code: ErrorCodeUnknown, Detail: sendErr.Error(), Retryable: true}
	}
}

// localFailure builds a classification for a failure this package knows the
// cause of directly (which store call failed), rather than one reported by
// a ChannelSender — the call site's own knowledge of what it just tried is
// stronger evidence than pattern-matching an error string, so no sentinel
// lookup happens here. Always Retryable: a local database hiccup is not
// inherently permanent, so it steps through the same bounded ladder any
// other transient failure does.
func localFailure(code ErrorCode, err error) classification {
	return classification{Code: code, Detail: sanitizeDetail(err.Error()), Retryable: true}
}

// secretLikePattern matches common secret-bearing key=value/header shapes
// (bearer tokens, api keys, passwords, authorization headers) so a
// classification's Detail never persists one even if an unexpected error
// string happened to embed it — belt-and-suspenders alongside every error
// source here already being provider/library errors that do not carry
// secrets by design.
var secretLikePattern = regexp.MustCompile(`(?i)(bearer\s+[a-z0-9._-]+|(api[_-]?key|token|password|secret|authorization)\s*[=:]\s*\S+)`)

const maxErrorDetailLen = 300

// sanitizeDetail caps length and redacts anything secret-shaped before a
// free-text error is persisted to campaign_send_attempts.error_detail or
// campaign_recipients.failure_reason, or returned through the read-only MCP
// tools — see this feature's own data-minimization requirement.
func sanitizeDetail(s string) string {
	s = secretLikePattern.ReplaceAllString(s, "[redacted]")
	if len(s) > maxErrorDetailLen {
		s = s[:maxErrorDetailLen] + "…"
	}
	return strings.TrimSpace(s)
}
