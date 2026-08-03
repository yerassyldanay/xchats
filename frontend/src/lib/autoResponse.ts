// Pure helpers for the auto-response schedule editor (AutoResponseDialog.vue).
// No Vue here — mode<->windows conversion, the coverage strip, and the badge
// summary are all plain data transforms so they can be unit tested without
// mounting anything.
import type { AutoResponse, AutoResponseWindow } from '@/types'

// WEEKDAYS follows time.Weekday / Postgres EXTRACT(DOW): Sunday=0..Saturday=6,
// never ISO's Monday=1 — must match the backend exactly or "Пн" here and
// "Monday" there silently mean different columns.
export const WEEKDAYS = [0, 1, 2, 3, 4, 5, 6] as const
export const WEEKDAY_SHORT: Record<number, string> = {
  0: 'Вс', 1: 'Пн', 2: 'Вт', 3: 'Ср', 4: 'Чт', 5: 'Пт', 6: 'Сб',
}
export const WEEKDAY_FULL: Record<number, string> = {
  0: 'Воскресенье', 1: 'Понедельник', 2: 'Вторник', 3: 'Среда',
  4: 'Четверг', 5: 'Пятница', 6: 'Суббота',
}

// A day is always in exactly one of these modes. The dialog never lets a
// user compose a raw wrapping interval directly — start/end here are always
// the business-hours bounds a human would say out loud ("outside 09:00 to
// 18:00"), never the canonical storage split.
export type DayMode = 'off' | 'all_day' | 'outside' | 'inside'
export const DAY_MODES: DayMode[] = ['off', 'all_day', 'outside', 'inside']
export const DAY_MODE_LABELS: Record<DayMode, string> = {
  off: 'Не отвечать',
  all_day: 'Круглосуточно',
  outside: 'Вне интервала',
  inside: 'В интервале',
}

export interface DayState {
  mode: DayMode
  start: string // "HH:MM", meaningful for outside/inside only
  end: string // "HH:MM", meaningful for outside/inside only
}

export function defaultDayState(): DayState {
  return { mode: 'off', start: '09:00', end: '18:00' }
}

function minutesOf(clock: string): number {
  const [h, m] = clock.split(':').map((n) => parseInt(n, 10))
  return (h || 0) * 60 + (m || 0)
}

// windowsToDayStates re-joins the backend's canonical, already-split windows
// (internal/autoresponse.Normalize) back into one mode per weekday — a saved
// "Вне интервала" round-trips into the SAME mode it was saved as, not a pair
// of raw windows the UI would flag as authored elsewhere.
export function windowsToDayStates(windows: AutoResponseWindow[]): Record<number, DayState> {
  const byDay = new Map<number, AutoResponseWindow[]>()
  for (const w of windows) {
    const list = byDay.get(w.weekday) ?? []
    list.push(w)
    byDay.set(w.weekday, list)
  }
  const out: Record<number, DayState> = {}
  for (const wd of WEEKDAYS) {
    const ws = (byDay.get(wd) ?? []).slice().sort((a, b) => minutesOf(a.start) - minutesOf(b.start))
    out[wd] = dayStateFromWindows(ws)
  }
  return out
}

function dayStateFromWindows(ws: AutoResponseWindow[]): DayState {
  if (ws.length === 0) return defaultDayState()
  if (ws.length === 1) {
    const [w] = ws
    if (w.start === '00:00' && w.end === '24:00') return { ...defaultDayState(), mode: 'all_day' }
    return { mode: 'inside', start: w.start, end: w.end }
  }
  if (ws.length === 2) {
    const [a, b] = ws
    if (a.start === '00:00' && b.end === '24:00' && minutesOf(a.end) <= minutesOf(b.start)) {
      return { mode: 'outside', start: a.end, end: b.start }
    }
  }
  // Anything else (multiple disjoint windows) was written outside this UI —
  // approximate it as "inside" the earliest window rather than silently
  // dropping data on the next save; the coverage strip still shows the real,
  // unsimplified picture regardless of how this label reads.
  const [w] = ws
  return { mode: 'inside', start: w.start, end: w.end }
}

// dayStatesToWindows is windowsToDayStates's inverse: one raw window per
// active day. "Вне интервала HH:MM-HH:MM" is submitted with start/end
// SWAPPED ({start: end, end: start}) — the same-weekday wrap the backend's
// ParseClock+Normalize expects and splits into the two canonical rows
// itself; this module never does that split.
export function dayStatesToWindows(days: Record<number, DayState>): AutoResponseWindow[] {
  const out: AutoResponseWindow[] = []
  for (const wd of WEEKDAYS) {
    const d = days[wd]
    if (!d || d.mode === 'off') continue
    if (d.mode === 'all_day') out.push({ weekday: wd, start: '00:00', end: '24:00' })
    else if (d.mode === 'inside') out.push({ weekday: wd, start: d.start, end: d.end })
    else if (d.mode === 'outside') out.push({ weekday: wd, start: d.end, end: d.start })
  }
  return out
}

// normalizeForCoverage mirrors internal/autoresponse.Normalize's wrap-split
// only (never the merge step — coverage only needs membership, not a
// canonical minimal set).
function normalizeForCoverage(windows: AutoResponseWindow[]): AutoResponseWindow[] {
  const out: AutoResponseWindow[] = []
  for (const w of windows) {
    const s = minutesOf(w.start)
    const e = minutesOf(w.end)
    if (s < e) {
      out.push(w)
    } else if (s > e) {
      if (e > 0) out.push({ weekday: w.weekday, start: '00:00', end: w.end })
      if (s < 1440) out.push({ weekday: w.weekday, start: w.start, end: '24:00' })
    }
    // s === e: zero-length, contributes nothing — same as the backend.
  }
  return out
}

// coverageStrip returns a 7x24 (weekday x hour) grid of whether that hour is
// covered — the only honest way to show a per-day schedule is really a set
// of per-day rules, not spills, and to surface a residual gap before saving.
export function coverageStrip(days: Record<number, DayState>): boolean[][] {
  const normalized = normalizeForCoverage(dayStatesToWindows(days))
  return WEEKDAYS.map((wd) => {
    const row: boolean[] = []
    for (let h = 0; h < 24; h++) {
      const minute = h * 60
      row.push(normalized.some((w) => w.weekday === wd && minute >= minutesOf(w.start) && minute < minutesOf(w.end)))
    }
    return row
  })
}

export function formatDelaySeconds(seconds: number): string {
  if (seconds <= 0) return 'без задержки'
  if (seconds < 60) return `${seconds} сек`
  const minutes = seconds / 60
  if (minutes < 60) return `${minutes} мин`
  const hours = minutes / 60
  return `${hours} ч`
}

export function formatCooldownSeconds(seconds: number): string {
  if (seconds <= 0) return 'без ограничения'
  return formatDelaySeconds(seconds)
}

// summarizeAutoResponse renders the accounts card's compact status line.
export function summarizeAutoResponse(policy: AutoResponse): string {
  if (!policy.enabled) return 'Выключен'
  const days = windowsToDayStates(policy.windows)
  const active = WEEKDAYS.filter((wd) => days[wd].mode !== 'off')
  if (active.length === 0) return 'Включён, расписание не задано'

  const dayLabel = (d: DayState): string => {
    if (d.mode === 'all_day') return 'круглосуточно'
    if (d.mode === 'outside') return `вне ${d.start}–${d.end}`
    if (d.mode === 'inside') return `${d.start}–${d.end}`
    return ''
  }
  const modeLabel = policy.reply_mode === 'fixed' ? 'Фикс. текст' : 'ИИ'
  const first = days[active[0]]
  const sameEveryActiveDay = active.every(
    (wd) => days[wd].mode === first.mode && days[wd].start === first.start && days[wd].end === first.end,
  )
  if (active.length === 7 && sameEveryActiveDay) {
    return `${modeLabel} · каждый день, ${dayLabel(first)}`
  }
  if (sameEveryActiveDay) {
    return `${modeLabel} · ${active.length} дн., ${dayLabel(first)}`
  }
  return `${modeLabel} · ${active.length} дн.`
}
