import { formatAmount, formatMoney, parseAmount } from './money'

describe('formatMoney', () => {
  it('formats whole rupees without decimals', () => {
    expect(formatMoney(842000)).toBe('Rs 8,420')
  })
  it('keeps paisa when present', () => {
    expect(formatMoney(842050)).toBe('Rs 8,420.50')
  })
  it('uses a real minus sign for negatives', () => {
    expect(formatMoney(-1500)).toBe('−Rs 15')
  })
  it('falls back to the currency code', () => {
    expect(formatMoney(1000, 'USD')).toBe('USD 10')
  })
})

describe('parseAmount', () => {
  it.each([
    ['8,420', 842000],
    ['8420.5', 842050],
    ['8420.05', 842005],
    ['Rs. 1,25,000', 12500000],
    ['  300 ', 30000],
    ['-50', -5000],
  ])('parses %s', (input, minor) => {
    expect(parseAmount(input)).toBe(minor)
  })
  it('returns null for empty input', () => {
    expect(parseAmount('  ')).toBeNull()
  })
  it('returns NaN for garbage and more than two decimals', () => {
    expect(parseAmount('abc')).toBeNaN()
    expect(parseAmount('1.234')).toBeNaN()
  })
  it('round-trips with formatAmount', () => {
    for (const v of [0, 5, 842000, 842050, 12345678]) {
      expect(parseAmount(formatAmount(v))).toBe(v)
    }
  })
})
