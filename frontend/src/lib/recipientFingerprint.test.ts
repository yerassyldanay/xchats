import { describe, expect, it } from 'vitest'
import { fingerprintOf } from './recipientFingerprint'

function fileOf(name: string, size: number, lastModified: number): File {
  const f = new File(['x'], name, { lastModified })
  Object.defineProperty(f, 'size', { value: size, configurable: true })
  return f
}

// CAM-09: the fingerprint is what Create/Save's staleness check is bound to —
// it must change on every meaningfully different input, and stay stable
// when nothing about the submitted recipient source actually changed.
describe('fingerprintOf', () => {
  it('changes when the pasted text changes', () => {
    expect(fingerprintOf('a', null)).not.toBe(fingerprintOf('b', null))
  })

  it('is stable for the identical text, called twice', () => {
    expect(fingerprintOf('same text', null)).toBe(fingerprintOf('same text', null))
  })

  it('changes when the file is replaced with a different one', () => {
    const f1 = fileOf('a.csv', 100, 1000)
    const f2 = fileOf('b.csv', 100, 1000)
    expect(fingerprintOf('', f1)).not.toBe(fingerprintOf('', f2))
  })

  it('changes when a same-named file is replaced (different size/lastModified)', () => {
    const f1 = fileOf('recipients.csv', 100, 1000)
    const f2 = fileOf('recipients.csv', 200, 2000)
    expect(fingerprintOf('', f1)).not.toBe(fingerprintOf('', f2))
  })

  it('changes when a file is cleared back to pasted text', () => {
    const f = fileOf('recipients.csv', 100, 1000)
    expect(fingerprintOf('some pasted text', f)).not.toBe(fingerprintOf('some pasted text', null))
  })

  it('a file always takes precedence over any pasted text in the fingerprint', () => {
    const f = fileOf('recipients.csv', 100, 1000)
    // Two different pasted-text values, same file: must fingerprint IDENTICALLY,
    // matching campaignRecipientsRequest's own file-over-text precedence.
    expect(fingerprintOf('text A', f)).toBe(fingerprintOf('text B', f))
  })
})
