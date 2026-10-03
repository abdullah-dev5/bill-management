import { describe, expect, it } from 'vitest'
import { parseNumber, unitsFromReadings } from './bill-form'

describe('parseNumber', () => {
  it('reads readings and unit counts', () => {
    expect(parseNumber('18,854')).toBe(18854)
    expect(parseNumber(' 1.5 ')).toBe(1.5)
    expect(parseNumber('')).toBeNull()
    expect(parseNumber('abc')).toBeNaN()
    expect(parseNumber('-3')).toBeNaN()
  })
})

describe('unitsFromReadings', () => {
  it('multiplies the difference by the factor', () => {
    expect(unitsFromReadings(18690, 18854, 1)).toBe(164)
    expect(unitsFromReadings(100, 140, 2.5)).toBe(100)
    expect(unitsFromReadings(100, 140, null)).toBe(40)
  })
  it('gives nothing when readings are missing or go down', () => {
    expect(unitsFromReadings(null, 140, 1)).toBeNull()
    expect(unitsFromReadings(200, 140, 1)).toBeNull()
  })
})
