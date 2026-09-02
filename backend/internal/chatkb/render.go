package chatkb

import (
	"fmt"
	"strings"
)

// Rendering the retrieved KB into the text the model reads.
//
// Two structural rules, both about provenance:
//
//   - The two states live in separate, explicitly banner-labelled sections.
//     There is no interleaving and no "effective value" line anywhere — a
//     model cannot mix what it was never shown mixed.
//   - Everything that differs is ALSO listed once more, up front, as an
//     explicit pending-changes block. Spotting a difference between two long
//     lists is exactly the kind of work a language model does unreliably, so
//     it is done in Go and handed over as a finding.

// RenderOptions bounds how much KB text one request may carry.
type RenderOptions struct {
	// MaxChars caps EACH state's section independently. 0 means no cap.
	// A section that hits the cap is cut at a record boundary and marked, so
	// the model is told its view is partial rather than silently believing a
	// truncated KB is the whole one.
	MaxChars int
}

// DefaultMaxChars is the per-section budget a request uses when none is
// configured — generous enough that a single business's whole KB fits
// comfortably, low enough that a runaway import cannot turn one chat message
// into a six-figure-token request.
const DefaultMaxChars = 60000

// Render produces the KB context block for a prompt: the pending-changes
// summary, then REAL, then DRAFT. Returns "" only when both states are
// completely empty, which the caller reports to the model as "this
// organization has no knowledge base yet" rather than passing an empty
// block off as data.
func Render(result Result, opts RenderOptions) string {
	maxChars := opts.MaxChars
	if maxChars == 0 {
		maxChars = DefaultMaxChars
	}
	if len(result.Real.Records) == 0 && len(result.Draft.Records) == 0 {
		return ""
	}

	var b strings.Builder
	renderPending(&b, result.Differences())
	b.WriteString(renderSection("=== REAL (LIVE) KNOWLEDGE BASE ===", SourceReal, result.Real.Records, maxChars))
	b.WriteString("\n")
	b.WriteString(renderSection("=== DRAFT (PENDING CHANGES APPLIED) KNOWLEDGE BASE ===", SourceDraft, result.Draft.Records, maxChars))
	return b.String()
}

func renderPending(b *strings.Builder, differences []Difference) {
	b.WriteString("=== PENDING DRAFT CHANGES (REAL vs DRAFT) ===\n")
	if len(differences) == 0 {
		b.WriteString("None. The draft knowledge base is identical to the real one.\n\n")
		return
	}
	for _, d := range differences {
		fmt.Fprintf(b, "- [%s] %s %q (%s)\n", strings.ToUpper(string(d.Change)), d.Kind, d.Title, d.Key)
		for _, f := range d.Fields {
			fmt.Fprintf(b, "    %s: REAL_KB=%s | DRAFT_KB=%s\n", f.Label, quoteOrNone(f.Real), quoteOrNone(f.Draft))
		}
	}
	b.WriteString("\n")
}

// renderSection writes one state's records under its banner. The banner
// repeats the source tag on every record so a line copied out of the middle
// of a long section still carries its own provenance.
func renderSection(banner string, src Source, records []Record, maxChars int) string {
	var b strings.Builder
	b.WriteString(banner)
	b.WriteString("\n")
	if len(records) == 0 {
		b.WriteString("(empty)\n")
		return b.String()
	}
	written := 0
	for i, rec := range records {
		block := renderRecord(rec, src)
		if maxChars > 0 && written+len(block) > maxChars && i > 0 {
			fmt.Fprintf(&b, "... (%d more records omitted — this view of the knowledge base is incomplete)\n", len(records)-i)
			break
		}
		written += len(block)
		b.WriteString(block)
	}
	return b.String()
}

func renderRecord(rec Record, src Source) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] %s | %s | key=%s\n", src, rec.Kind, rec.Title, rec.Key)
	for _, f := range rec.Fields {
		fmt.Fprintf(&b, "  %s: %s\n", f.Label, singleLine(f.Value))
	}
	return b.String()
}

// singleLine keeps one field on one line. A topic body is markdown with its
// own newlines, which would otherwise break the "one fact per line" shape
// the rest of the block relies on.
func singleLine(v string) string {
	if !strings.ContainsAny(v, "\r\n") {
		return v
	}
	return strings.Join(strings.FieldsFunc(v, func(r rune) bool { return r == '\n' || r == '\r' }), " ⏎ ")
}

func quoteOrNone(v string) string {
	if v == "" {
		return "(not set)"
	}
	return `"` + singleLine(v) + `"`
}
