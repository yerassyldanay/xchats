import { describe, expect, it } from 'vitest'
import { mountKb } from '@/test/mount'
import StatTiles from './StatTiles.vue'

describe('StatTiles', () => {
  it('renders the four counts with their labels', () => {
    const wrapper = mountKb(StatTiles, { props: { counts: { added: 3, updated: 2, removed: 1, total: 6 } } })
    const text = wrapper.text()
    expect(text).toContain('3')
    expect(text).toContain('Добавлено')
    expect(text).toContain('2')
    expect(text).toContain('Изменено')
    expect(text).toContain('1')
    expect(text).toContain('Удалено')
    expect(text).toContain('6')
    expect(text).toContain('Всего')
  })
})
