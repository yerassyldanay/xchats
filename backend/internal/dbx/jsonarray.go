package dbx

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// UUIDArray adapts a []uuid.UUID for a uuid[] column ported to a SQLite TEXT
// column holding a JSON array (with a json_valid CHECK — see the schema
// type-mapping table). A nil slice both scans from and is written as "[]",
// never SQL NULL: every uuid[] column in this schema is NOT NULL DEFAULT
// '{}'. It is also what the `= ANY($n)` -> `IN (SELECT value FROM
// json_each(?))` translation binds its right-hand side as.
//
// Used as a type conversion at the exact Scan/bind call site for one of
// these columns — rows.Scan((*dbx.UUIDArray)(&t.GalleryImages)) and
// tx.Exec(ctx, q, ..., dbx.UUIDArray(t.GalleryImages)) — rather than
// changing the struct field's own type, so every other read of that field
// stays a plain []uuid.UUID.
type UUIDArray []uuid.UUID

// Value implements driver.Valuer.
func (a UUIDArray) Value() (driver.Value, error) {
	if a == nil {
		a = UUIDArray{}
	}
	b, err := json.Marshal([]uuid.UUID(a))
	if err != nil {
		return nil, fmt.Errorf("dbx: marshal UUIDArray: %w", err)
	}
	return string(b), nil
}

// Scan implements sql.Scanner.
func (a *UUIDArray) Scan(src any) error {
	if src == nil {
		*a = UUIDArray{}
		return nil
	}
	raw, err := scanBytes(src)
	if err != nil {
		return fmt.Errorf("dbx: UUIDArray: %w", err)
	}
	var out []uuid.UUID
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("dbx: unmarshal UUIDArray %q: %w", raw, err)
	}
	*a = out
	return nil
}

// StringArray is UUIDArray's sibling for the schema's one text[] column
// (mcp_oauth_clients.redirect_uris).
type StringArray []string

// Value implements driver.Valuer.
func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		a = StringArray{}
	}
	b, err := json.Marshal([]string(a))
	if err != nil {
		return nil, fmt.Errorf("dbx: marshal StringArray: %w", err)
	}
	return string(b), nil
}

// Scan implements sql.Scanner.
func (a *StringArray) Scan(src any) error {
	if src == nil {
		*a = StringArray{}
		return nil
	}
	raw, err := scanBytes(src)
	if err != nil {
		return fmt.Errorf("dbx: StringArray: %w", err)
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("dbx: unmarshal StringArray %q: %w", raw, err)
	}
	*a = out
	return nil
}

func scanBytes(src any) ([]byte, error) {
	switch v := src.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	default:
		return nil, fmt.Errorf("cannot scan %T", src)
	}
}
