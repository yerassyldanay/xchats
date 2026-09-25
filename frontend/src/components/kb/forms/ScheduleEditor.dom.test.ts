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
