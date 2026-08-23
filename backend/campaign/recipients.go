package campaign

import (
	"strconv"
	"strings"
)

// PreviewStatus is a parsed recipient row's bucket before any persistence —
// distinct from RecipientStatus, which is a PERSISTED row's lifecycle state.
type PreviewStatus string

const (
	PreviewValid     PreviewStatus = "valid"
	PreviewInvalid   PreviewStatus = "invalid"
	PreviewDuplicate PreviewStatus = "duplicate"
)

// ParsedRecipient is one row extracted from pasted or uploaded input.
type ParsedRecipient struct {
	Raw string // the row's original identity cell, pre-normalization
	// Name seeds the channel contact's display_name — from a detected
	// "name" column (see nameHeaderNames) or, header-less, a lone second
	// column. Not itself included in Attributes: callers building Render's
	// vars combine Name (as "name") with Attributes themselves.
	Name       string
	Attributes map[string]string // every other detected column — feeds Render's vars

	NormalizedIdentity string // "" unless Status is PreviewValid or PreviewDuplicate
	Status             PreviewStatus
	Reason             string // populated for PreviewInvalid/PreviewDuplicate
}

// ParseResult is ParseRecipients' (or ApplyUnreachable's) full bucketed output.
type ParseResult struct {
	Rows      []ParsedRecipient
	Valid     int
	Invalid   int
	Duplicate int
}

// Total is the number of rows actually extracted from the input.
func (r ParseResult) Total() int { return len(r.Rows) }

// phoneHeaderNames/nameHeaderNames are the header labels ParseRecipients
// recognizes (case-insensitively, trimmed) for the identity and name
// columns.
var phoneHeaderNames = map[string]bool{
	"phone": true, "phone_number": true, "number": true, "tel": true,
	"telephone": true, "mobile": true, "msisdn": true, "destination": true,
	"recipient": true, "contact": true, "chat_id": true,
}
var nameHeaderNames = map[string]bool{
	"name": true, "full_name": true, "fullname": true, "display_name": true,
}

// headerDigitThreshold is how many digit characters a first-row cell needs
// before ParseRecipients treats that row as DATA rather than a header —
// see detectHeader's own doc comment for why this is a heuristic, not a
// certainty.
const headerDigitThreshold = 3

// ParseRecipients turns raw pasted or uploaded text into bucketed rows.
//
// Rows: split on newlines when the input has any; a single-line paste with
// no newlines is instead split on whichever of comma/semicolon/tab actually
// occurs in it (a flat list like "+7 701 ..., +7 702 ...").
//
// Columns: within multi-row input the delimiter is detected once from the
// first row (the first of comma/semicolon/tab that appears in it; none
// found means a single column) and applied to every row. The first row is
// treated as a HEADER only when it has at least 2 rows total AND no cell in
// it contains headerDigitThreshold or more digits — so "name,phone,city" is
// recognized as a header but "Aigul,77011234567,Almaty" is not. With a
// header, a column named like "phone" (phoneHeaderNames) is the identity
// column and one named like "name" (nameHeaderNames) seeds Name; every
// other column becomes an Attribute keyed by its (trimmed) header text.
// Without a header, column 0 is always the identity and, for exactly two
// columns, column 1 is treated as Name.
//
// channel selects the identity's shape: for ChannelTelegram every cell must
// be a bare (optionally "-"-prefixed, for a group/channel) numeric chat id
// — "@username" is rejected explicitly, with a reason naming why, since the
// Bot API cannot resolve a username to a chat. Every other channel treats
// the identity as a phone number, normalized against defaultCountry (see
// NormalizePhone).
//
// Deduplication is against the NORMALIZED IDENTITY within this one batch: a
// row whose identity repeats an earlier VALID row's is bucketed Duplicate,
// not Invalid — it isn't wrong, it's redundant. Rows already Invalid never
// participate in dedup (there is no identity to compare).
func ParseRecipients(raw, channel, defaultCountry string) ParseResult {
	rows := splitRows(raw)
	if len(rows) == 0 {
		return ParseResult{}
	}

	delim := detectDelimiter(rows[0])
	table := make([][]string, len(rows))
	for i, r := range rows {
		table[i] = splitColumns(r, delim)
	}

	header, dataStart := detectHeader(table)
	phoneCol, nameCol, attrCols := resolveColumns(header, table[dataStart:])

	var out ParseResult
	seen := make(map[string]bool, len(table)-dataStart)
	for _, cols := range table[dataStart:] {
		row := parseRow(cols, phoneCol, nameCol, attrCols, channel, defaultCountry)
		if row.Status == PreviewValid {
			if seen[row.NormalizedIdentity] {
				row.Status = PreviewDuplicate
				row.Reason = "duplicate of an earlier row in this list"
			} else {
				seen[row.NormalizedIdentity] = true
			}
		}
		switch row.Status {
		case PreviewValid:
			out.Valid++
		case PreviewDuplicate:
			out.Duplicate++
		default:
			out.Invalid++
		}
		out.Rows = append(out.Rows, row)
	}
	return out
}

// ApplyUnreachable demotes every currently-Valid row whose NormalizedIdentity
// is present in unreachable to Invalid, stamping reason. This is the pure
// half of two checks the HTTP preview handler resolves live (against a
// provider or the store) and passes in here: the WhatsApp IsOnWhatsApp
// batch check ("not registered on WhatsApp") and a warm-only channel's
// existing-conversation gate ("no existing conversation on this account").
// Rows already Invalid or Duplicate are left exactly as they were.
func ApplyUnreachable(result ParseResult, unreachable map[string]bool, reason string) ParseResult {
	if len(unreachable) == 0 {
		return result
	}
	out := ParseResult{Rows: make([]ParsedRecipient, len(result.Rows)), Valid: result.Valid, Invalid: result.Invalid, Duplicate: result.Duplicate}
	for i, row := range result.Rows {
		if row.Status == PreviewValid && unreachable[row.NormalizedIdentity] {
			row.Status = PreviewInvalid
			row.Reason = reason
			out.Valid--
			out.Invalid++
		}
		out.Rows[i] = row
	}
	return out
}

// --- row/column splitting --------------------------------------------------

func splitRows(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	if strings.Contains(raw, "\n") {
		return splitNonEmpty(raw, "\n")
	}
	// A single-line paste: split on whichever flat separator actually
	// occurs, so "a, b, c" and "a; b; c" and "a\tb\tc" all become 3 rows.
	for _, sep := range []string{",", ";", "\t"} {
		if strings.Contains(raw, sep) {
			return splitNonEmpty(raw, sep)
		}
	}
	return splitNonEmpty(raw, "\n") // a lone value, or nothing at all
}

func splitNonEmpty(s, sep string) []string {
	var out []string
	for _, part := range strings.Split(s, sep) {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func detectDelimiter(firstRow string) string {
	for _, sep := range []string{",", ";", "\t"} {
		if strings.Contains(firstRow, sep) {
			return sep
		}
	}
	return "" // single column
}

func splitColumns(row, delim string) []string {
	if delim == "" {
		return []string{strings.TrimSpace(row)}
	}
	parts := strings.Split(row, delim)
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out
}

// detectHeader reports whether table's first row is a header and, if so,
// where the data actually starts. This is a heuristic, not a certainty: a
// header row is recognized by having NO cell with headerDigitThreshold or
// more digits, which correctly separates "name,phone,city" from real
// phone-bearing data but can misfire on a multi-row list whose first row is
// itself malformed, non-numeric garbage (there is no signal left to tell
// that apart from a genuine header). A single-row input is always data —
// there is nothing to send if the one row present were treated as a header
// instead.
func detectHeader(table [][]string) (header []string, dataStart int) {
	if len(table) < 2 {
		return nil, 0
	}
	for _, cell := range table[0] {
		if countDigits(cell) >= headerDigitThreshold {
			return nil, 0
		}
	}
	return table[0], 1
}

func countDigits(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	return n
}

// resolveColumns picks which column index is the identity, which (if any)
// is the name, and which remaining indices become Attributes (keyed by
// their header text) — from header when there is one, else positionally.
func resolveColumns(header []string, dataRows [][]string) (phoneCol, nameCol int, attrCols map[int]string) {
	nameCol = -1
	if header != nil {
		phoneCol = -1
		for i, h := range header {
			key := strings.ToLower(strings.TrimSpace(h))
			if phoneCol == -1 && phoneHeaderNames[key] {
				phoneCol = i
			}
			if nameCol == -1 && nameHeaderNames[key] {
				nameCol = i
			}
		}
		if phoneCol == -1 {
			phoneCol = columnWithMostDigits(dataRows) // no recognized header — best-effort
		}
		attrCols = map[int]string{}
		for i, h := range header {
			if i == phoneCol || i == nameCol {
				continue
			}
			key := strings.TrimSpace(h)
			if key == "" {
				key = "col" + strconv.Itoa(i+1)
			}
			attrCols[i] = key
		}
		return phoneCol, nameCol, attrCols
	}

	// No header: column 0 is always the identity. Exactly two columns ->
	// column 1 is Name (the common "phone,name" paste shape); more than two
	// columns with no header get positional col2/col3/... attribute keys.
	phoneCol = 0
	width := 0
	for _, row := range dataRows {
		if len(row) > width {
			width = len(row)
		}
	}
	attrCols = map[int]string{}
	if width == 2 {
		nameCol = 1
	} else {
		for i := 1; i < width; i++ {
			attrCols[i] = "col" + strconv.Itoa(i+1)
		}
	}
	return phoneCol, nameCol, attrCols
}

func columnWithMostDigits(dataRows [][]string) int {
	if len(dataRows) == 0 {
		return 0
	}
	best, bestDigits := 0, -1
	for i, cell := range dataRows[0] {
		if d := countDigits(cell); d > bestDigits {
			best, bestDigits = i, d
		}
	}
	return best
}

func cellAt(cols []string, i int) string {
	if i < 0 || i >= len(cols) {
		return ""
	}
	return strings.TrimSpace(cols[i])
}

// --- per-row parsing ---------------------------------------------------

func parseRow(cols []string, phoneCol, nameCol int, attrCols map[int]string, channel, defaultCountry string) ParsedRecipient {
	raw := cellAt(cols, phoneCol)
	row := ParsedRecipient{Raw: raw}
	if nameCol >= 0 {
		row.Name = cellAt(cols, nameCol)
	}
	if len(attrCols) > 0 {
		row.Attributes = make(map[string]string, len(attrCols))
		for i, key := range attrCols {
			if v := cellAt(cols, i); v != "" {
				row.Attributes[key] = v
			}
		}
	}
	if raw == "" {
		row.Status = PreviewInvalid
		row.Reason = "no phone number in this row"
		return row
	}

	if channel == ChannelTelegram {
		id, ok := normalizeTelegramChatID(raw)
		if !ok {
			row.Status = PreviewInvalid
			if strings.HasPrefix(raw, "@") {
				row.Reason = "Telegram usernames can't be resolved to a chat — pick from an existing conversation instead"
			} else {
				row.Reason = "must be a numeric Telegram chat id"
			}
			return row
		}
		row.NormalizedIdentity = id
		row.Status = PreviewValid
		return row
	}

	id, ok := NormalizePhone(raw, defaultCountry)
	if !ok {
		row.Status = PreviewInvalid
		row.Reason = "could not parse a phone number"
		return row
	}
	row.NormalizedIdentity = id
	row.Status = PreviewValid
	return row
}

func normalizeTelegramChatID(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	neg := strings.HasPrefix(s, "-")
	digits := s
	if neg {
		digits = s[1:]
	}
	if digits == "" {
		return "", false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	if neg {
		return "-" + digits, true
	}
	return digits, true
}

// --- phone normalization -----------------------------------------------

// MinPhoneDigits/MaxPhoneDigits bound a "plausible E.164" number — the same
// 7-15 digit policy internal/httpapi's handleCreateChat already applies to
// a manually-typed compose number (see digitsOnly there).
const (
	MinPhoneDigits = 7
	MaxPhoneDigits = 15
)

// NormalizePhone normalizes raw against defaultCountry (bare digits, no
// leading "+", e.g. "7"): strip everything but digits, then in order —
//
//	leading "8" + 10 more digits          -> replace the leading 8 with defaultCountry
//	already defaultCountry-prefixed + 10  -> keep as-is
//	exactly 10 digits                     -> prepend defaultCountry
//	otherwise                             -> accept as-is if it is a
//	                                          plausible E.164 number
//	                                          (MinPhoneDigits..MaxPhoneDigits
//	                                          digits) — a foreign number
//	                                          already carrying its own
//	                                          country code
//
// ok is false when none of the above produces a plausible number.
func NormalizePhone(raw, defaultCountry string) (normalized string, ok bool) {
	digits := digitsOnly(raw)
	switch {
	case len(digits) == 11 && strings.HasPrefix(digits, "8"):
		return defaultCountry + digits[1:], true
	case strings.HasPrefix(digits, defaultCountry) && len(digits) == len(defaultCountry)+10:
		return digits, true
	case len(digits) == 10:
		return defaultCountry + digits, true
	case len(digits) >= MinPhoneDigits && len(digits) <= MaxPhoneDigits:
		return digits, true
	default:
		return "", false
	}
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteByte(byte(r))
		}
	}
	return b.String()
}
