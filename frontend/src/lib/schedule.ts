// Pure browser-local <-> UTC conversion for channel automation schedules.
// A ScheduleWindow's shape is identical on both sides of the wire (weekday
// 0=Sunday..6=Saturday, start/end in minutes-since-midnight, end<=start
// means the window wraps past midnight into (weekday+1)%7) — only the
// TIME ZONE the minutes are measured in differs, tracked by which function
// you call, never by the type itself. Mirrors backend/automation.Window's
// own Contains semantics exactly, so a window round-trips through the API
// with no drift.
export interface ScheduleWindow {
  weekday: number
  start_minute: number
  end_minute: number
}

const MINUTES_PER_DAY = 1440
const MINUTES_PER_WEEK = 7 * MINUTES_PER_DAY

// currentUtcOffsetMinutes returns how far local time is AHEAD of UTC, in
// minutes (e.g. +300 for UTC+5, -180 for UTC-3). Date.getTimezoneOffset()
// is defined the opposite way round (UTC minus local), hence the negation.
export function currentUtcOffsetMinutes(reference: Date = new Date()): number {
  return -reference.getTimezoneOffset()
}

// formatUtcOffset renders an offset the way operators expect to read it:
// "UTC+5", "UTC-3:30", "UTC+0" — never "UTC+5:00" for a whole-hour offset.
export function formatUtcOffset(offsetMinutes: number): string {
  const sign = offsetMinutes < 0 ? '-' : '+'
  const abs = Math.abs(offsetMinutes)
  const hours = Math.floor(abs / 60)
  const minutes = abs % 60
  return minutes === 0 ? `UTC${sign}${hours}` : `UTC${sign}${hours}:${String(minutes).padStart(2, '0')}`
}

function duration(w: ScheduleWindow): number {
  const raw = w.end_minute - w.start_minute
  return raw > 0 ? raw : raw + MINUTES_PER_DAY
}

function normalizeToWeek(totalMinutes: number): number {
  return ((totalMinutes % MINUTES_PER_WEEK) + MINUTES_PER_WEEK) % MINUTES_PER_WEEK
}

// shiftOne shifts a single window by deltaMinutes (positive or negative),
// returning one window in the common case or two when a FULL 24-hour
// window (duration === 1440) lands on a non-midnight boundary after the
// shift — a single (weekday, start, end) row cannot express "exactly 24h
// starting at a non-zero minute" without start==end, which this schema
// treats as invalid, so that one case is represented as two contiguous
// rows instead (the day it starts on, then the remainder on the next day).
function shiftOne(w: ScheduleWindow, deltaMinutes: number): ScheduleWindow[] {
  const dur = duration(w)
  const shiftedStart = normalizeToWeek(w.weekday * MINUTES_PER_DAY + w.start_minute + deltaMinutes)
  const weekday = Math.floor(shiftedStart / MINUTES_PER_DAY)
  const startMinute = shiftedStart % MINUTES_PER_DAY
  const rawEnd = startMinute + dur

  if (dur === MINUTES_PER_DAY && startMinute !== 0) {
    return [
      { weekday, start_minute: startMinute, end_minute: MINUTES_PER_DAY },
      { weekday: (weekday + 1) % 7, start_minute: 0, end_minute: startMinute },
    ]
  }
  if (rawEnd > MINUTES_PER_DAY) {
    return [{ weekday, start_minute: startMinute, end_minute: rawEnd - MINUTES_PER_DAY }]
  }
  return [{ weekday, start_minute: startMinute, end_minute: rawEnd }]
}

function shiftAll(windows: ScheduleWindow[], deltaMinutes: number): ScheduleWindow[] {
  return windows.flatMap((w) => shiftOne(w, deltaMinutes))
}

// utcToLocal converts stored UTC windows into the browser's local time zone
// for display/editing.
export function utcToLocal(windows: ScheduleWindow[], offsetMinutes: number = currentUtcOffsetMinutes()): ScheduleWindow[] {
  return shiftAll(windows, offsetMinutes)
}

// localToUtc converts browser-local windows (as edited in the dialog) back
// to UTC for saving.
export function localToUtc(windows: ScheduleWindow[], offsetMinutes: number = currentUtcOffsetMinutes()): ScheduleWindow[] {
  return shiftAll(windows, -offsetMinutes)
}

// --- HH:MM <-> minutes, for native <input type="time"> fields ------------

// minutesToHHMM formats start-of-day minutes (or an end already known to be
// < 1440) as "HH:MM".
export function minutesToHHMM(minutes: number): string {
  const m = ((minutes % MINUTES_PER_DAY) + MINUTES_PER_DAY) % MINUTES_PER_DAY
  const h = Math.floor(m / 60)
  const mm = m % 60
  return `${String(h).padStart(2, '0')}:${String(mm).padStart(2, '0')}`
}

export function hhmmToMinutes(hhmm: string): number {
  const [h, m] = hhmm.split(':').map((x) => parseInt(x, 10))
  return (h || 0) * 60 + (m || 0)
}

// endMinutesFromInput/endInputFromMinutes handle the one value a native
// time input cannot express directly: end_minute's valid range is
// [1, 1440], and 0 is never a valid end — so an end-time field showing
// "00:00" unambiguously means "midnight at the END of this day", i.e. 1440,
// never literal 0.
export function endMinutesFromInput(hhmm: string): number {
  const m = hhmmToMinutes(hhmm)
  return m === 0 ? MINUTES_PER_DAY : m
}
export function endInputFromMinutes(minutes: number): string {
  return minutes === MINUTES_PER_DAY ? '00:00' : minutesToHHMM(minutes)
}

// --- presets (expressed in local time; convert with localToUtc to save) --

export type PresetName = 'always' | 'nights' | 'weekends' | 'custom'

// alwaysWindows covers the whole week. It is deliberately timezone-neutral
// (every minute of every day, in ANY zone, is still every minute of every
// day in any other) — used directly as both the local display shape and
// the UTC save payload, bypassing localToUtc/utcToLocal entirely, so it is
// always exactly 7 rows instead of the up-to-14 shiftOne's split rule would
// otherwise produce for a non-hour-aligned offset.
export function alwaysWindows(): ScheduleWindow[] {
  return [0, 1, 2, 3, 4, 5, 6].map((weekday) => ({ weekday, start_minute: 0, end_minute: MINUTES_PER_DAY }))
}

// nightsWindows is every day, startHour -> endHour (default 22:00-06:00,
// wrapping past midnight), in local time.
export function nightsWindows(startHour = 22, endHour = 6): ScheduleWindow[] {
  return [0, 1, 2, 3, 4, 5, 6].map((weekday) => ({ weekday, start_minute: startHour * 60, end_minute: endHour * 60 }))
}

// weekendsWindows is Saturday + Sunday, all day, in local time.
export function weekendsWindows(): ScheduleWindow[] {
  return [
    { weekday: 6, start_minute: 0, end_minute: MINUTES_PER_DAY },
    { weekday: 0, start_minute: 0, end_minute: MINUTES_PER_DAY },
  ]
}

function sameWindowSet(a: ScheduleWindow[], b: ScheduleWindow[]): boolean {
  if (a.length !== b.length) return false
  const key = (w: ScheduleWindow) => `${w.weekday}:${w.start_minute}:${w.end_minute}`
  const as = new Set(a.map(key))
  for (const w of b) if (!as.has(key(w))) return false
  return true
}

// detectPreset classifies a local window list as one of the three canned
// presets, or 'custom' if it matches none exactly.
export function detectPreset(localWindows: ScheduleWindow[]): PresetName {
  if (sameWindowSet(localWindows, alwaysWindows())) return 'always'
  if (sameWindowSet(localWindows, nightsWindows())) return 'nights'
  if (sameWindowSet(localWindows, weekendsWindows())) return 'weekends'
  return 'custom'
}

export const WEEKDAY_COUNT = 7
