package aiprompt

import (
	"encoding/json"
	"strings"
	"testing"
)

func decodeFact(t *testing.T, raw string) (AdditionalFact, error) {
	t.Helper()
	var f AdditionalFact
	err := json.Unmarshal([]byte(raw), &f)
	return f, err
}

func TestAdditionalFact_UnmarshalJSON_AcceptsNumberBooleanString(t *testing.T) {
	f, err := decodeFact(t, `{"ref":"limit_on_devices","value":5,"instruction":"Максимальное количество устройств."}`)
	if err != nil {
		t.Fatalf("unmarshal number: %v", err)
	}
	n, ok := f.Value.(json.Number)
	if !ok || n.String() != "5" {
		t.Errorf("Value = %#v, want json.Number(5)", f.Value)
	}

	f, err = decodeFact(t, `{"ref":"has_warranty","value":true,"instruction":"Есть ли гарантия."}`)
	if err != nil {
		t.Fatalf("unmarshal bool: %v", err)
	}
	if b, ok := f.Value.(bool); !ok || !b {
		t.Errorf("Value = %#v, want bool(true)", f.Value)
	}

	f, err = decodeFact(t, `{"ref":"model_code","value":"XZ-500","instruction":"Точный код модели."}`)
	if err != nil {
		t.Fatalf("unmarshal string: %v", err)
	}
	if s, ok := f.Value.(string); !ok || s != "XZ-500" {
		t.Errorf("Value = %#v, want string(XZ-500)", f.Value)
	}
}

// TestAdditionalFact_UnmarshalJSON_PreservesLargeIntegerPrecision is the
// json.Number contract's whole point: decoding a value through a plain
// interface{} (no UseNumber) silently rounds a large integer through
// float64. A hidden exact fact silently losing precision would be a
// correctness bug no validation could ever catch downstream.
func TestAdditionalFact_UnmarshalJSON_PreservesLargeIntegerPrecision(t *testing.T) {
	f, err := decodeFact(t, `{"ref":"serial_max","value":9007199254740993,"instruction":"x"}`)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	n, ok := f.Value.(json.Number)
	if !ok {
		t.Fatalf("Value = %#v, want json.Number", f.Value)
	}
	if n.String() != "9007199254740993" {
		t.Errorf("Value.String() = %q, want %q (float64 would round this)", n.String(), "9007199254740993")
	}
}

func TestAdditionalFact_UnmarshalJSON_RejectsNullArrayObject(t *testing.T) {
	cases := []string{
		`{"ref":"x","value":null,"instruction":"i"}`,
		`{"ref":"x","value":[1,2],"instruction":"i"}`,
		`{"ref":"x","value":{"a":1},"instruction":"i"}`,
	}
	for _, raw := range cases {
		if _, err := decodeFact(t, raw); err == nil {
			t.Errorf("decode(%s): want error, got nil", raw)
		}
	}
}

func TestAdditionalFact_UnmarshalJSON_RejectsUnknownKeys(t *testing.T) {
	_, err := decodeFact(t, `{"ref":"x","value":1,"instruction":"i","extra":"nope"}`)
	if err == nil {
		t.Fatal("want error for unknown key, got nil")
	}
}

// TestAdditionalFact_MarshalJSON_RoundTripsNumberVerbatim asserts the
// numeral's exact source text survives a decode/encode/decode round trip
// UNCHANGED — json.Number preserves the literal digits as written, never
// renormalizing "3.140000" to "3.14" the way a float64 round trip would
// (float64 would also risk losing precision on a large integer, which
// TestAdditionalFact_UnmarshalJSON_PreservesLargeIntegerPrecision covers
// directly).
func TestAdditionalFact_MarshalJSON_RoundTripsNumberVerbatim(t *testing.T) {
	f, err := decodeFact(t, `{"ref":"x","value":3.140000,"instruction":"i"}`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back AdditionalFact
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	n, ok := back.Value.(json.Number)
	if !ok || n.String() != "3.140000" {
		t.Errorf("round-tripped Value = %#v, want json.Number(3.140000) preserved verbatim", back.Value)
	}
	if !strings.Contains(string(b), `"value":3.140000`) {
		t.Errorf("marshaled JSON = %s, want a bare numeral for value", b)
	}
}

func numFact(ref string, n int64, instruction string) AdditionalFact {
	return AdditionalFact{Ref: ref, Value: json.Number(itoa(n)), Instruction: instruction}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestValidateFacts_ValidList(t *testing.T) {
	facts := []AdditionalFact{
		numFact("limit_on_devices", 5, "Максимальное количество устройств. Формулируй нейтрально: «Количество устройств: …»."),
		{Ref: "has_trial", Value: true, Instruction: "Есть ли пробный период."},
		{Ref: "model_code", Value: "XZ-500", Instruction: "Точный код модели, используемый в гарантийных документах."},
	}
	if err := ValidateFacts(facts, []string{"price"}, nil); err != nil {
		t.Fatalf("ValidateFacts: unexpected error: %v", err)
	}
}

func TestValidateFacts_RefSyntax(t *testing.T) {
	cases := []struct {
		name string
		ref  string
		ok   bool
	}{
		{"lowercase snake case", "working_pressure", true},
		{"single letter", "x", true},
		{"leading digit rejected", "5limit", false},
		{"uppercase rejected", "Limit", false},
		{"hyphen rejected", "limit-on-devices", false},
		{"dot rejected", "limit.on.devices", false},
		{"empty rejected", "", false},
		{"space rejected", "limit on devices", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			facts := []AdditionalFact{{Ref: c.ref, Value: json.Number("1"), Instruction: "instruction text"}}
			err := ValidateFacts(facts, nil, nil)
			if c.ok && err != nil {
				t.Errorf("ref %q: unexpected error: %v", c.ref, err)
			}
			if !c.ok && err == nil {
				t.Errorf("ref %q: want error, got nil", c.ref)
			}
		})
	}
}

func TestValidateFacts_RejectsCollisionWithConcreteColumn(t *testing.T) {
	facts := []AdditionalFact{{Ref: "price", Value: json.Number("1"), Instruction: "i"}}
	err := ValidateFacts(facts, []string{"price", "fee"}, nil)
	if err == nil {
		t.Fatal("want error for ref colliding with a concrete column, got nil")
	}
}

func TestValidateFacts_RejectsDuplicateRef(t *testing.T) {
	facts := []AdditionalFact{
		{Ref: "limit_on_devices", Value: json.Number("5"), Instruction: "first"},
		{Ref: "limit_on_devices", Value: json.Number("7"), Instruction: "second"},
	}
	err := ValidateFacts(facts, nil, nil)
	if err == nil {
		t.Fatal("want error for duplicate ref, got nil")
	}
}

func TestValidateFacts_RejectsEmptyStringValue(t *testing.T) {
	facts := []AdditionalFact{{Ref: "model_code", Value: "   ", Instruction: "i"}}
	if err := ValidateFacts(facts, nil, nil); err == nil {
		t.Fatal("want error for blank string value, got nil")
	}
}

func TestValidateFacts_RejectsMissingOrEmptyInstruction(t *testing.T) {
	for _, instr := range []string{"", "   "} {
		facts := []AdditionalFact{{Ref: "x", Value: json.Number("1"), Instruction: instr}}
		if err := ValidateFacts(facts, nil, nil); err == nil {
			t.Errorf("instruction %q: want error, got nil", instr)
		}
	}
}

func TestValidateFacts_RejectsOverLongInstruction(t *testing.T) {
	facts := []AdditionalFact{{Ref: "x", Value: json.Number("1"), Instruction: strings.Repeat("a", MaxFactInstructionLen+1)}}
	if err := ValidateFacts(facts, nil, nil); err == nil {
		t.Fatal("want error for over-long instruction, got nil")
	}
}

func TestValidateFacts_RejectsOverLongStringValue(t *testing.T) {
	facts := []AdditionalFact{{Ref: "x", Value: strings.Repeat("a", MaxFactStringValueLen+1), Instruction: "i"}}
	if err := ValidateFacts(facts, nil, nil); err == nil {
		t.Fatal("want error for over-long string value, got nil")
	}
}

// TestValidateFacts_LengthLimitsCountUnicodeCharactersNotBytes guards the
// byte-vs-rune-count bug MaxFactStringValueLen/MaxFactInstructionLen exist
// to avoid: a Cyrillic or Kazakh string at exactly the documented character
// limit must be accepted, and one character over must be rejected. Every
// character here is 2 bytes in UTF-8, so a byte-length check would reject
// the "exactly at the limit" case at roughly half the advertised length —
// this is the boundary a byte-based regression would actually trip on.
func TestValidateFacts_LengthLimitsCountUnicodeCharactersNotBytes(t *testing.T) {
	atLimit := strings.Repeat("б", MaxFactStringValueLen)
	overLimit := strings.Repeat("б", MaxFactStringValueLen+1)

	if err := ValidateFacts([]AdditionalFact{{Ref: "x", Value: atLimit, Instruction: "инструкция"}}, nil, nil); err != nil {
		t.Fatalf("a %d-character Cyrillic value at the limit must be accepted, got: %v", MaxFactStringValueLen, err)
	}
	if err := ValidateFacts([]AdditionalFact{{Ref: "x", Value: overLimit, Instruction: "инструкция"}}, nil, nil); err == nil {
		t.Fatalf("a %d-character Cyrillic value one over the limit must be rejected, got nil", MaxFactStringValueLen+1)
	}

	atLimitInstruction := strings.Repeat("ә", MaxFactInstructionLen) // Kazakh-specific letter, still 2 bytes in UTF-8
	overLimitInstruction := strings.Repeat("ә", MaxFactInstructionLen+1)

	if err := ValidateFacts([]AdditionalFact{{Ref: "y", Value: json.Number("1"), Instruction: atLimitInstruction}}, nil, nil); err != nil {
		t.Fatalf("a %d-character Kazakh instruction at the limit must be accepted, got: %v", MaxFactInstructionLen, err)
	}
	if err := ValidateFacts([]AdditionalFact{{Ref: "y", Value: json.Number("1"), Instruction: overLimitInstruction}}, nil, nil); err == nil {
		t.Fatalf("a %d-character Kazakh instruction one over the limit must be rejected, got nil", MaxFactInstructionLen+1)
	}
}

// TestValidateFacts_RejectsNumbersTheKBEditorCannotEditExactly guards the
// exact-value guarantee a json.Number's UseNumber decoding promises: this
// package can hold arbitrary source digits, but the frontend's number
// <input> parses through JS `Number()` on every keystroke, which silently
// rounds anything outside these two cases — so ValidateFacts must reject
// them rather than accept a value the KB editor cannot round-trip.
func TestValidateFacts_RejectsNumbersTheKBEditorCannotEditExactly(t *testing.T) {
	cases := []struct {
		name  string
		value json.Number
	}{
		{"integer one past Number.MAX_SAFE_INTEGER", json.Number("9007199254740993")},
		{"long decimal beyond float64 precision", json.Number("1.234567890123456789")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			facts := []AdditionalFact{{Ref: "x", Value: c.value, Instruction: "i"}}
			if err := ValidateFacts(facts, nil, nil); err == nil {
				t.Fatalf("want error for %s (%s), got nil", c.name, c.value)
			}
		})
	}
}

// TestValidateFacts_AllowsOrdinaryNumbers is the counterpart to the test
// above: everyday whole numbers and decimals — including ones with no
// special binary-fraction relationship, like 0.3 — must still pass, since
// the guard is significant-digit count, not bit-exact float64
// representability (which almost no ordinary decimal has).
func TestValidateFacts_AllowsOrdinaryNumbers(t *testing.T) {
	for _, v := range []json.Number{"5", "-12", "185000", "0.3", "12.5", "9007199254740991", "-9007199254740991"} {
		facts := []AdditionalFact{{Ref: "x", Value: v, Instruction: "i"}}
		if err := ValidateFacts(facts, nil, nil); err != nil {
			t.Errorf("ordinary number %s must be accepted, got: %v", v, err)
		}
	}
}

func TestValidateFacts_RejectsTooManyFacts(t *testing.T) {
	facts := make([]AdditionalFact, MaxAdditionalFacts+1)
	for i := range facts {
		facts[i] = AdditionalFact{Ref: "fact_" + itoaPlain(i), Value: json.Number("1"), Instruction: "i"}
	}
	if err := ValidateFacts(facts, nil, nil); err == nil {
		t.Fatal("want error exceeding MaxAdditionalFacts, got nil")
	}
}

func itoaPlain(i int) string {
	if i == 0 {
		return "0"
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}

// TestValidateFacts_RejectsInstructionContainingItsOwnValue is the leak
// check on the fact's own instruction — required by the spec's "an
// instruction containing its exact value is rejected".
func TestValidateFacts_RejectsInstructionContainingItsOwnValue(t *testing.T) {
	facts := []AdditionalFact{
		{Ref: "limit_on_devices", Value: json.Number("5"), Instruction: "Максимум 5 устройств."},
	}
	err := ValidateFacts(facts, nil, nil)
	if err == nil {
		t.Fatal("want error: instruction leaks its own exact value, got nil")
	}
}

func TestValidateFacts_RejectsInstructionContainingFactToken(t *testing.T) {
	facts := []AdditionalFact{
		{Ref: "limit_on_devices", Value: json.Number("5"), Instruction: "См. {{product.x.limit_on_devices}} для деталей."},
	}
	err := ValidateFacts(facts, nil, nil)
	if err == nil {
		t.Fatal("want error: instruction contains a {{...}} token, got nil")
	}
}

// TestValidateFacts_AllowsInstructionContainingUnrelatedNumber guards
// against an overly aggressive leak check: an instruction may still discuss
// numbers in general terms as long as it never states THIS fact's own
// exact value.
func TestValidateFacts_AllowsInstructionContainingUnrelatedNumber(t *testing.T) {
	facts := []AdditionalFact{
		{Ref: "limit_on_devices", Value: json.Number("5"), Instruction: "Формулируй нейтрально, без указания конкретного числа в тексте пояснения."},
	}
	if err := ValidateFacts(facts, nil, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestValidateFacts_RejectsProseLeak covers "Ensure exact additional-fact
// values never leak into the prompt through ... description, advantages
// ...": a product's OTHER prose fields must not restate a hidden fact's
// exact value either.
func TestValidateFacts_RejectsProseLeak(t *testing.T) {
	facts := []AdditionalFact{
		{Ref: "working_pressure", Value: json.Number("250"), Instruction: "Рабочее давление, бар."},
	}
	prose := map[string]string{"description": "Рабочее давление 250 бар, надёжный насос."}
	err := ValidateFacts(facts, nil, prose)
	if err == nil {
		t.Fatal("want error: description leaks the fact's exact value, got nil")
	}
}

func TestValidateFacts_BooleanValuesExemptFromLeakCheck(t *testing.T) {
	facts := []AdditionalFact{
		{Ref: "has_trial", Value: true, Instruction: "Есть ли пробный период."},
	}
	// A prose field mentioning "true"/"да" is common, ordinary language —
	// booleans carry no distinctive literal to leak-check against.
	prose := map[string]string{"description": "Да, это отличный товар."}
	if err := ValidateFacts(facts, nil, prose); err != nil {
		t.Errorf("unexpected error for boolean fact vs. ordinary prose: %v", err)
	}
}

func TestValidateFacts_EmptyListIsValid(t *testing.T) {
	if err := ValidateFacts(nil, []string{"price"}, map[string]string{"description": "любой текст"}); err != nil {
		t.Errorf("unexpected error for empty facts list: %v", err)
	}
}
