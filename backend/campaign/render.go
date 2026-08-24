package campaign

import (
	"regexp"
	"strings"
)

// tokenRe matches a {{variable_name}} placeholder — letters, digits and
// underscore only, optional whitespace inside the braces.
var tokenRe = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

// separators are the punctuation Render treats as "adjacent to a variable"
// when that variable resolves empty — a comma, semicolon or em-dash, each
// followed by one space, matching how a human would actually punctuate a
// dropped clause.
var separators = []string{", ", "; ", " — "}

// Render substitutes every {{variable}} placeholder in tmpl with vars,
// trimming each value first. A variable that resolves empty (missing from
// vars, or present as "" /whitespace-only) is dropped together with ONE
// adjacent separator — preferring the separator immediately BEFORE the
// placeholder, falling back to the one immediately after when there is
// none before — so "Здравствуйте, {{name}}!" with an empty name renders
// "Здравствуйте!", not "Здравствуйте , !" or "Здравствуйте  !". Any
// resulting run of two or more spaces is collapsed to one, and the final
// result is trimmed.
func Render(tmpl string, vars map[string]string) string {
	matches := tokenRe.FindAllStringSubmatchIndex(tmpl, -1)
	if len(matches) == 0 {
		return strings.TrimSpace(collapseSpaces(tmpl))
	}

	var b strings.Builder
	last := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		keyStart, keyEnd := m[2], m[3]
		key := tmpl[keyStart:keyEnd]
		segment := tmpl[last:start]

		val := strings.TrimSpace(vars[key])
		if val != "" {
			b.WriteString(segment)
			b.WriteString(val)
			last = end
			continue
		}

		// Empty value: try to drop one adjacent separator, preferring the
		// one immediately before the placeholder.
		strippedBefore := false
		for _, sep := range separators {
			if strings.HasSuffix(segment, sep) {
				b.WriteString(segment[:len(segment)-len(sep)])
				strippedBefore = true
				break
			}
		}
		if strippedBefore {
			last = end
			continue
		}

		b.WriteString(segment)
		after := tmpl[end:]
		strippedAfter := false
		for _, sep := range separators {
			if strings.HasPrefix(after, sep) {
				last = end + len(sep) // skip the separator entirely
				strippedAfter = true
				break
			}
		}
		if !strippedAfter {
			last = end
		}
	}
	b.WriteString(tmpl[last:])

	return strings.TrimSpace(collapseSpaces(b.String()))
}

// ExtractVariables returns the distinct {{variable}} names referenced in
// tmpl, in first-appearance order. Used to populate campaigns.variables — an
// informational cache the UI renders as "this message uses: name, order"
// without re-parsing the template client-side; Render itself never consults
// it, deriving substitutions straight from whatever a caller passes as vars.
func ExtractVariables(tmpl string) []string {
	matches := tokenRe.FindAllStringSubmatch(tmpl, -1)
	seen := make(map[string]bool, len(matches))
	var out []string
	for _, m := range matches {
		key := m[1]
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}

// collapseSpaces replaces every run of two or more ASCII spaces with one —
// deliberately narrower than \s+ so it never touches newlines a template
// author put there on purpose.
var runOfSpaces = regexp.MustCompile(` {2,}`)

func collapseSpaces(s string) string {
	return runOfSpaces.ReplaceAllString(s, " ")
}
