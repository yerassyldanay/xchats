import { describe, expect, it } from 'vitest'
import type { AutoResponse, AutoResponseWindow } from '@/types'
import {
  WEEKDAY_FULL,
  WEEKDAY_SHORT,
  WEEKDAYS,
  coverageStrip,
  dayStatesToWindows,
  defaultDayState,
  formatCooldownSeconds,
  formatDelaySeconds,
  summarizeAutoResponse,
  windowsToDayStates,
  type DayState,
} from './autoResponse'

// allOff is the seed every dialog starts from before anything is loaded.
function allOff(): Record<number, DayState> {
  return Object.fromEntries(WEEKDAYS.map((wd) => [wd, defaultDayState()])) as Record<number, DayState>
}

function policy(over: Partial<AutoResponse> & { windows: AutoResponseWindow[] }): AutoResponse {
  return {
    enabled: true,
    reply_mode: 'ai',
    fixed_text: '',
    timezone: 'Asia/Almaty',
    delay_seconds: 120,
    cooldown_seconds: 300,
    skip_when_escalated: true,
    pause_when_assigned: false,
    ...over,
  }
}

describe('WEEKDAYS convention', () => {
  // This must never drift from Go's time.Weekday / Postgres EXTRACT(DOW): a
  // silent switch to ISO's Monday=1 here would make every schedule saved
  // through this UI land on the wrong day server-side.
  it('pins Sunday=0..Saturday=6, never ISO Monday=1', () => {
    expect(WEEKDAYS).toEqual([0, 1, 2, 3, 4, 5, 6])
    expect(WEEKDAY_SHORT[0]).toBe('Вс')
    expect(WEEKDAY_SHORT[1]).toBe('Пн')
    expect(WEEKDAY_FULL[0]).toBe('Воскресенье')
    expect(WEEKDAY_FULL[6]).toBe('Суббота')
  })
})

describe('windowsToDayStates', () => {
  it('defaults every weekday to off when there are no windows', () => {
    const days = windowsToDayStates([])
    for (const wd of WEEKDAYS) expect(days[wd]).toEqual(defaultDayState())
  })

  it('reads a full-day window as all_day', () => {
    const days = windowsToDayStates([{ weekday: 3, start: '00:00', end: '24:00' }])
    expect(days[3].mode).toBe('all_day')
    expect(days[1].mode).toBe('off')
  })

  it('reads a single non-wrapping window as inside', () => {
    const days = windowsToDayStates([{ weekday: 2, start: '09:00', end: '18:00' }])
    expect(days[2]).toEqual({ mode: 'inside', start: '09:00', end: '18:00' })
  })

  it('re-joins the backend\'s canonical split pair back into outside — the round-trip the schedule editor depends on', () => {
    // This is exactly the shape internal/autoresponse.Normalize produces for a
    // same-weekday wrap ("outside 09:00-18:00" -> [00:00,09:00) + [18:00,24:00)).
    const days = windowsToDayStates([
      { weekday: 1, start: '00:00', end: '09:00' },
      { weekday: 1, start: '18:00', end: '24:00' },
    ])
    expect(days[1]).toEqual({ mode: 'outside', start: '09:00', end: '18:00' })
  })

  it('re-joins regardless of input order (sorts by start minute first)', () => {
    const days = windowsToDayStates([
      { weekday: 1, start: '18:00', end: '24:00' },
      { weekday: 1, start: '00:00', end: '09:00' },
    ])
    expect(days[1]).toEqual({ mode: 'outside', start: '09:00', end: '18:00' })
  })

  it('a pair that does not match the outside shape falls back to inside the earliest window, never dropping data silently', () => {
    const days = windowsToDayStates([
      { weekday: 4, start: '08:00', end: '10:00' },
      { weekday: 4, start: '14:00', end: '16:00' },
    ])
    expect(days[4]).toEqual({ mode: 'inside', start: '08:00', end: '10:00' })
  })

  it('three-or-more windows on one day also fall back to inside the earliest', () => {
    const days = windowsToDayStates([
      { weekday: 5, start: '08:00', end: '10:00' },
      { weekday: 5, start: '12:00', end: '13:00' },
      { weekday: 5, start: '16:00', end: '18:00' },
    ])
    expect(days[5]).toEqual({ mode: 'inside', start: '08:00', end: '10:00' })
  })

  it('keeps each weekday independent', () => {
    const days = windowsToDayStates([
      { weekday: 0, start: '00:00', end: '24:00' },
      { weekday: 6, start: '10:00', end: '14:00' },
    ])
    expect(days[0].mode).toBe('all_day')
    expect(days[6]).toEqual({ mode: 'inside', start: '10:00', end: '14:00' })
    for (const wd of [1, 2, 3, 4, 5]) expect(days[wd].mode).toBe('off')
  })
})

describe('dayStatesToWindows', () => {
  it('emits nothing for off days', () => {
    expect(dayStatesToWindows(allOff())).toEqual([])
  })

  it('emits a full-day window for all_day', () => {
    const days = allOff()
    days[2] = { mode: 'all_day', start: '09:00', end: '18:00' } // start/end unused in this mode
    expect(dayStatesToWindows(days)).toEqual([{ weekday: 2, start: '00:00', end: '24:00' }])
  })

  it('emits the bounds verbatim for inside', () => {
    const days = allOff()
    days[3] = { mode: 'inside', start: '09:00', end: '18:00' }
    expect(dayStatesToWindows(days)).toEqual([{ weekday: 3, start: '09:00', end: '18:00' }])
  })

  it('emits SWAPPED bounds for outside — a same-weekday wrap the backend splits, never a pre-split pair from this module', () => {
    const days = allOff()
    days[1] = { mode: 'outside', start: '09:00', end: '18:00' }
    expect(dayStatesToWindows(days)).toEqual([{ weekday: 1, start: '18:00', end: '09:00' }])
  })

  it('only emits windows for active days, in weekday order', () => {
    const days = allOff()
    days[5] = { mode: 'inside', start: '08:00', end: '12:00' }
    days[1] = { mode: 'all_day', start: '00:00', end: '24:00' }
    expect(dayStatesToWindows(days).map((w) => w.weekday)).toEqual([1, 5])
  })
})

describe('inside/all_day/off round-trip through windowsToDayStates(dayStatesToWindows(...))', () => {
  // "outside" is deliberately excluded here: dayStatesToWindows emits a raw
  // wrapping window for it, and only the backend's Normalize (not this
  // module) splits that into the canonical pair windowsToDayStates expects —
  // see the dedicated re-join test above for that half of the contract.
  it('off, all_day, and inside all survive a direct round trip unchanged', () => {
    const days = allOff()
    days[0] = { mode: 'all_day', start: '09:00', end: '18:00' }
    days[2] = { mode: 'inside', start: '10:30', end: '15:45' }
    // days[1], days[3..6] stay 'off'
    const roundTripped = windowsToDayStates(dayStatesToWindows(days))
    for (const wd of WEEKDAYS) expect(roundTripped[wd]).toEqual(days[wd])
  })
})

describe('coverageStrip', () => {
  it('is all-false for an all-off schedule', () => {
    const strip = coverageStrip(allOff())
    expect(strip).toHaveLength(7)
    for (const row of strip) expect(row).toEqual(Array(24).fill(false))
  })

  it('is all-true for all_day', () => {
    const days = allOff()
    days[2] = { mode: 'all_day', start: '09:00', end: '18:00' }
    expect(coverageStrip(days)[2]).toEqual(Array(24).fill(true))
  })

  it('inside 09:00-18:00 covers hours [9,18) exactly — half-open, so 18 itself is NOT covered', () => {
    const days = allOff()
    days[3] = { mode: 'inside', start: '09:00', end: '18:00' }
    const row = coverageStrip(days)[3]
    expect(row.slice(0, 9)).toEqual(Array(9).fill(false))
    expect(row.slice(9, 18)).toEqual(Array(9).fill(true))
    expect(row.slice(18)).toEqual(Array(6).fill(false))
  })

  it('outside 09:00-18:00 covers the complement — [0,9) and [18,24)', () => {
    const days = allOff()
    days[4] = { mode: 'outside', start: '09:00', end: '18:00' }
    const row = coverageStrip(days)[4]
    expect(row.slice(0, 9)).toEqual(Array(9).fill(true))
    expect(row.slice(9, 18)).toEqual(Array(9).fill(false))
    expect(row.slice(18)).toEqual(Array(6).fill(true))
  })

  it('rows are indexed in WEEKDAYS order (row 0 = Sunday)', () => {
    const days = allOff()
    days[0] = { mode: 'all_day', start: '09:00', end: '18:00' }
    const strip = coverageStrip(days)
    expect(strip[0]).toEqual(Array(24).fill(true))
    expect(strip[1]).toEqual(Array(24).fill(false))
  })
})

describe('formatDelaySeconds', () => {
  it('renders the zero case as "без задержки", not "0 сек"', () => {
    expect(formatDelaySeconds(0)).toBe('без задержки')
  })
  it('renders sub-minute delays in seconds', () => {
    expect(formatDelaySeconds(30)).toBe('30 сек')
    expect(formatDelaySeconds(59)).toBe('59 сек')
  })
  it('renders every DELAY_OPTIONS value used by the dialog', () => {
    expect(formatDelaySeconds(60)).toBe('1 мин')
    expect(formatDelaySeconds(120)).toBe('2 мин')
    expect(formatDelaySeconds(300)).toBe('5 мин')
    expect(formatDelaySeconds(600)).toBe('10 мин')
    expect(formatDelaySeconds(900)).toBe('15 мин')
    expect(formatDelaySeconds(1800)).toBe('30 мин')
  })
  it('renders whole hours', () => {
    expect(formatDelaySeconds(3600)).toBe('1 ч')
    expect(formatDelaySeconds(7200)).toBe('2 ч')
  })
  it('does not round a non-exact-multiple input — pins the current no-rounding behavior', () => {
    expect(formatDelaySeconds(90)).toBe('1.5 мин')
  })
})

describe('formatCooldownSeconds', () => {
  it('renders the zero case as "без ограничения" (a different message from formatDelaySeconds\' zero case)', () => {
    expect(formatCooldownSeconds(0)).toBe('без ограничения')
  })
  it('otherwise delegates to formatDelaySeconds verbatim', () => {
    expect(formatCooldownSeconds(300)).toBe(formatDelaySeconds(300))
    expect(formatCooldownSeconds(3600)).toBe(formatDelaySeconds(3600))
  })
})

describe('summarizeAutoResponse', () => {
  it('a disabled policy summarizes as "Выключен" regardless of any stored windows', () => {
    expect(summarizeAutoResponse(policy({ enabled: false, windows: [{ weekday: 1, start: '09:00', end: '18:00' }] }))).toBe('Выключен')
  })

  it('enabled with no windows flags the dead-schedule case explicitly', () => {
    expect(summarizeAutoResponse(policy({ windows: [] }))).toBe('Включён, расписание не задано')
  })

  it('every day, same all_day mode -> the "каждый день" shorthand', () => {
    const windows = WEEKDAYS.map((wd) => ({ weekday: wd, start: '00:00', end: '24:00' }))
    expect(summarizeAutoResponse(policy({ windows }))).toBe('ИИ · каждый день, круглосуточно')
  })

  it('Mon-Fri, same inside window -> day count + shared range', () => {
    const windows = [1, 2, 3, 4, 5].map((wd) => ({ weekday: wd, start: '09:00', end: '18:00' }))
    expect(summarizeAutoResponse(policy({ windows }))).toBe('ИИ · 5 дн., 09:00–18:00')
  })

  it('an outside-hours schedule labels itself with "вне"', () => {
    const windows = [
      { weekday: 1, start: '00:00', end: '09:00' },
      { weekday: 1, start: '18:00', end: '24:00' },
    ]
    expect(summarizeAutoResponse(policy({ windows }))).toBe('ИИ · 1 дн., вне 09:00–18:00')
  })

  it('mixed modes across active days drop the shared-range suffix entirely', () => {
    const windows = [
      { weekday: 1, start: '09:00', end: '18:00' },
      { weekday: 2, start: '00:00', end: '24:00' },
    ]
    expect(summarizeAutoResponse(policy({ windows }))).toBe('ИИ · 2 дн.')
  })

  it('fixed reply mode uses the "Фикс. текст" label instead of "ИИ"', () => {
    const windows = [{ weekday: 1, start: '09:00', end: '18:00' }]
    expect(summarizeAutoResponse(policy({ windows, reply_mode: 'fixed', fixed_text: 'brb' }))).toBe('Фикс. текст · 1 дн., 09:00–18:00')
  })
})
