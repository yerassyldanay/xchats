import { describe, expect, it } from 'vitest'
import { mountKb } from '@/test/mount'
import ScheduleEditor from './ScheduleEditor.vue'
import type { Schedule } from '@/types'

function mountEditor(modelValue: Schedule = []) {
  return mountKb(ScheduleEditor, { props: { modelValue } })
}

// lastEmitted reads the most recent update:modelValue payload — every
// interaction below fires at least one, since ScheduleEditor is a pure
// controlled component (no internal buffer, see its own doc comment).
function lastEmitted(wrapper: ReturnType<typeof mountEditor>): Schedule {
  const events = wrapper.emitted('update:modelValue')
  expect(events).toBeTruthy()
  return events![events!.length - 1][0] as Schedule
}

describe('ScheduleEditor — toggling a weekday', () => {
  it('checking an off day adds it with default hours and no breaks, keyed by ref', async () => {
    const wrapper = mountEditor([])
    const checkbox = wrapper.find('[data-testid="schedule-worked-tue"]')
    expect(checkbox.exists()).toBe(true)

    await checkbox.setValue(true)

    const schedule = lastEmitted(wrapper)
    expect(schedule).toHaveLength(1)
    expect(schedule[0].ref).toBe('tue')
    expect(schedule[0].day).toBe('Вторник') // localized display label, captured at toggle time
    expect(schedule[0].start).toMatch(/^\d{2}:\d{2}$/)
    expect(schedule[0].end).toMatch(/^\d{2}:\d{2}$/)
    expect(schedule[0].breaks).toEqual([])
  })

  it('unchecking a worked day removes its entry entirely — never blanks its times', async () => {
    const seeded: Schedule = [{ ref: 'tue', day: 'Вторник', start: '10:00', end: '19:00', breaks: [] }]
    const wrapper = mountEditor(seeded)

    await wrapper.find('[data-testid="schedule-worked-tue"]').setValue(false)

    const schedule = lastEmitted(wrapper)
    expect(schedule).toEqual([])
  })

  it('emits days back in Monday-first order regardless of toggle order', async () => {
    const wrapper = mountEditor([])
    await wrapper.find('[data-testid="schedule-worked-fri"]').setValue(true)
    await wrapper.setProps({ modelValue: lastEmitted(wrapper) })
    await wrapper.find('[data-testid="schedule-worked-mon"]').setValue(true)

    const schedule = lastEmitted(wrapper)
    expect(schedule.map((d) => d.ref)).toEqual(['mon', 'fri'])
  })
})

describe('ScheduleEditor — copy-hours shortcuts', () => {
  it('applying to weekdays copies Monday\'s hours to Tue–Fri, worked or not, leaving Sat/Sun untouched', async () => {
    const seeded: Schedule = [
      { ref: 'mon', day: 'Понедельник', start: '11:00', end: '20:00', breaks: [] },
      { ref: 'sat', day: 'Суббота', start: '09:00', end: '15:00', breaks: [] },
    ]
    const wrapper = mountEditor(seeded)

    await wrapper.find('[data-testid="schedule-apply-weekdays"]').trigger('click')

    const schedule = lastEmitted(wrapper)
    expect(schedule.map((d) => d.ref)).toEqual(['mon', 'tue', 'wed', 'thu', 'fri', 'sat'])
    for (const ref of ['mon', 'tue', 'wed', 'thu', 'fri']) {
      const day = schedule.find((d) => d.ref === ref)!
      expect(day.start).toBe('11:00')
      expect(day.end).toBe('20:00')
    }
    // Saturday was never a target — its own hours survive unchanged.
    const sat = schedule.find((d) => d.ref === 'sat')!
    expect(sat.start).toBe('09:00')
    expect(sat.end).toBe('15:00')
  })

  it('applying to all days copies Monday\'s hours to every day of the week', async () => {
    const seeded: Schedule = [{ ref: 'mon', day: 'Понедельник', start: '08:00', end: '17:00', breaks: [] }]
    const wrapper = mountEditor(seeded)

    await wrapper.find('[data-testid="schedule-apply-all-days"]').trigger('click')

    const schedule = lastEmitted(wrapper)
    expect(schedule).toHaveLength(7)
    for (const day of schedule) {
      expect(day.start).toBe('08:00')
      expect(day.end).toBe('17:00')
    }
  })

  it('falls back to the default hours when Monday is not worked yet', async () => {
    const wrapper = mountEditor([])

    await wrapper.find('[data-testid="schedule-apply-all-days"]').trigger('click')

    const schedule = lastEmitted(wrapper)
    expect(schedule).toHaveLength(7)
    expect(schedule[0].start).toMatch(/^\d{2}:\d{2}$/)
    expect(new Set(schedule.map((d) => d.start)).size).toBe(1) // every day got the same source hours
  })

  it('preserves an existing target day\'s own breaks — only hours are synced', async () => {
    const seeded: Schedule = [
      { ref: 'mon', day: 'Понедельник', start: '10:00', end: '19:00', breaks: [] },
      { ref: 'wed', day: 'Среда', start: '09:00', end: '18:00', breaks: [{ start: '13:00', end: '14:00' }] },
    ]
    const wrapper = mountEditor(seeded)

    await wrapper.find('[data-testid="schedule-apply-weekdays"]').trigger('click')

    const schedule = lastEmitted(wrapper)
    const wed = schedule.find((d) => d.ref === 'wed')!
    expect(wed.start).toBe('10:00')
    expect(wed.end).toBe('19:00')
    expect(wed.breaks).toEqual([{ start: '13:00', end: '14:00' }])
  })
})

describe('ScheduleEditor — breaks', () => {
  it('adding a break seeds it from the shift hours', async () => {
    const seeded: Schedule = [{ ref: 'wed', day: 'Среда', start: '10:00', end: '19:00', breaks: [] }]
    const wrapper = mountEditor(seeded)

    await wrapper.find('[data-testid="schedule-break-add-wed"]').trigger('click')

    const schedule = lastEmitted(wrapper)
    expect(schedule[0].breaks).toEqual([{ start: '10:00', end: '19:00' }])
  })

  it('editing a break input updates only that break, leaving the shift and other fields untouched', async () => {
    const seeded: Schedule = [
      { ref: 'thu', day: 'Четверг', start: '09:00', end: '18:00', breaks: [{ start: '13:00', end: '14:00' }] },
    ]
    const wrapper = mountEditor(seeded)

    await wrapper.find('[data-testid="schedule-break-start-thu-0"]').setValue('13:30')

    const schedule = lastEmitted(wrapper)
    expect(schedule[0].start).toBe('09:00')
    expect(schedule[0].end).toBe('18:00')
    expect(schedule[0].breaks).toEqual([{ start: '13:30', end: '14:00' }])
  })

  it('removing a break clears it from the list without touching the shift', async () => {
    const seeded: Schedule = [
      { ref: 'fri', day: 'Пятница', start: '10:00', end: '20:00', breaks: [{ start: '14:00', end: '15:00' }] },
    ]
    const wrapper = mountEditor(seeded)

    await wrapper.find('[data-testid="schedule-break-remove-fri-0"]').trigger('click')

    const schedule = lastEmitted(wrapper)
    expect(schedule[0].breaks).toEqual([])
    expect(schedule[0].start).toBe('10:00')
    expect(schedule[0].end).toBe('20:00')
  })

  it('shows a soft warning when a break overlaps the shift boundary, without blocking anything', async () => {
    const seeded: Schedule = [
      { ref: 'sat', day: 'Суббота', start: '10:00', end: '18:00', breaks: [{ start: '17:30', end: '18:30' }] },
    ]
    const wrapper = mountEditor(seeded)
    expect(wrapper.find('[data-testid="schedule-warning-sat"]').exists()).toBe(true)
  })
})
