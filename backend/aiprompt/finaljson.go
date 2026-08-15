package aiprompt

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ExtractedFinal is the result of separating a model's FINAL customer-response JSON
// from any reasoning/preamble text the provider returned combined with it. The
// caller must keep the complete provider response byte-for-byte as its own raw
// record — nothing here ever mutates or reconstructs model output; extraction only
// SELECTS a span that already exists verbatim in the input.
type ExtractedFinal struct {
	// Final is the exact text of the selected final JSON object, unmodified.
	Final string
	// NonFinal is everything around the selected object (reasoning narration,
	// partial diagnostic objects, prose) — kept for debugging, never for scoring
	// and never for the customer. Empty when the whole input was the final answer.
	NonFinal string
	// Method records how the final answer was found — one of the Extract*
	// constants below.
	Method string
}

// Extraction methods, most direct first.
const (
	// ExtractDirect: the entire output (trimmed) is one JSON object.
	ExtractDirect = "direct"
	// ExtractOuterFence: one outer markdown code fence around one JSON object.
	ExtractOuterFence = "outer_fence"
	// ExtractFencedBlock: a fenced JSON block inside combined reasoning+answer text.
	ExtractFencedBlock = "fenced_block"
	// ExtractBalancedObject: a complete balanced JSON object located inside
	// combined reasoning+answer text.
	ExtractBalancedObject = "balanced_object"
)

// operationalProperties are the fields that make an object a customer-response
// answer rather than a reasoning fragment or a diagnostic-only blob like
// {"confidence": 1}. Derived from propertyDescriptions so the two can never drift.
func operationalProperties() []string {
	var out []string
	for _, p := range propertyDescriptions {
		if !p.Diagnostic {
			out = append(out, p.Name)
		}
	}
	return out
}

// ExtractFinalOutput locates the model's final customer-response JSON inside raw
// provider output that may also carry reasoning text. It is a defensive fallback:
// prefer provider configuration that returns clean final content, but providers
// still return combined output in practice.
//
// Order of acceptance:
//  1. The whole (trimmed) output is one JSON object -> direct.
//  2. The whole output wrapped in ONE outer markdown fence -> outer_fence.
//     Both accept the object as the final answer if it even ATTEMPTS the
//     operational contract (has at least one operational key) — field-level
//     type/shape problems are then reported by contract validation, not hidden
//     as a parse failure.
//  3. Otherwise the output is combined reasoning+answer: every fenced JSON
//     block and every complete balanced JSON object is a candidate, processed
//     LAST to FIRST, and the final answer is the last complete object whose
//     operational fields are all present with correct types. Partial
//     diagnostic-only objects (e.g. {"confidence": 1}) are ignored.
//
// If no complete operational object exists — including when reasoning consumed
// the completion budget and the final JSON is truncated — extraction fails.
// A truncated or incomplete object is NEVER repaired or reconstructed.
//
// This is a thin instantiation of extractJSONObject over the customer-response
// contract's own two-tier acceptance rule (loose whole-input check, strict
// embedded-candidate check) — see ExtractJSONObject for the contract-agnostic,
// single-predicate form other callers (e.g. internal/kbimport's synthesis
// parser) use instead.
func ExtractFinalOutput(raw string) (ExtractedFinal, bool) {
	return extractJSONObject(raw, hasOperationalProperty, operationalPropertiesTyped)
}

// ExtractJSONObject is the contract-agnostic core of ExtractFinalOutput: it
// runs the exact same direct / outer-fence / fenced-block / balanced-object
// scan, but accept is the ONE predicate applied uniformly at every stage
// (there is no loose-whole-input-vs-strict-embedded-candidate distinction —
// that asymmetry is specific to the customer-response contract's own
// diagnostic-field carve-out, preserved in ExtractFinalOutput above, not a
// property of the scan itself). A caller with a single "is this my shape"
// test — e.g. "does this object have a top-level calls field" — reaches for
// this directly instead of duplicating the scan.
func ExtractJSONObject(raw string, accept func(map[string]json.RawMessage) bool) (ExtractedFinal, bool) {
	return extractJSONObject(raw, accept, accept)
}

// extractJSONObject is the shared scan-and-select implementation: locate a
// JSON object in raw, preferring (in order) the whole trimmed input, the
// whole input stripped of one outer markdown fence, then — for combined
// reasoning+answer text — the last fenced or balanced embedded object that
// satisfies acceptCandidate, scanned last to first so the FINAL answer wins
// over earlier reasoning fragments. acceptWhole gates the first two stages;
// acceptCandidate gates the third.
func extractJSONObject(raw string, acceptWhole, acceptCandidate func(map[string]json.RawMessage) bool) (ExtractedFinal, bool) {
	trimmed := strings.TrimSpace(raw)
	if m, ok := decodeOneObject(trimmed); ok && acceptWhole(m) {
		return ExtractedFinal{Final: trimmed, Method: ExtractDirect}, true
	}
	stripped := strings.TrimSpace(stripMarkdownFences(trimmed))
	if stripped != trimmed {
		if m, ok := decodeOneObject(stripped); ok && acceptWhole(m) {
			return ExtractedFinal{Final: stripped, Method: ExtractOuterFence}, true
		}
	}

	candidates := scanJSONCandidates(raw)
	for i := len(candidates) - 1; i >= 0; i-- {
		c := candidates[i]
		m, ok := decodeOneObject(c.text)
		if !ok || !acceptCandidate(m) {
			continue
		}
		return ExtractedFinal{
			Final:    c.text,
			NonFinal: nonFinalAround(raw, c),
			Method:   c.method,
		}, true
	}
	return ExtractedFinal{}, false
}

// decodeOneObject decodes s as exactly one JSON object with nothing but
// whitespace after it. Returns false for non-objects, null, invalid or
// truncated JSON, and trailing content.
func decodeOneObject(s string) (map[string]json.RawMessage, bool) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &m); err != nil || m == nil {
		return nil, false
	}
	return m, true
}

func hasOperationalProperty(m map[string]json.RawMessage) bool {
	for _, name := range operationalProperties() {
		if _, ok := m[name]; ok {
			return true
		}
	}
	return false
}

// operationalPropertiesTyped reports whether ALL operational fields are present
// with the correct types — the bar an embedded candidate must clear to count as
// the final answer (diagnostic fields stay irrelevant here, as everywhere).
func operationalPropertiesTyped(m map[string]json.RawMessage) bool {
	for _, name := range operationalProperties() {
		rawProperty, ok := m[name]
		if !ok || !responsePropertyTypeOK(name, rawProperty) {
			return false
		}
	}
	return true
}

// jsonCandidate is one complete JSON object found inside combined output, with
// its byte span in the original raw string.
type jsonCandidate struct {
	text       string
	start, end int
	method     string
}

// fencedBlockRE matches one whole markdown code fence anywhere in the text,
// capturing its body. (?s) so the body may span lines.
var fencedBlockRE = regexp.MustCompile("(?s)```[a-zA-Z]*[ \t]*\n(.*?)```")

// scanJSONCandidates finds every complete JSON object in raw: bodies of fenced
// code blocks first, then balanced top-level objects in the remaining text.
// Candidates are returned in order of appearance. Only COMPLETE objects are
// found — a truncated object simply never becomes a candidate.
func scanJSONCandidates(raw string) []jsonCandidate {
	var candidates []jsonCandidate
	type span struct{ start, end int }
	var fencedSpans []span

	for _, idx := range fencedBlockRE.FindAllStringSubmatchIndex(raw, -1) {
		fencedSpans = append(fencedSpans, span{idx[0], idx[1]})
		body := strings.TrimSpace(raw[idx[2]:idx[3]])
		if _, ok := decodeOneObject(body); ok {
			candidates = append(candidates, jsonCandidate{
				text:   body,
				start:  idx[0],
				end:    idx[1],
				method: ExtractFencedBlock,
			})
		}
	}

	insideFence := func(i int) bool {
		for _, s := range fencedSpans {
			if i >= s.start && i < s.end {
				return true
			}
		}
		return false
	}

	for i := 0; i < len(raw); {
		if raw[i] != '{' || insideFence(i) {
			i++
			continue
		}
		obj, consumed, ok := decodeBalancedObject(raw[i:])
		if !ok {
			i++
			continue
		}
		candidates = append(candidates, jsonCandidate{
			text:   obj,
			start:  i,
			end:    i + consumed,
			method: ExtractBalancedObject,
		})
		i += consumed
	}

	sortCandidatesByStart(candidates)
	return candidates
}

// decodeBalancedObject decodes the JSON value starting at s[0] (a '{'),
// tolerating any trailing content after it. Returns the exact object text and
// how many bytes of s it spans. False when the object never completes (e.g.
// truncated output) or is not valid JSON.
func decodeBalancedObject(s string) (text string, consumed int, ok bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	var rawMsg json.RawMessage
	if err := dec.Decode(&rawMsg); err != nil {
		return "", 0, false
	}
	if len(rawMsg) == 0 || rawMsg[0] != '{' {
		return "", 0, false
	}
	return string(rawMsg), int(dec.InputOffset()), true
}

func sortCandidatesByStart(candidates []jsonCandidate) {
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].start < candidates[j-1].start; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}
}

// nonFinalAround returns everything in raw outside the selected candidate's
// span — the reasoning/diagnostic remainder, preserved verbatim for debugging
// (joined with one newline when the answer sits mid-text).
func nonFinalAround(raw string, c jsonCandidate) string {
	before := strings.TrimSpace(raw[:c.start])
	after := strings.TrimSpace(raw[c.end:])
	switch {
	case before == "":
		return after
	case after == "":
		return before
	default:
		return before + "\n" + after
	}
}
