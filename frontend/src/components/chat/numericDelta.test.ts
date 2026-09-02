import { describe, expect, it } from 'vitest'
import { numericDelta } from './numericDelta'

describe('numericDelta — comparable quantities', () => {
  it('computes a price drop with its unit, in the spec\'s own example', () => {
    expect(numericDelta('12 000 KZT', '10 800 KZT')).toEqual({ label: '−1 200 KZT', increased: false })
  })

  it('computes an increase', () => {
    expect(numericDelta('8 000 ₸', '9 500 ₸')).toEqual({ label: '+1 500 ₸', increased: true })
  })

  it('handles a bare number with no unit', () => {
    expect(numericDelta('120', '90')).toEqual({ label: '−30', increased: false })
  })

  it('reads a comma followed by three digits as grouping', () => {
    expect(numericDelta('12,000 KZT', '10,800 KZT')?.label).toBe('−1 200 KZT')
  })

  it('reads a comma followed by fewer digits as a decimal point', () => {
    expect(numericDelta('12,5 %', '13,5 %')).toEqual({ label: '+1 %', increased: true })
  })

  it('handles a decimal point', () => {
    expect(numericDelta('1.50 USD', '2.25 USD')).toEqual({ label: '+0,75 USD', increased: true })
  })

  it('handles a non-breaking space as a group separator', () => {
    expect(numericDelta('12 000 ₸', '11 000 ₸')?.label).toBe('−1 000 ₸')
  })
})

describe('numericDelta — everything it must refuse to guess at', () => {
  it('returns null when the values are equal', () => {
    expect(numericDelta('12 000 ₸', '12 000 ₸')).toBeNull()
  })

  it('returns null when the units differ — 12 dollars is not 12 tenge', () => {
    expect(numericDelta('12 USD', '10 KZT')).toBeNull()
  })

  it('returns null for free text', () => {
    expect(numericDelta('по договорённости', 'бесплатно')).toBeNull()
  })

  it('returns null when a side is missing entirely', () => {
    expect(numericDelta('', '10 800 KZT')).toBeNull()
    expect(numericDelta('12 000 KZT', '')).toBeNull()
  })

  it('returns null for a range — two numbers are not one quantity', () => {
    expect(numericDelta('2-3 дня', '1-2 дня')).toBeNull()
  })

  it('returns null when a digit is part of a word', () => {
    expect(numericDelta('Vitamin D3', 'Vitamin D4')).toBeNull()
  })

  it('returns null for a number it cannot read unambiguously', () => {
    expect(numericDelta('1.2.3 ₸', '1.2.4 ₸')).toBeNull()
  })

  it('returns null when only one side is numeric', () => {
    expect(numericDelta('12 000 ₸', 'по запросу')).toBeNull()
  })
})
