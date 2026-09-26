package aiprompt

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// schedule.go is the shared weekly-schedule contract for the beauty-salon
// vertical (PLAN.md "Beauty Salon Knowledge Base Extension"): one JSON
// column (ai_contacts.schedule, ai_specialists.schedule) holding at most 7
// ScheduleDay entries, one per weekday actually worked. It is the single
// source of truth for:
//   - NormalizeSchedule: the write-path validator every persistence package
//     (internal/kbstore) calls before a schedule is ever stored, exactly the
//     way facts.go's ValidateFacts gates additional_facts.
//   - the customer-facing schedule/schedule_<day> fact tokens (catalog.go's
//     buildFacts, via scheduleDayText/scheduleFullText below).
//   - the machine-readable schedule-reasoning lines rendered into the
//     %%SPECIALISTS%% block (prompt.go's renderSpecialists), which show the
//     model raw shift/break boundaries so it can reason about an arbitrary
//     customer-requested interval — the model must still answer using the
//     token, never by writing those boundaries itself; see
//     validateScheduleLiteralContract in contract.go.
//   - EvaluateShift/EvaluateShiftPoint, the reference implementation of
//     PLAN.md's half-open-interval shift/break classification rules.

// WeekdayRef is the closed, authoritative weekday key. Display labels
// (ScheduleDay.Day) are import/authoring metadata only — every reasoning
// and lookup path keys on Ref.
type WeekdayRef string

const (
	Monday    WeekdayRef = "mon"
	Tuesday   WeekdayRef = "tue"
	Wednesday WeekdayRef = "wed"
	Thursday  WeekdayRef = "thu"
	Friday    WeekdayRef = "fri"
	Saturday  WeekdayRef = "sat"
	Sunday    WeekdayRef = "sun"
)

// weekdayOrder is Monday-first canonical chronological order — the order
// NormalizeSchedule sorts into and every renderer iterates in.
var weekdayOrder = []WeekdayRef{Monday, Tuesday, Wednesday, Thursday, Friday, Saturday, Sunday}

var weekdayIndex = func() map[WeekdayRef]int {
	m := make(map[WeekdayRef]int, len(weekdayOrder))
	for i, w := range weekdayOrder {
		m[w] = i
	}
	return m
}()

func validWeekdayRef(ref WeekdayRef) bool {
	_, ok := weekdayIndex[ref]
	return ok
}

// ScheduleBreak is one break inside a ScheduleDay's shift, HH:MM 24-hour.
type ScheduleBreak struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// ScheduleDay is one worked weekday: a single shift interval plus any
// number of breaks inside it. V1 supports one shift interval per day — a
// split shift (two separate working intervals) is out of scope.
type ScheduleDay struct {
	Ref    WeekdayRef      `json:"ref"`
	Day    string          `json:"day"`
	Start  string          `json:"start"`
	End    string          `json:"end"`
	Breaks []ScheduleBreak `json:"breaks"`
}

// Schedule is the full weekly contract: at most 7 entries, one per worked
// weekday. A missing weekday is an off-day. nil/empty means "no structured
// schedule" (the entity falls back to prose/booking-only handling).
type Schedule []ScheduleDay

// ScheduleColumn adapts a Schedule for the `schedule` JSON TEXT column
// ai_contacts/ai_specialists share — the same "named-slice-type
// implementing driver.Valuer/sql.Scanner, converted at the exact Scan/bind
// call site" idiom as aiprompt.FactsColumn (facts.go) and internal/dbx.
// UUIDArray. A nil schedule both scans from and is written as "[]", never
// SQL NULL.
type ScheduleColumn Schedule

// Value implements driver.Valuer.
func (s ScheduleColumn) Value() (driver.Value, error) {
	if s == nil {
		s = ScheduleColumn{}
	}
	b, err := json.Marshal(Schedule(s))
	if err != nil {
		return nil, fmt.Errorf("aiprompt: marshal ScheduleColumn: %w", err)
	}
	return string(b), nil
}

// Scan implements sql.Scanner.
func (s *ScheduleColumn) Scan(src any) error {
	if src == nil {
		*s = ScheduleColumn{}
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		return fmt.Errorf("aiprompt: ScheduleColumn: cannot scan %T", src)
	}
	var out []ScheduleDay
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("aiprompt: unmarshal ScheduleColumn %q: %w", raw, err)
	}
	*s = out
	return nil
}

var hhmmPattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// parseHHMM converts a validated "HH:MM" string to minutes since midnight.
// Callers must check hhmmPattern (or rely on NormalizeSchedule having
// already validated it) — this never receives unvalidated input in
// production, so it panics on malformed input rather than threading a
// second error return through every caller (mirrors availabilityStatusRU's
// "BuildCatalog should have rejected it first" fail-loud doctrine).
func parseHHMM(s string) int {
	if !hhmmPattern.MatchString(s) {
		panic(fmt.Sprintf("aiprompt: parseHHMM: invalid HH:MM %q — caller must validate first", s))
	}
	h, _ := strconv.Atoi(s[0:2])
	m, _ := strconv.Atoi(s[3:5])
	return h*60 + m
}

// NormalizeSchedule validates the full semantic contract for one Schedule
// and returns a canonically-ordered copy (weekdays Monday-first, each day's
// breaks sorted by start time) ready to persist. Every write path
// (internal/kbstore) calls this before a schedule ever reaches storage,
// exactly the way facts.go's ValidateFacts gates additional_facts before a
// write. Rules (PLAN.md "Validation rules"):
//   - a weekday occurs at most once
//   - ref is one of mon..sun; start/end/break times are valid 24-hour HH:MM
//   - start < end (overnight shifts are rejected)
//   - every break lies inside [start,end) and breaks do not overlap each other
func NormalizeSchedule(s Schedule) (Schedule, error) {
	if len(s) == 0 {
		return Schedule{}, nil
	}
	if len(s) > len(weekdayOrder) {
		return nil, fmt.Errorf("aiprompt: schedule has %d entries, more than the %d weekdays", len(s), len(weekdayOrder))
	}
	out := make(Schedule, len(s))
	copy(out, s)
	seen := make(map[WeekdayRef]bool, len(out))
	for i := range out {
		d := &out[i]
		if !validWeekdayRef(d.Ref) {
			return nil, fmt.Errorf("aiprompt: schedule day has invalid ref %q", d.Ref)
		}
		if seen[d.Ref] {
			return nil, fmt.Errorf("aiprompt: schedule day %q appears more than once", d.Ref)
		}
		seen[d.Ref] = true
		if !hhmmPattern.MatchString(d.Start) {
			return nil, fmt.Errorf("aiprompt: schedule day %q has invalid start time %q", d.Ref, d.Start)
		}
		if !hhmmPattern.MatchString(d.End) {
			return nil, fmt.Errorf("aiprompt: schedule day %q has invalid end time %q", d.Ref, d.End)
		}
		start, end := parseHHMM(d.Start), parseHHMM(d.End)
		if start >= end {
			return nil, fmt.Errorf("aiprompt: schedule day %q: start must be before end (overnight shifts are not supported)", d.Ref)
		}
		breaks := make([]ScheduleBreak, len(d.Breaks))
		copy(breaks, d.Breaks)
		sort.Slice(breaks, func(a, b int) bool { return breaks[a].Start < breaks[b].Start })
		prevEnd := -1
		for _, br := range breaks {
			if !hhmmPattern.MatchString(br.Start) {
				return nil, fmt.Errorf("aiprompt: schedule day %q has a break with invalid start time %q", d.Ref, br.Start)
			}
			if !hhmmPattern.MatchString(br.End) {
				return nil, fmt.Errorf("aiprompt: schedule day %q has a break with invalid end time %q", d.Ref, br.End)
			}
			bStart, bEnd := parseHHMM(br.Start), parseHHMM(br.End)
			if bStart >= bEnd {
				return nil, fmt.Errorf("aiprompt: schedule day %q: break start must be before break end", d.Ref)
			}
			if bStart < start || bEnd > end {
				return nil, fmt.Errorf("aiprompt: schedule day %q: break %s–%s is not inside the shift %s–%s", d.Ref, br.Start, br.End, d.Start, d.End)
			}
			if prevEnd >= 0 && bStart < prevEnd {
				return nil, fmt.Errorf("aiprompt: schedule day %q: breaks overlap", d.Ref)
			}
			prevEnd = bEnd
		}
		d.Breaks = breaks
	}
	sort.Slice(out, func(a, b int) bool { return weekdayIndex[out[a].Ref] < weekdayIndex[out[b].Ref] })
	return out, nil
}

// DayByRef returns the schedule entry for ref, or nil when that weekday is
// not worked.
func (s Schedule) DayByRef(ref WeekdayRef) *ScheduleDay {
	for i := range s {
		if s[i].Ref == ref {
			return &s[i]
		}
	}
	return nil
}

// weekdayNamesRU/KK are the reviewed display names scheduleDayText renders.
// KK is DRAFT machine-translated, pending native speaker review — same
// doctrine as every other Kazakh wording table in this package (registry.go's
// DeliveryWordingKK).
var weekdayNamesRU = map[WeekdayRef]string{
	Monday: "понедельник", Tuesday: "вторник", Wednesday: "среда",
	Thursday: "четверг", Friday: "пятница", Saturday: "суббота", Sunday: "воскресенье",
}

var weekdayNamesKK = map[WeekdayRef]string{
	Monday: "дүйсенбі", Tuesday: "сейсенбі", Wednesday: "сәрсенбі",
	Thursday: "бейсенбі", Friday: "жұма", Saturday: "сенбі", Sunday: "жексенбі",
}

func weekdayName(ref WeekdayRef, lang string) string {
	if lang == "kk" {
		return weekdayNamesKK[ref]
	}
	return weekdayNamesRU[ref]
}

// scheduleDayText renders one worked weekday's complete customer-ready
// sentence fragment — the value the schedule_<day> fact token resolves to.
// Never called for an off-day: DayByRef returning nil means no token at all
// ("only scheduled days receive daily tokens" — PLAN.md).
func scheduleDayText(d ScheduleDay, lang string) string {
	name := weekdayName(d.Ref, lang)
	if name == "" {
		name = string(d.Ref)
	}
	text := capitalizeFirst(name) + ": " + d.Start + "–" + d.End
	if len(d.Breaks) == 0 {
		return text
	}
	var parts []string
	for _, br := range d.Breaks {
		parts = append(parts, br.Start+"–"+br.End)
	}
	if lang == "kk" {
		word := "үзіліс"
		if len(parts) > 1 {
			word = "үзілістер"
		}
		return text + " (" + word + " " + strings.Join(parts, ", ") + ")"
	}
	word := "перерыв"
	if len(parts) > 1 {
		word = "перерывы"
	}
	return text + " (" + word + " " + strings.Join(parts, ", ") + ")"
}

// scheduleFullText renders the complete working week: one scheduleDayText
// line per worked day, in canonical (Monday-first) order. Empty schedule
// yields "" — no token (addFact already omits a blank value).
func scheduleFullText(s Schedule, lang string) string {
	var lines []string
	for _, ref := range weekdayOrder {
		if d := s.DayByRef(ref); d != nil {
			lines = append(lines, scheduleDayText(*d, lang))
		}
	}
	return strings.Join(lines, "; ")
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// scheduleReasoningLines renders the machine-readable weekday/shift/break
// boundary lines the %%SPECIALISTS%% block shows the model (PLAN.md: "a
// machine-readable schedule reasoning section containing weekday, shift,
// and break boundaries"). Every one of the 7 weekdays gets a line —
// including off-days, stated explicitly, so the model never has to infer
// "not listed" as "not worked" — because the customer's requested day/time
// is free text the model itself must classify (EvaluateShift documents the
// exact rule the model is instructed to follow); code cannot pre-classify
// every conceivable phrasing. This is the one place raw shift/break clock
// times legitimately appear in the prompt: the model must still answer
// using the schedule/schedule_<day> TOKEN, never by copying these digits
// into reply_text — validateScheduleLiteralContract (contract.go) enforces
// that at response-validation time.
func scheduleReasoningLines(s Schedule) []string {
	lines := make([]string, 0, len(weekdayOrder))
	for _, ref := range weekdayOrder {
		d := s.DayByRef(ref)
		if d == nil {
			lines = append(lines, string(ref)+": off")
			continue
		}
		line := string(ref) + ": " + d.Start + "-" + d.End
		if len(d.Breaks) > 0 {
			var parts []string
			for _, br := range d.Breaks {
				parts = append(parts, br.Start+"-"+br.End)
			}
			line += " break " + strings.Join(parts, ",")
		}
		lines = append(lines, line)
	}
	return lines
}

// scheduleTimePattern matches any HH:MM-shaped clock time, with or without
// a leading zero on a single-digit hour (0-9, 00-23) — validateScheduleLiteralContract
// (contract.go) uses it to catch the model writing a shift/break boundary
// itself instead of using a schedule token. "9:00" is exactly as much a
// leaked literal as "09:00"; the pattern must not require the padding a
// model has no reason to always produce.
var scheduleTimePattern = regexp.MustCompile(`\b([01]?\d|2[0-3]):[0-5]\d\b`)

// spelledOutHourPattern catches the common "at <spelled-out hour> <daypart>"
// Russian construction ("работает с десяти утра") that scheduleTimePattern,
// being digit-shaped, cannot see at all. A curated stem list for one through
// twelve plus every daypart word and noon/midnight, not a numeral parser —
// an uncommon phrasing (an ordinal "в десятом часу", spelled-out minutes)
// can still slip through. The same documented, accepted tradeoff
// bookingConfirmationRE (contract.go) makes: catching the common real
// phrasings beats matching nothing while chasing a complete grammar.
// No leading \b: RE2's word-boundary class is ASCII-only ([0-9A-Za-z_]), so
// \b never matches at a Cyrillic-letter boundary — the same reason
// bookingConfirmationRE (above) carries no \b either.
var spelledOutHourPattern = regexp.MustCompile(`(?i)(час|полдень|полночь|одиннадцат|двенадцат|десят|девят|восем|сем|шест|пят|четыр|тр[её]|дв[ае]х?|одн[оа])[а-я]*\s+(утра|дня|вечера|ночи)`)

// isSalonOrganization reports whether kb carries any structured salon data
// (PLAN.md "Beauty Salon Knowledge Base Extension") — scopes
// validateScheduleLiteralContract/validateSalonConfirmationGuard (contract.go)
// to salon organizations only, so a non-salon org's existing shop-kb
// prompt/response behavior never changes. Checks kb.Specialists/kb.Services
// directly (the same definition response.salonFrameSelected uses to pick
// the frame in the first place) rather than the DERIVED presence of a
// schedule-shaped fact column: a specialist whose schedule happens to be
// empty (no hours entered yet) still contributes zero schedule_* facts to
// the catalog, so that derived signal silently went missing — and with it,
// both guards — for a salon org that plainly is one.
func isSalonOrganization(kb *KB) bool {
	return kb != nil && (len(kb.Specialists) > 0 || len(kb.Services) > 0)
}

// ShiftState is EvaluateShift/EvaluateShiftPoint's classification result.
type ShiftState string

const (
	// ShiftOnDuty: the requested interval/point is fully within the shift
	// and touches no break.
	ShiftOnDuty ShiftState = "on_shift"
	// ShiftBreakOverlap: the day is worked and the requested interval/point
	// would otherwise be in-shift, but it intersects a break.
	ShiftBreakOverlap ShiftState = "break_overlap"
	// ShiftOff: the weekday is not worked at all, the point/interval falls
	// outside the shift, or an interval only partially overlaps the shift.
	// PLAN.md: "partial shift or break overlap is not a valid complete
	// interval" — a partial overlap is classified the same as fully
	// outside, not as a third state, since neither is a bookable complete
	// interval.
	ShiftOff ShiftState = "off_shift"
)

// EvaluateShiftPoint classifies a single point in time against a weekday's
// schedule (day == nil means the weekday is not worked at all) using
// PLAN.md's rule: "a point time is on-shift when start <= time < end and it
// is outside every break." at must be a valid HH:MM string.
func EvaluateShiftPoint(day *ScheduleDay, at string) (ShiftState, error) {
	if !hhmmPattern.MatchString(at) {
		return "", fmt.Errorf("aiprompt: invalid time %q", at)
	}
	if day == nil {
		return ShiftOff, nil
	}
	t := parseHHMM(at)
	start, end := parseHHMM(day.Start), parseHHMM(day.End)
	if t < start || t >= end {
		return ShiftOff, nil
	}
	for _, br := range day.Breaks {
		bStart, bEnd := parseHHMM(br.Start), parseHHMM(br.End)
		if t >= bStart && t < bEnd {
			return ShiftBreakOverlap, nil
		}
	}
	return ShiftOnDuty, nil
}

// EvaluateShift classifies a half-open interval [reqStart,reqEnd) against a
// weekday's schedule (day == nil means the weekday is not worked at all),
// using PLAN.md's rules:
//   - on-shift: reqStart < reqEnd, shift.start <= reqStart, reqEnd <=
//     shift.end, and the interval does not overlap any break
//   - break overlap: reqStart < break.end && reqEnd > break.start for some
//     break, while otherwise within the shift
//   - off/outside: the day is not worked, or the interval is fully or
//     partially outside [shift.start, shift.end) — a partial overlap is
//     never a valid complete interval, so it classifies the same as fully
//     outside, not as a distinct state
func EvaluateShift(day *ScheduleDay, reqStart, reqEnd string) (ShiftState, error) {
	if !hhmmPattern.MatchString(reqStart) {
		return "", fmt.Errorf("aiprompt: invalid interval start %q", reqStart)
	}
	if !hhmmPattern.MatchString(reqEnd) {
		return "", fmt.Errorf("aiprompt: invalid interval end %q", reqEnd)
	}
	a, b := parseHHMM(reqStart), parseHHMM(reqEnd)
	if a >= b {
		return "", fmt.Errorf("aiprompt: interval start %q must be before end %q", reqStart, reqEnd)
	}
	if day == nil {
		return ShiftOff, nil
	}
	start, end := parseHHMM(day.Start), parseHHMM(day.End)
	if a < start || b > end {
		return ShiftOff, nil // fully or partially outside the shift
	}
	for _, br := range day.Breaks {
		bStart, bEnd := parseHHMM(br.Start), parseHHMM(br.End)
		if a < bEnd && b > bStart {
			return ShiftBreakOverlap, nil
		}
	}
	return ShiftOnDuty, nil
}
