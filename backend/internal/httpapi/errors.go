package httpapi

// Unified machine error codes (plan/7-api-contracts.md). "OK" means success.
const (
	ErrOK                 = "OK"
	ErrValidation         = "VALIDATION_ERROR"
	ErrUnauthorized       = "UNAUTHORIZED"
	ErrForbidden          = "FORBIDDEN"
	ErrNotFound           = "NOT_FOUND"
	ErrConflict           = "CONFLICT"
	ErrRateLimited        = "RATE_LIMITED"
	ErrWebhookUnauthorized = "WEBHOOK_UNAUTHORIZED"
	ErrAccountNotAssigned  = "ACCOUNT_NOT_ASSIGNED"
	ErrAccountNotConnected = "ACCOUNT_NOT_CONNECTED"
	ErrInstanceNotFound    = "INSTANCE_NOT_FOUND"
	ErrSendFailed          = "SEND_FAILED"
	ErrMediaUnavailable    = "MEDIA_UNAVAILABLE"
	ErrEvolution           = "EVOLUTION_ERROR"
	ErrAIUnavailable       = "AI_UNAVAILABLE"
	ErrDraftStale          = "DRAFT_STALE"
	ErrInternal            = "INTERNAL"
)
