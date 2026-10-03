import { describe, expect, it } from 'vitest'
import { formatFieldValue, orderedFields } from './extraction-panel'

describe('formatFieldValue', () => {
  it('formats money, months, dates and plain values', () => {
    expect(formatFieldValue('payableMinor', '326600')).toBe('Rs 3,266')
    expect(formatFieldValue('billingMonth', '2026-09')).toBe('September 2026')
    expect(formatFieldValue('dueDate', '2026-10-05')).toBe('05 Oct 2026')
    expect(formatFieldValue('units', '164')).toBe('164')
  })
})

describe('orderedFields', () => {
  it('puts known fields first in a fixed order', () => {
    const f = (value: string) => ({ value, confidence: 0.9, source: 'x' })
    const out = orderedFields({ units: f('1'), extra: f('2'), payableMinor: f('3'), providerName: f('4') })
    expect(out.map(([k]) => k)).toEqual(['providerName', 'payableMinor', 'units', 'extra'])
  })
})
