import { describe, it, expect } from 'vitest'
import {
  currentUtcOffsetMinutes,
  formatUtcOffset,
  utcToLocal,
  localToUtc,
  minutesToHHMM,
  hhmmToMinutes,
  endMinutesFromInput,
  endInputFromMinutes,
  alwaysWindows,
  nightsWindows,
  weekendsWindows,
  detectPreset,
  type ScheduleWindow,
} from './schedule'

describe('formatUtcOffset', () => {
  it('formats a positive whole-hour offset', () => {
    expect(formatUtcOffset(300)).toBe('UTC+5')
  })
  it('formats a negative whole-hour offset', () => {
    expect(formatUtcOffset(-180)).toBe('UTC-3')
  })
  it('formats zero', () => {
    expect(formatUtcOffset(0)).toBe('UTC+0')
  })
  it('formats a positive half-hour offset', () => {
    expect(formatUtcOffset(330)).toBe('UTC+5:30') // India
  })
  it('formats a negative non-hour offset', () => {
    expect(formatUtcOffset(-210)).toBe('UTC-3:30') // Newfoundland
  })
})

describe('currentUtcOffsetMinutes', () => {
  it('is the negation of Date.getTimezoneOffset()', () => {
    const d = new Date('2026-08-06T12:00:00Z')
    expect(currentUtcOffsetMinutes(d)).toBe(-d.getTimezoneOffset())
  })
})

describe('localToUtc / utcToLocal round trip', () => {
  it('is a no-op at offset 0', () => {
    const w: ScheduleWindow[] = [{ weekday: 1, start_minute: 540, end_minute: 1020 }]
    expect(localToUtc(w, 0)).toEqual(w)
    expect(utcToLocal(w, 0)).toEqual(w)
  })

  it('shifts a same-day window forward for a positive offset without crossing midnight', () => {
    // Local Monday 09:00-17:00 at UTC+2 -> UTC Monday 07:00-15:00.
    const local: ScheduleWindow[] = [{ weekday: 1, start_minute: 9 * 60, end_minute: 17 * 60 }]
    const utc = localToUtc(local, 120)
    expect(utc).toEqual([{ weekday: 1, start_minute: 7 * 60, end_minute: 15 * 60 }])
    // And converting back reproduces the original local window exactly.
    expect(utcToLocal(utc, 120)).toEqual(local)
  })

  it('shifts a same-day window backward for a negative offset without crossing midnight', () => {
    // Local Monday 09:00-17:00 at UTC-3 -> UTC Monday 12:00-20:00.
    const local: ScheduleWindow[] = [{ weekday: 1, start_minute: 9 * 60, end_minute: 17 * 60 }]
    const utc = localToUtc(local, -180)
    expect(utc).toEqual([{ weekday: 1, start_minute: 12 * 60, end_minute: 20 * 60 }])
    expect(utcToLocal(utc, -180)).toEqual(local)
  })

  it('crosses a weekday boundary forward when the local offset pushes the window past midnight', () => {
    // Local Monday 23:00-23:30 at UTC+5 -> UTC Monday 18:00-18:30 (offset
    // large enough that converting local->UTC moves it EARLIER in the day,
    // not across a boundary here — pick a case that DOES cross: local
    // Monday 22:00-23:30 at UTC-3 -> UTC Tuesday 01:00-02:30).
    const local: ScheduleWindow[] = [{ weekday: 1, start_minute: 22 * 60, end_minute: 23 * 60 + 30 }]
    const utc = localToUtc(local, -180)
    expect(utc).toEqual([{ weekday: 2, start_minute: 1 * 60, end_minute: 2 * 60 + 30 }])
    expect(utcToLocal(utc, -180)).toEqual(local)
  })

  it('crosses a weekday boundary backward (into the PREVIOUS day) for a positive offset', () => {
    // Local Monday 00:00-01:00 at UTC+5 -> UTC Sunday 19:00-20:00.
    const local: ScheduleWindow[] = [{ weekday: 1, start_minute: 0, end_minute: 60 }]
    const utc = localToUtc(local, 300)
    expect(utc).toEqual([{ weekday: 0, start_minute: 19 * 60, end_minute: 20 * 60 }])
    expect(utcToLocal(utc, 300)).toEqual(local)
  })

  it('wraps the week boundary (Saturday local -> Sunday UTC and back)', () => {
    // Local Saturday 23:30-23:59 at UTC-2 -> UTC Sunday 01:30-01:59.
    const local: ScheduleWindow[] = [{ weekday: 6, start_minute: 23 * 60 + 30, end_minute: 23 * 60 + 59 }]
    const utc = localToUtc(local, -120)
    expect(utc).toEqual([{ weekday: 0, start_minute: 1 * 60 + 30, end_minute: 1 * 60 + 59 }])
    expect(utcToLocal(utc, -120)).toEqual(local)
  })

  it('preserves an already-overnight window (does not "fix" the wrap)', () => {
    // Local every-day 22:00-06:00 (Nights) at UTC+5 -> UTC 17:00-01:00 (still wraps).
    const local: ScheduleWindow[] = [{ weekday: 3, start_minute: 22 * 60, end_minute: 6 * 60 }]
    const utc = localToUtc(local, 300)
    expect(utc).toEqual([{ weekday: 3, start_minute: 17 * 60, end_minute: 1 * 60 }])
    expect(utcToLocal(utc, 300)).toEqual(local)
  })

  it('converts multiple independent windows in one call (non-contiguous periods)', () => {
    const local: ScheduleWindow[] = [
      { weekday: 1, start_minute: 9 * 60, end_minute: 12 * 60 },
      { weekday: 1, start_minute: 13 * 60, end_minute: 18 * 60 },
      { weekday: 5, start_minute: 20 * 60, end_minute: 23 * 60 },
    ]
    const utc = localToUtc(local, 60)
    expect(utc).toEqual([
      { weekday: 1, start_minute: 8 * 60, end_minute: 11 * 60 },
      { weekday: 1, start_minute: 12 * 60, end_minute: 17 * 60 },
      { weekday: 5, start_minute: 19 * 60, end_minute: 22 * 60 },
    ])
    expect(utcToLocal(utc, 60)).toEqual(local)
  })

  it('splits a full-day window into two UTC rows when the offset does not land on midnight', () => {
    // A single full local day (Monday 00:00-24:00) at UTC+5 cannot be
    // expressed as one UTC row starting at 19:00 the prior day without
    // start==end, so it must become two contiguous rows.
    const local: ScheduleWindow[] = [{ weekday: 1, start_minute: 0, end_minute: 1440 }]
    const utc = localToUtc(local, 300)
    expect(utc).toEqual([
      { weekday: 0, start_minute: 19 * 60, end_minute: 1440 }, // Sunday 19:00-24:00
      { weekday: 1, start_minute: 0, end_minute: 19 * 60 }, // Monday 00:00-19:00
    ])
    // Converting back splits differently but still covers exactly Monday:
    // Sunday-UTC 19:00-24:00 + 5h = Monday-local 00:00-05:00, and
    // Monday-UTC 00:00-19:00 + 5h = Monday-local 05:00-24:00.
    const backToLocal = utcToLocal(utc, 300)
    expect(backToLocal).toEqual([
      { weekday: 1, start_minute: 0, end_minute: 5 * 60 },
      { weekday: 1, start_minute: 5 * 60, end_minute: 1440 },
    ])
  })

  it('does not split when the offset is a whole-day multiple (rare, but should stay 1:1)', () => {
    // Shifting Monday back by exactly one full day (-1440 minutes) lands
    // squarely on Sunday's midnight boundary, so no split is needed.
    const local: ScheduleWindow[] = [{ weekday: 1, start_minute: 0, end_minute: 1440 }]
    expect(localToUtc(local, 1440)).toEqual([{ weekday: 0, start_minute: 0, end_minute: 1440 }])
  })
})

describe('alwaysWindows', () => {
  it('covers all seven days as full 24h windows', () => {
    const w = alwaysWindows()
    expect(w).toHaveLength(7)
    for (let d = 0; d < 7; d++) {
      expect(w).toContainEqual({ weekday: d, start_minute: 0, end_minute: 1440 })
    }
  })
})

describe('nightsWindows', () => {
  it('defaults to 22:00-06:00 every day', () => {
    const w = nightsWindows()
    expect(w).toHaveLength(7)
    expect(w[0]).toEqual({ weekday: 0, start_minute: 22 * 60, end_minute: 6 * 60 })
  })
  it('accepts custom hours', () => {
    const w = nightsWindows(23, 7)
    expect(w[2]).toEqual({ weekday: 2, start_minute: 23 * 60, end_minute: 7 * 60 })
  })
})

describe('weekendsWindows', () => {
  it('covers Saturday and Sunday, all day', () => {
    const w = weekendsWindows()
    expect(w).toEqual([
      { weekday: 6, start_minute: 0, end_minute: 1440 },
      { weekday: 0, start_minute: 0, end_minute: 1440 },
    ])
  })
})

describe('detectPreset', () => {
  it('recognizes Always regardless of array order', () => {
    const shuffled = [...alwaysWindows()].reverse()
    expect(detectPreset(shuffled)).toBe('always')
  })
  it('recognizes Nights', () => {
    expect(detectPreset(nightsWindows())).toBe('nights')
  })
  it('recognizes Weekends', () => {
    expect(detectPreset(weekendsWindows())).toBe('weekends')
  })
  it('falls back to custom for anything else', () => {
    expect(detectPreset([{ weekday: 1, start_minute: 540, end_minute: 1020 }])).toBe('custom')
  })
  it('falls back to custom for an empty schedule', () => {
    expect(detectPreset([])).toBe('custom')
  })
  it('does not mistake a near-miss (one extra window) for Nights', () => {
    expect(detectPreset([...nightsWindows(), { weekday: 1, start_minute: 0, end_minute: 60 }])).toBe('custom')
  })
})

describe('HH:MM <-> minutes helpers', () => {
  it('round-trips ordinary times', () => {
    expect(minutesToHHMM(hhmmToMinutes('09:05'))).toBe('09:05')
    expect(minutesToHHMM(0)).toBe('00:00')
    expect(minutesToHHMM(1439)).toBe('23:59')
  })
  it('endMinutesFromInput treats "00:00" as end-of-day (1440), never 0', () => {
    expect(endMinutesFromInput('00:00')).toBe(1440)
    expect(endMinutesFromInput('06:00')).toBe(360)
  })
  it('endInputFromMinutes is the inverse', () => {
    expect(endInputFromMinutes(1440)).toBe('00:00')
    expect(endInputFromMinutes(360)).toBe('06:00')
  })
})
