/** Money is integer minor units (paisa) everywhere — docs/adr/001. */

/** Non-breaking space, so "Rs" never wraps away from its amount. */
const NBSP = String.fromCharCode(0xa0)

const numberFormat = new Intl.NumberFormat('en-PK', { maximumFractionDigits: 2, minimumFractionDigits: 0 })
const fixed2 = new Intl.NumberFormat('en-PK', { minimumFractionDigits: 2, maximumFractionDigits: 2 })

/** Display prefix for a currency code ("PKR" → "Rs"). */
export function currencySymbol(currency: string): string {
  return currency === 'PKR' ? 'Rs' : currency
}

/** 842000 → "Rs 8,420"; 842050 → "Rs 8,420.50"; −1500 → "−Rs 15". */
export function formatMoney(minor: number, currency = 'PKR'): string {
  const negative = minor < 0
  const abs = Math.abs(minor)
  const major = abs / 100
  const body = abs % 100 === 0 ? numberFormat.format(major) : fixed2.format(major)
  return `${negative ? '−' : ''}${currencySymbol(currency)}${NBSP}${body}`
}

/** Just the number part, for inputs: 842050 → "8,420.50". */
export function formatAmount(minor: number): string {
  const abs = Math.abs(minor)
  const s = abs % 100 === 0 ? numberFormat.format(abs / 100) : fixed2.format(abs / 100)
  return minor < 0 ? `-${s}` : s
}

/**
 * Parses user input to minor units without floating-point maths.
 * "8,420" → 842000, "8420.5" → 842050, "" → null, "abc" → NaN.
 */
export function parseAmount(input: string): number | null {
  const s = input.replace(/[,\s]/g, '').replace(/^rs\.?/i, '')
  if (s === '') return null
  const m = /^(-)?(\d+)(?:\.(\d{0,2}))?$/.exec(s)
  if (!m) return Number.NaN
  const [, sign, whole = '0', frac = ''] = m
  const minor = Number(whole) * 100 + Number(frac.padEnd(2, '0'))
  return sign ? -minor : minor
}
