package llmprovider

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "xchats/llmprovider"

func tracer() trace.Tracer { return otel.GetTracerProvider().Tracer(tracerName) }

type contextKey uint8

const (
	sessionIDKey contextKey = iota
	userIDKey
)

// WithSessionID attaches a session identifier (e.g. conversation ID) to ctx —
// read by startGeneration to set langfuse.session.id on the generation span.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// WithUserID attaches a user identifier (e.g. contact ref) to ctx — read by
// startGeneration to set langfuse.user.id on the generation span.
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

// Trace wraps a root span representing one logical LLM-backed operation (one
// ResponseService.Respond call). Reused pattern from
// internal/brain/llm/tracing.go — this package duplicates rather than imports
// it, since that package's helpers are unexported and internal/llmprovider
// must not depend on internal/brain.
type Trace struct{ span trace.Span }

// TraceOptions are the trace-level attributes set on the root span.
type TraceOptions struct {
	SessionID string   // langfuse.session.id — groups a conversation
	UserID    string   // langfuse.user.id — the end user/contact
	Tags      []string // langfuse.trace.tags — feature labels for filtering
	Input     string   // langfuse.trace.input — the meaningful input (e.g. the customer message)
}

// StartTrace opens a root span named name. Subsequent Complete calls made with
// the returned ctx nest under it as generations.
func StartTrace(ctx context.Context, name string, o TraceOptions) (context.Context, *Trace) {
	ctx, span := tracer().Start(ctx, name)
	if o.SessionID != "" {
		span.SetAttributes(attribute.String("langfuse.session.id", o.SessionID))
	}
	if o.UserID != "" {
		span.SetAttributes(attribute.String("langfuse.user.id", o.UserID))
	}
	if len(o.Tags) > 0 {
		span.SetAttributes(attribute.StringSlice("langfuse.trace.tags", o.Tags))
	}
	if o.Input != "" {
		span.SetAttributes(attribute.String("langfuse.trace.input", o.Input))
	}
	return ctx, &Trace{span: span}
}

// End finalizes the trace: on err it records the failure; otherwise it stores
// the final output (langfuse.trace.output).
func (t *Trace) End(output string, err error) {
	if err != nil {
		t.span.RecordError(err)
		t.span.SetStatus(codes.Error, err.Error())
	} else {
		if output != "" {
			t.span.SetAttributes(attribute.String("langfuse.trace.output", output))
		}
		t.span.SetStatus(codes.Ok, "")
	}
	t.span.End()
}

// genParams describes one model invocation for a generation span.
type genParams struct {
	name        string
	provider    string
	model       string
	temperature float64
	maxTokens   int
	input       string
}

type generation struct{ span trace.Span }

// startGeneration opens a generation span for a single LLM call.
func startGeneration(ctx context.Context, p genParams) (context.Context, *generation) {
	name := p.name
	if name == "" {
		name = "llm.generation"
	}
	ctx, span := tracer().Start(ctx, name, trace.WithSpanKind(trace.SpanKindClient))

	span.SetAttributes(
		attribute.String("langfuse.observation.type", "generation"),
		attribute.String("gen_ai.system", p.provider),
		attribute.String("gen_ai.request.model", p.model),
	)
	if p.temperature > 0 {
		span.SetAttributes(attribute.Float64("gen_ai.request.temperature", p.temperature))
	}
	if p.maxTokens > 0 {
		span.SetAttributes(attribute.Int("gen_ai.request.max_tokens", p.maxTokens))
	}
	if sid := sessionIDFromContext(ctx); sid != "" {
		span.SetAttributes(attribute.String("langfuse.session.id", sid))
	}
	if uid := userIDFromContext(ctx); uid != "" {
		span.SetAttributes(attribute.String("langfuse.user.id", uid))
	}
	if p.input != "" {
		span.SetAttributes(attribute.String("langfuse.observation.input", p.input))
	}
	return ctx, &generation{span: span}
}

func (g *generation) setUsage(inputTokens, outputTokens int) {
	if inputTokens > 0 || outputTokens > 0 {
		g.span.SetAttributes(
			attribute.Int("gen_ai.usage.input_tokens", inputTokens),
			attribute.Int("gen_ai.usage.output_tokens", outputTokens),
		)
	}
}

func (g *generation) end(output string, err error) {
	if err != nil {
		g.span.RecordError(err)
		g.span.SetStatus(codes.Error, err.Error())
	} else {
		if output != "" {
			g.span.SetAttributes(attribute.String("langfuse.observation.output", output))
		}
		g.span.SetStatus(codes.Ok, "")
	}
	g.span.End()
}
