import { describe, expect, it } from 'vitest'
import type { Extraction } from '@/api/client'
import { normalizeRef, reviewFromExtraction } from './scan-page'

const f = (value: string, confidence = 0.95) => ({ value, confidence, source: 'pdf-text+pitc' })

const extraction = (fields: Extraction['fields'], extra: Partial<Extraction> = {}): Extraction => ({
  id: 'e1', documentId: 'd1', status: 'succeeded', tiers: [], fields, lineItems: [], lateFees: [], history: [], warnings: [],
  createdAt: '2026-10-03T10:00:00Z', ...extra,
})

describe('reviewFromExtraction', () => {
  it('maps fields to form values with their confidence', () => {
    const r = reviewFromExtraction(
      extraction(
        {
          billingMonth: f('2026-09'), issueDate: f('2026-09-18'), dueDate: f('2026-10-05', 0.7),
          currentChargesMinor: f('326600'), arrearsMinor: f('0'), payableMinor: f('326600'), payableAfterDueMinor: f('338900'),
        },
        { lateFees: [{ until: '2026-10-08', amountMinor: 338900 }, { amountMinor: 351200 }] },
      ),
    )
    expect(r.values).toMatchObject({
      periodKey: '2026-09', issueDate: '2026-09-18', expenseDate: '2026-09-18', dueDate: '2026-10-05',
      currentChargesMinor: 326600, arrearsMinor: 0, payableAfterDueMinor: 338900, lateStepUntil: '2026-10-08', payableFinalMinor: 351200,
    })
    // The payable equals charges + arrears, so the form computes it instead.
    expect(r.values.payableMinor).toBeUndefined()
    expect(r.confidence.dueDate).toBe(0.7)
  })

  it('keeps a printed payable that differs from the sum', () => {
    const r = reviewFromExtraction(extraction({ currentChargesMinor: f('100000'), payableMinor: f('95000') }))
    expect(r.values.payableMinor).toBe(95000)
  })

  it('returns nothing for failed or missing extractions', () => {
    expect(reviewFromExtraction(null).values).toEqual({})
    expect(reviewFromExtraction(extraction({ dueDate: f('2026-10-05') }, { status: 'failed' })).values).toEqual({})
  })
})

describe('normalizeRef', () => {
  it('compares reference numbers ignoring spaces and dashes', () => {
    expect(normalizeRef('09 12345 0012345')).toBe(normalizeRef('09123450012345'))
    expect(normalizeRef('nt-448812')).toBe('NT448812')
    expect(normalizeRef(null)).toBe('')
  })
})
