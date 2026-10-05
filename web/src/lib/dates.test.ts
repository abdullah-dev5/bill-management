import { describe, expect, it } from 'vitest'
import { parseUserDate, parseUserMonth } from './dates'

describe('parseUserDate', () => {
  it.each([
    ['2026-10-08', '2026-10-08'],
    ['8/10/2026', '2026-10-08'],
    ['08-10-26', '2026-10-08'],
    ['8 Oct 2026', '2026-10-08'],
    ['08-Oct-26', '2026-10-08'],
    ['11-Oct-2024', '2024-10-11'],
    ['October 8, 2026', '2026-10-08'],
  ])('%s → %s', (text, want) => expect(parseUserDate(text)).toBe(want))

  it.each(['', 'soon', '31/02/2026', '2026-13-01', '8 Foo 2026'])('rejects %j', (text) => expect(parseUserDate(text)).toBeNull())
})

describe('parseUserMonth', () => {
  it.each([
    ['2026-09', '2026-09'],
    ['9/2026', '2026-09'],
    ['Sep 2026', '2026-09'],
    ['September 2026', '2026-09'],
    ['Sep-24', '2024-09'],
  ])('%s → %s', (text, want) => expect(parseUserMonth(text)).toBe(want))

  it.each(['', '2026-13', 'Foo 2026'])('rejects %j', (text) => expect(parseUserMonth(text)).toBeNull())
})
