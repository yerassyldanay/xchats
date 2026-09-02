import { describe, expect, it } from 'vitest'
import { mountKb } from '@/test/mount'
import KbComparisonCard from './KbComparisonCard.vue'
import KbItemCard from './KbItemCard.vue'
import type { KbComparisonData, KbItemData } from '@/types'

const vitaminComparison: KbComparisonData = {
  kind: 'products',
  key: 'vitamin-d',
  title: 'Vitamin D',
  change: 'updated',
  real: {
    kind: 'products', key: 'vitamin-d', title: 'Vitamin D', source: 'REAL_KB',
    fields: [{ key: 'price', label: 'Price', value: '12 000 ₸' }],
  },
  draft: {
    kind: 'products', key: 'vitamin-d', title: 'Vitamin D', source: 'DRAFT_KB',
    fields: [{ key: 'price', label: 'Price', value: '10 800 ₸' }],
  },
  fields: [{ key: 'price', label: 'Price', real: '12 000 ₸', draft: '10 800 ₸' }],
}

describe('KbComparisonCard', () => {
  // The rule the whole feature rests on: an operator must never be able to
  // read a pending value as the live one. Both values appear, and both
  // columns are labelled with the state they belong to.
  it('shows both values under explicit REAL and DRAFT labels', () => {
    const wrapper = mountKb(KbComparisonCard, { props: { data: vitaminComparison } })
    const text = wrapper.text()
    expect(text).toContain('12 000 ₸')
    expect(text).toContain('10 800 ₸')
    // Russian is the default locale (see i18n/index.ts).
    expect(text).toContain('Текущая')
    expect(text).toContain('Черновик')
  })

  it('states the arithmetic difference', () => {
    const wrapper = mountKb(KbComparisonCard, { props: { data: vitaminComparison } })
    expect(wrapper.text()).toContain('−1 200 ₸')
  })

  it('shows no difference for a non-numeric change rather than inventing one', () => {
    const wrapper = mountKb(KbComparisonCard, {
      props: {
        data: {
          ...vitaminComparison,
          fields: [{ key: 'warranty', label: 'Warranty', real: '12 месяцев', draft: 'по договорённости' }],
        } satisfies KbComparisonData,
      },
    })
    const text = wrapper.text()
    expect(text).toContain('12 месяцев')
    expect(text).toContain('по договорённости')
    expect(text).not.toMatch(/[+−]\d/)
  })

  it('labels an addition, and marks the absent live side as unset', () => {
    const wrapper = mountKb(KbComparisonCard, {
      props: {
        data: {
          ...vitaminComparison,
          change: 'added',
          real: null,
          fields: [{ key: 'price', label: 'Price', real: '', draft: '6 500 ₸' }],
        } satisfies KbComparisonData,
      },
    })
    expect(wrapper.text()).toContain('Добавлено')
    expect(wrapper.text()).toContain('не задано')
  })
})

describe('KbItemCard', () => {
  it('carries the source of the record it shows', () => {
    const data: KbItemData = {
      record: {
        kind: 'products', key: 'omega-3', title: 'Omega 3', source: 'REAL_KB',
        fields: [{ key: 'price', label: 'Price', value: '8 000 ₸' }],
      },
    }
    const wrapper = mountKb(KbItemCard, { props: { data } })
    expect(wrapper.text()).toContain('Omega 3')
    expect(wrapper.text()).toContain('8 000 ₸')
    expect(wrapper.text()).toContain('Текущая')
    expect(wrapper.text()).not.toContain('Черновик')
  })
})
