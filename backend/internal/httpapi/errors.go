package httpapi

// Unified machine error codes (plan/7-api-contracts.md). "OK" means success.
const (
	ErrOK                     = "OK"
	ErrValidation             = "VALIDATION_ERROR"
	ErrUnauthorized           = "UNAUTHORIZED"
	ErrForbidden              = "FORBIDDEN"
	ErrNotFound               = "NOT_FOUND"
	ErrConflict               = "CONFLICT"
	ErrRateLimited            = "RATE_LIMITED"
	ErrWebhookUnauthorized    = "WEBHOOK_UNAUTHORIZED"
	ErrAccountNotAssigned     = "ACCOUNT_NOT_ASSIGNED"
	ErrAccountNotConnected    = "ACCOUNT_NOT_CONNECTED"
	ErrSendFailed             = "SEND_FAILED"
	ErrMediaUnavailable       = "MEDIA_UNAVAILABLE"
	ErrWhatsApp               = "WHATSAPP_ERROR"
	ErrTelegram               = "TELEGRAM_ERROR"
	ErrTelegramToken          = "TELEGRAM_TOKEN_REJECTED"
	ErrAIUnavailable          = "AI_UNAVAILABLE"
	ErrDraftStale             = "DRAFT_STALE"
	ErrAutomationOff          = "AUTOMATION_OFF"
	ErrInternal               = "INTERNAL"
	ErrPasswordChangeRequired = "PASSWORD_CHANGE_REQUIRED"

	// Settings surface (internal/credentials, internal/settings, internal/tunnel).
	ErrCredentialInvalid    = "CREDENTIAL_INVALID"    // the provider's own API rejected it (bad key)
	ErrCredentialUnverified = "CREDENTIAL_UNVERIFIED" // could not be checked; caller must resubmit with force=true to save anyway
	ErrCredentialStore      = "CREDENTIAL_STORE"      // no credential store available, or a store read/write failed
	ErrTunnelUnavailable    = "TUNNEL_UNAVAILABLE"    // the tunnel feature is not configured, or Start/Stop itself failed

	// Meta channels surface (internal/meta, internal/whatsappcloud, internal/metaingest).
	ErrMetaError            = "META_ERROR"              // Meta's Graph API rejected a call — see the message for its own error text
	ErrMetaAppNotConfigured = "META_APP_NOT_CONFIGURED" // no "meta" App ID/Secret saved in Settings yet
)
