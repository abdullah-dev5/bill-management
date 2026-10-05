import { describe, expect, it } from 'vitest'
import { periodRange } from './data-section'

describe('periodRange', () => {
  it('covers calendar years and the last 12 months', () => {
    expect(periodRange('thisYear', '2026-10-04')).toEqual({ from: '2026-01-01', to: '2026-12-31' })
    expect(periodRange('lastYear', '2026-10-04')).toEqual({ from: '2025-01-01', to: '2025-12-31' })
    expect(periodRange('12m', '2026-10-04')).toEqual({ from: '2025-11-01', to: '2026-10-04' })
    expect(periodRange('12m', '2026-01-15')).toEqual({ from: '2025-02-01', to: '2026-01-15' })
    expect(periodRange('all', '2026-10-04')).toEqual({})
    expect(periodRange('custom', '2026-10-04', { from: '2026-03-01' })).toEqual({ from: '2026-03-01' })
  })
})
