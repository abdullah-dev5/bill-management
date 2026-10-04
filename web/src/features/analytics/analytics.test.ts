import { describe, expect, it } from 'vitest'
import { change, presetRange } from './analytics-page'
import { niceScale } from './charts'

describe('niceScale', () => {
  it('rounds the top to a clean step', () => {
    expect(niceScale(5011600)).toEqual({ top: 8000000, step: 2000000 })
    expect(niceScale(950)).toEqual({ top: 1000, step: 250 })
    expect(niceScale(0)).toEqual({ top: 4, step: 1 })
  })
})

describe('presetRange', () => {
  it('ends every preset with the current month', () => {
    expect(presetRange('month', '2026-10')).toEqual({ from: '2026-10', to: '2026-10' })
    expect(presetRange('3m', '2026-02')).toEqual({ from: '2025-12', to: '2026-02' })
    expect(presetRange('year', '2026-10')).toEqual({ from: '2026-01', to: '2026-10' })
    expect(presetRange('12m', '2026-10')).toEqual({ from: '2025-11', to: '2026-10' })
  })
})

describe('change', () => {
  it('gives the difference and the percentage', () => {
    expect(change(1200, 1000)).toEqual({ diff: 200, pct: 20 })
    expect(change(800, 1000)).toEqual({ diff: -200, pct: -20 })
    expect(change(500, 0)).toEqual({ diff: 500, pct: null })
  })
})
