package pgimport

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	sqlitemigrations "github.com/yerassyldanay/xchats/backend/migrations/sqlite"
)

// TestKindForPgType_CoversEveryContractType parses the REAL embedded
// schema contract and asserts kindForPgType classifies every pg_type
// string that actually appears in it — a type kindForPgType has not been
// taught (like the missing "jsonb" case caught by hand during development,
// before this test existed) must fail loadContract loudly, never silently
// fall through to a wrong classification.
func TestKindForPgType_CoversEveryContractType(t *testing.T) {
	var c contract
	if err := json.Unmarshal(sqlitemigrations.SchemaContractJSON, &c); err != nil {
		t.Fatalf("parse schema_contract.json: %v", err)
	}
	seen := map[string]bool{}
	for tableName, tbl := range c.Tables {
		for _, col := range tbl.Columns {
			seen[col.PgType] = true
			if _, err := kindForPgType(col.PgType); err != nil {
				t.Errorf("table %s column %s: %v", tableName, col.Name, err)
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("the embedded schema contract yielded zero columns — is schema_contract.json actually embedded?")
	}
	t.Logf("classified %d distinct pg_type values across the schema", len(seen))
}

func TestKindForPgType_RejectsUnknownType(t *testing.T) {
	if _, err := kindForPgType("some_made_up_type"); err == nil {
		t.Fatal("kindForPgType accepted an unrecognized type without error")
	}
}

func TestLoadContract_ExcludesViews(t *testing.T) {
	tables, err := loadContract()
	if err != nil {
		t.Fatalf("loadContract: %v", err)
	}
	for _, tbl := range tables {
		if tbl.Name == "inbox_accounts_v" || tbl.Name == "inbox_chats_v" ||
			tbl.Name == "inbox_message_media_v" || tbl.Name == "inbox_messages_v" {
			t.Fatalf("loadContract returned a view (%s) as if it were an importable base table", tbl.Name)
		}
	}
	if len(tables) != 32 {
		t.Fatalf("loadContract returned %d tables, want 32 (the schema's base table count as of this writing — if a migration legitimately added/removed a table, update this number)", len(tables))
	}
}

func TestBindValue_NullScalarStaysNil(t *testing.T) {
	v, err := bindValue(kindScalar, nil)
	if err != nil {
		t.Fatalf("bindValue: %v", err)
	}
	if v != nil {
		t.Fatalf("bindValue(kindScalar, nil) = %v, want nil", v)
	}
}

func TestBindValue_NullArrayBecomesEmptyArrayNotNil(t *testing.T) {
	v, err := bindValue(kindArray, nil)
	if err != nil {
		t.Fatalf("bindValue: %v", err)
	}
	sa, ok := v.(dbx.StringArray)
	if !ok {
		t.Fatalf("bindValue(kindArray, nil) = %T, want dbx.StringArray", v)
	}
	if len(sa) != 0 {
		t.Fatalf("bindValue(kindArray, nil) = %v, want an empty (non-nil-in-effect) array", sa)
	}
}

func TestBindValue_ArrayConvertsElementsToStringArray(t *testing.T) {
	v, err := bindValue(kindArray, []interface{}{"a", "b", "c"})
	if err != nil {
		t.Fatalf("bindValue: %v", err)
	}
	sa, ok := v.(dbx.StringArray)
	if !ok {
		t.Fatalf("bindValue = %T, want dbx.StringArray", v)
	}
	if len(sa) != 3 || sa[0] != "a" || sa[1] != "b" || sa[2] != "c" {
		t.Fatalf("bindValue = %v, want [a b c]", sa)
	}
}

func TestBindValue_ArrayRejectsNonStringElement(t *testing.T) {
	if _, err := bindValue(kindArray, []interface{}{42}); err == nil {
		t.Fatal("bindValue accepted a non-string array element without error")
	}
}

func TestBindValue_TimestampConvertsToUTC(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("tzdata unavailable in this environment: %v", err)
	}
	local := time.Date(2026, 1, 15, 10, 30, 0, 0, loc)
	v, err := bindValue(kindTimestamp, local)
	if err != nil {
		t.Fatalf("bindValue: %v", err)
	}
	got, ok := v.(time.Time)
	if !ok {
		t.Fatalf("bindValue = %T, want time.Time", v)
	}
	if got.Location() != time.UTC {
		t.Fatalf("bindValue location = %v, want UTC", got.Location())
	}
	if !got.Equal(local) {
		t.Fatalf("bindValue = %v, want the same instant as %v", got, local)
	}
}

func TestBindValue_TimestampRejectsWrongType(t *testing.T) {
	if _, err := bindValue(kindTimestamp, "not-a-time"); err == nil {
		t.Fatal("bindValue accepted a non-time.Time value for a timestamp column without error")
	}
}

func TestBindValue_ScalarPassesThroughUnchanged(t *testing.T) {
	for _, v := range []any{"x", int64(5), int32(5), 1.5, true} {
		got, err := bindValue(kindScalar, v)
		if err != nil {
			t.Fatalf("bindValue(%v): %v", v, err)
		}
		if got != v {
			t.Fatalf("bindValue(%v) = %v, want unchanged", v, got)
		}
	}

	// []byte is not comparable with == / !=, so it needs its own
	// byte-by-byte check rather than sharing the loop above.
	b := []byte{1, 2, 3}
	got, err := bindValue(kindScalar, b)
	if err != nil {
		t.Fatalf("bindValue(%v): %v", b, err)
	}
	gb, ok := got.([]byte)
	if !ok || len(gb) != len(b) {
		t.Fatalf("bindValue(%v) = %v, want unchanged", b, got)
	}
	for i := range b {
		if gb[i] != b[i] {
			t.Fatalf("bindValue(%v) = %v, want unchanged", b, got)
		}
	}
}
