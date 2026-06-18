package llm

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "xchats/llm"

// tracer returns the package tracer (no-op when Langfuse is disabled).
func tracer() trace.Tracer { return otel.GetTracerProvider().Tracer(tracerName) }

// Trace wraps a root span representing one logical LLM-backed operation (e.g. a
// reply draft). It carries the Langfuse trace-level fields — name (the span name),
// session, user, tags, and trace input/output — so the nested generation only has
// to describe the model call. When no TracerProvider is installed it is a no-op.
type Trace struct{ span trace.Span }

// TraceOptions are the trace-level attributes set on the root span.
type TraceOptions struct {
	SessionID string   // langfuse.session.id — groups a conversation
	UserID    string   // langfuse.user.id — the end user/contact
	Tags      []string // langfuse.trace.tags — feature labels for filtering
	Input     string   // langfuse.trace.input — the meaningful input (e.g. the customer message), NOT all args
}

// StartTrace opens a root span named name (→ Langfuse trace name). Subsequent
// LLM calls made with the returned ctx nest under it as generations.
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

// End finalizes the trace: on err it records the failure; otherwise it stores the
// final output (langfuse.trace.output).
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
	name        string   // descriptive span/observation name, e.g. "llm.draft"
	provider    string   // gen_ai.system
	model       string   // gen_ai.request.model
	temperature float64  // gen_ai.request.temperature (omitted when 0)
	maxTokens   int      // gen_ai.request.max_tokens (omitted when 0)
	tags        []string // langfuse.trace.tags — only meaningful when this generation IS the root span
	input       string   // langfuse.observation.input (PII redacted by the caller)
}

// generation wraps an OTel span representing one Langfuse generation observation.
type generation struct{ span trace.Span }

// startGeneration opens a generation span for a single LLM call. Session/user IDs
// are read from ctx (WithSessionID / WithUserID) so a standalone generation (no
// parent trace) still maps to the right Langfuse trace. When the call nests under
// a StartTrace span those trace-level fields come from the root instead.
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
	if len(p.tags) > 0 {
		span.SetAttributes(attribute.StringSlice("langfuse.trace.tags", p.tags))
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

// setUsage records token counts when the provider returned them.
func (g *generation) setUsage(inputTokens, outputTokens int) {
	if inputTokens > 0 || outputTokens > 0 {
		g.span.SetAttributes(
			attribute.Int("gen_ai.usage.input_tokens", inputTokens),
			attribute.Int("gen_ai.usage.output_tokens", outputTokens),
		)
	}
}

// end finalizes the span: on err it records the failure; otherwise it stores the
// completion text as the generation output.
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
