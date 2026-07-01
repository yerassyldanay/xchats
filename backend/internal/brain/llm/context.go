package llm

import "context"

type contextKey uint8

const (
	sessionIDKey contextKey = iota
	userIDKey
)

// WithSessionID attaches a session identifier (e.g. chat ID) to ctx. The tracing
// helper reads it to set langfuse.session.id on the generation span.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// WithUserID attaches a user identifier (e.g. customer phone) to ctx. The tracing
// helper reads it to set langfuse.user.id on the generation span.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func sessionIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(sessionIDKey).(string); ok {
		return v
	}
	return ""
}

func userIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	return ""
}
