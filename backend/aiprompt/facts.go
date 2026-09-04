package aiprompt

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// facts.go is the "virtual fact column" data contract: a seller-authored,
// per-entity list of {ref, value, instruction} triples that become
// {{product.<ref>.<fact_ref>}} / {{tariff.<ref>.<fact_ref>}} /
// {{tariff_info.main.<fact_ref>}} prompt tokens exactly the way a
// code-owned registry column (registry.go) becomes {{product.<ref>.price}}
// — except the column itself is operator-defined rather than closed at
// compile time. The model always sees the token and Instruction, never
// Value: catalog.go/contract.go are the only places Value is ever read.
//
// AdditionalFact is stored as one element of the additional_facts JSON
// array column (ai_products/ai_tariffs/ai_tariff_info) — see
// backend/migrations/sqlite/0017_kb_virtual_facts.up.sql.

// factRefPattern is the closed syntax for a virtual fact's ref: lowercase
// snake_case, starting with a letter, so it can never collide with a
// {{table.ref.column}} token's own separators and reads identically to a
// code-owned column name (registry.go's Column values follow the same
// shape).
var factRefPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

const (
	// MaxAdditionalFacts bounds how many virtual facts one entity may carry
	// — generous enough for a real spec sheet, small enough that a prompt
	// can never be blown out by an unbounded list.
	MaxAdditionalFacts = 50
	// MaxFactInstructionLen bounds Instruction's length in Unicode code
	// points, not bytes (prompt-visible prose, rendered once per eligible
	// request — see renderAdditionalFacts) — ValidateFacts checks it with
	// utf8.RuneCountInString so Cyrillic/Kazakh text gets the same limit in
	// characters a Latin-only string would, not half of it.
	MaxFactInstructionLen = 500
	// MaxFactStringValueLen bounds a string-typed Value's length in Unicode
	// code points, the same way MaxFactInstructionLen does.
	MaxFactStringValueLen = 300
)

// AdditionalFact is one virtual fact column: Ref names the token segment,
// Value is the exact hidden scalar (never shown to the model), Instruction
// is the prompt-visible explanation of the fact and how to phrase it
// safely. Value holds exactly one of json.Number, bool, or string — never
// nil, an array, or an object (UnmarshalJSON enforces this; ValidateFacts
// enforces everything else: ref syntax, uniqueness, non-collision with
// concrete columns, non-empty/trimmed values, and instruction hygiene).
type AdditionalFact struct {
	Ref         string
	Value       any // json.Number | bool | string
	Instruction string
}

// jsonAdditionalFact is AdditionalFact's wire shape — used only to drive
// (Un)MarshalJSON so the exported struct itself needs no json tags and
// Value can be decoded through a json.Number-preserving path (plain
// json.Unmarshal into `any` always collapses a JSON number to float64,
// silently rounding a large or precise value — see json.Decoder.UseNumber).
type jsonAdditionalFact struct {
	Ref         string          `json:"ref"`
	Value       json.RawMessage `json:"value"`
	Instruction string          `json:"instruction"`
}

// UnmarshalJSON enforces the additional-fact JSON SHAPE: exactly the three
// keys ref/value/instruction (an unknown key is a hard decode error, the
// same discipline ValidateResponse holds the model-facing contract to —
// aiprompt: unknown JSON keys always fail closed rather than being
// silently ignored), and value is exactly one JSON number, boolean, or
// string — never null, an array, or an object. Semantic validation (ref
// syntax, non-empty, length limits, instruction hygiene, uniqueness,
// collision with concrete columns) is ValidateFacts' job, run explicitly
// by every write path — decoding a single already-stored fact has no way
// to see its siblings or the owning entity's concrete columns.
func (f *AdditionalFact) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var raw jsonAdditionalFact
	if err := dec.Decode(&raw); err != nil {
		return fmt.Errorf("aiprompt: invalid additional fact: %w", err)
	}
	if len(raw.Value) == 0 {
		return fmt.Errorf("aiprompt: additional fact %q is missing a value", raw.Ref)
	}
	valueDec := json.NewDecoder(bytes.NewReader(raw.Value))
	valueDec.UseNumber()
	var value any
	if err := valueDec.Decode(&value); err != nil {
		return fmt.Errorf("aiprompt: additional fact %q has an invalid value: %w", raw.Ref, err)
	}
	switch value.(type) {
	case json.Number, bool, string:
		// allowed
	case nil:
		return fmt.Errorf("aiprompt: additional fact %q: value must not be null", raw.Ref)
	default:
		return fmt.Errorf("aiprompt: additional fact %q: value must be a number, boolean, or string — not an array or object", raw.Ref)
	}
	f.Ref = raw.Ref
	f.Value = value
	f.Instruction = raw.Instruction
	return nil
}

// MarshalJSON round-trips Value's exact JSON representation — json.Number
// marshals back to a bare numeral (it IS the numeral's own text), bool and
// string marshal natively.
func (f AdditionalFact) MarshalJSON() ([]byte, error) {
	switch f.Value.(type) {
	case json.Number, bool, string:
	default:
		return nil, fmt.Errorf("aiprompt: additional fact %q has an unmarshalable value type %T", f.Ref, f.Value)
	}
	return json.Marshal(jsonAdditionalFact{Ref: f.Ref, Value: mustMarshal(f.Value), Instruction: f.Instruction})
}

// FactsColumn adapts a []AdditionalFact for the additional_facts JSON TEXT
// column every persistence package that reads/writes ai_products/
// ai_tariffs/ai_tariff_info shares (internal/kbstore, internal/
// responsestore) — the same "named-slice-type implementing driver.Valuer/
// sql.Scanner, converted at the exact Scan/bind call site" idiom
// internal/dbx.UUIDArray establishes, just defined here instead of dbx so
// dbx itself never needs to import a business type (aiprompt has no
// database dependency of its own — see this package's doc comment — and
// database/sql/driver is the bare interface package, not a driver, so this
// does not touch the "only internal/dbx may import database/sql" boundary
// internal/dbtest's architecture test enforces). A nil slice both scans
// from and is written as "[]", never SQL NULL, matching every other JSON
// array column in the schema.
type FactsColumn []AdditionalFact

// Value implements driver.Valuer.
func (a FactsColumn) Value() (driver.Value, error) {
	if a == nil {
		a = FactsColumn{}
	}
	b, err := json.Marshal([]AdditionalFact(a))
	if err != nil {
		return nil, fmt.Errorf("aiprompt: marshal FactsColumn: %w", err)
	}
	return string(b), nil
}

// Scan implements sql.Scanner.
func (a *FactsColumn) Scan(src any) error {
	if src == nil {
		*a = FactsColumn{}
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		return fmt.Errorf("aiprompt: FactsColumn: cannot scan %T", src)
	}
	var out []AdditionalFact
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("aiprompt: unmarshal FactsColumn %q: %w", raw, err)
	}
	*a = out
	return nil
}

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err) // json.Number | bool | string can never fail to marshal
	}
	return b
}

// valueKindAndText classifies v (a decoded AdditionalFact.Value) into the
// substitution ValueKind it drives and the value's canonical text form —
// the number's own digits, or the trimmed string, verbatim. Booleans have
// no single "text" (substitution instead picks reviewed wording per
// reply_language — see resolveVirtualFact in contract.go), so their text
// return is always "", ok true.
func valueKindAndText(v any) (text string, kind ValueKind, ok bool) {
	switch val := v.(type) {
	case json.Number:
		return val.String(), KindVirtualNumber, true
	case bool:
		return "", KindVirtualBoolean, true
	case string:
		return strings.TrimSpace(val), KindVirtualString, true
	default:
		return "", "", false
	}
}

const (
	// maxSafeIntegerMagnitude is JavaScript's Number.MAX_SAFE_INTEGER
	// (2^53 - 1): the largest-magnitude whole number a float64 — and
	// therefore the number <input> AdditionalFactsEditor.vue's
	// setNumberValue uses — can hold without rounding. This package itself
	// preserves a json.Number's exact source digits (UnmarshalJSON's
	// UseNumber path), but that guarantee is worthless if the browser
	// already mangled the digits before the request left the KB editor, so
	// ValidateFacts rejects what the frontend cannot edit exactly rather
	// than accepting a value only the backend can represent.
	maxSafeIntegerMagnitude = 9007199254740991
	// maxSafeSignificantDigits bounds a decimal (non-integer) numeric
	// fact's significant digits to what a float64 reliably round-trips —
	// comfortably inside its guaranteed ~15-17 digit precision. Ordinary
	// decimals (0.3, 12.5) are nowhere near this and are unaffected; this
	// only catches a value with more digits than anyone reads or types by
	// hand, e.g. an unrounded computed result pasted in.
	maxSafeSignificantDigits = 15
)

// numberIsJSSafe reports whether n can round-trip through a JS `Number` —
// what the KB editor's number <input> parses every keystroke into, and what
// JSON.stringify re-serializes on submit — without silently losing digits.
// A whole number is checked by exact magnitude (Number.isSafeInteger's own
// rule); a decimal is checked by significant-digit count rather than
// bit-exact float64 representability, since almost no ordinary decimal
// (0.3 included) is bit-exact and that would reject nearly everything.
func numberIsJSSafe(n json.Number) bool {
	s := string(n)
	mantissa := s
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mantissa = s[:i]
	}
	if !strings.Contains(mantissa, ".") {
		i, err := n.Int64()
		if err != nil {
			return false // more digits than even int64 holds — well past float64-safe
		}
		return i <= maxSafeIntegerMagnitude && i >= -maxSafeIntegerMagnitude
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, mantissa)
	digits = strings.Trim(digits, "0")
	return len(digits) <= maxSafeSignificantDigits
}

// ValidateFacts enforces the full semantic contract for one entity's
// additional_facts list (a product, a tariff, or the tariff_info
// singleton). reservedRefs is that entity's own concrete fact-column names
// (e.g. []string{"price"} for a product) — a virtual ref may never shadow
// one. prose is every OTHER prompt-visible field the same entity carries
// (description, advantages, best_for, availability_note, ... — whatever
// the caller has; a nil/empty map is fine for tariff_info, which has none)
// keyed by a human-readable field name used only in error text.
//
// Every write path (kbstore's MCP upserts, live-write patches, draft
// merges) calls this before a fact list is ever persisted, so invalid
// input never reaches kbd_draft or a live ai_* row. aiprompt.BuildCatalog
// calls it again before emitting a single virtual token — defense in
// depth against a fact that somehow reached storage unvalidated (a future
// write path that forgets to call this, a manual DB edit), exactly the
// same belt-and-suspenders validRef already applies to a product/tariff's
// own ref.
func ValidateFacts(facts []AdditionalFact, reservedRefs []string, prose map[string]string) error {
	if len(facts) > MaxAdditionalFacts {
		return fmt.Errorf("aiprompt: %d additional facts exceeds the limit of %d", len(facts), MaxAdditionalFacts)
	}
	reserved := make(map[string]bool, len(reservedRefs))
	for _, r := range reservedRefs {
		reserved[r] = true
	}
	seen := make(map[string]bool, len(facts))
	for _, f := range facts {
		if !factRefPattern.MatchString(f.Ref) {
			return fmt.Errorf("aiprompt: additional fact ref %q must match %s", f.Ref, factRefPattern.String())
		}
		if reserved[f.Ref] {
			return fmt.Errorf("aiprompt: additional fact ref %q collides with a concrete fact column of the same name", f.Ref)
		}
		if seen[f.Ref] {
			return fmt.Errorf("aiprompt: duplicate additional fact ref %q", f.Ref)
		}
		seen[f.Ref] = true

		text, kind, ok := valueKindAndText(f.Value)
		if !ok {
			return fmt.Errorf("aiprompt: additional fact %q has an unsupported value type %T", f.Ref, f.Value)
		}
		if kind == KindVirtualString && text == "" {
			return fmt.Errorf("aiprompt: additional fact %q: string value must be non-empty after trimming", f.Ref)
		}
		if kind == KindVirtualString && utf8.RuneCountInString(text) > MaxFactStringValueLen {
			return fmt.Errorf("aiprompt: additional fact %q: value exceeds %d characters", f.Ref, MaxFactStringValueLen)
		}
		if kind == KindVirtualNumber && !numberIsJSSafe(f.Value.(json.Number)) {
			return fmt.Errorf("aiprompt: additional fact %q: numeric value cannot be edited exactly in the KB editor "+
				"(whole numbers must stay within ±%d, decimals within %d significant digits) — store it as a string instead",
				f.Ref, maxSafeIntegerMagnitude, maxSafeSignificantDigits)
		}

		instruction := strings.TrimSpace(f.Instruction)
		if instruction == "" {
			return fmt.Errorf("aiprompt: additional fact %q: instruction is required", f.Ref)
		}
		if utf8.RuneCountInString(instruction) > MaxFactInstructionLen {
			return fmt.Errorf("aiprompt: additional fact %q: instruction exceeds %d characters", f.Ref, MaxFactInstructionLen)
		}
		if placeholderPattern.MatchString(instruction) {
			return fmt.Errorf("aiprompt: additional fact %q: instruction must not contain a {{...}} fact token", f.Ref)
		}
		// Numbers/strings only — see valueKindAndText's doc comment on why a
		// boolean's localized wording is never leak-checked.
		if (kind == KindVirtualNumber || kind == KindVirtualString) && containsLiteral(instruction, text) {
			return fmt.Errorf("aiprompt: additional fact %q: instruction must not contain the fact's own exact value", f.Ref)
		}
	}
	for _, f := range facts {
		text, kind, ok := valueKindAndText(f.Value)
		if !ok || kind == KindVirtualBoolean {
			continue
		}
		for field, s := range prose {
			if s == "" {
				continue
			}
			if containsLiteral(s, text) {
				return fmt.Errorf("aiprompt: %s must not contain the exact value of additional fact %q", field, f.Ref)
			}
		}
	}
	return nil
}
