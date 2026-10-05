/** Dates from the API are calendar dates ("2026-10-08"), never instants. Parse them as local dates. */

export function parseDate(iso: string): Date {
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(y ?? 1970, (m ?? 1) - 1, d ?? 1)
}

export function toISODate(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

export function todayISO(): string {
  return toISODate(new Date())
}

export function currentMonthKey(): string {
  return todayISO().slice(0, 7)
}

// Fixed short names: newer browser locale data writes September as "Sept", which bills never use.
const SHORT_MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
const monthYear = new Intl.DateTimeFormat('en-GB', { month: 'long', year: 'numeric' })
const pad = (n: number) => String(n).padStart(2, '0')

/** "08 Oct" */
export const formatDayMonth = (iso: string) => {
  const d = parseDate(iso)
  return `${pad(d.getDate())} ${SHORT_MONTHS[d.getMonth()]}`
}
/** "08 Oct 2026" */
export const formatDate = (iso: string) => `${formatDayMonth(iso)} ${parseDate(iso).getFullYear()}`
/** "2026-09" → "September 2026" */
export const formatMonth = (key: string) => monthYear.format(parseDate(key + '-01'))
/** "2026-09" → "Sep 2026" */
export const formatMonthShort = (key: string) => {
  const d = parseDate(key + '-01')
  return `${SHORT_MONTHS[d.getMonth()]} ${d.getFullYear()}`
}
/** Short month name for a 0-based month index. */
export const shortMonthName = (index: number) => SHORT_MONTHS[index] ?? ''

/** Whole days from today until iso (negative = past). */
export function daysUntil(iso: string, today = new Date()): number {
  const a = parseDate(toISODate(today)).getTime()
  const b = parseDate(iso).getTime()
  return Math.round((b - a) / 86_400_000)
}

export function shiftMonth(key: string, delta: number): string {
  const d = parseDate(key + '-01')
  d.setMonth(d.getMonth() + delta)
  return toISODate(d).slice(0, 7)
}

const MONTHS = ['jan', 'feb', 'mar', 'apr', 'may', 'jun', 'jul', 'aug', 'sep', 'oct', 'nov', 'dec']

function monthIndex(word: string): number {
  return MONTHS.indexOf(word.slice(0, 3).toLowerCase())
}

function fullYear(y: string): number {
  const n = Number(y)
  return y.length <= 2 ? 2000 + n : n
}

function validDate(y: number, m: number, d: number): string | null {
  const dt = new Date(y, m, d)
  if (dt.getFullYear() !== y || dt.getMonth() !== m || dt.getDate() !== d || y < 1900 || y > 2200) return null
  return toISODate(dt)
}

/**
 * Reads a date typed by a person: "2026-10-08", "8/10/2026" (day first, as printed on Pakistani bills),
 * "8 Oct 2026", "08-Oct-26", "Oct 8, 2026". Returns YYYY-MM-DD or null.
 */
export function parseUserDate(text: string): string | null {
  const s = text.trim()
  let m = s.match(/^(\d{4})-(\d{1,2})-(\d{1,2})$/)
  if (m) return validDate(Number(m[1]), Number(m[2]) - 1, Number(m[3]))
  m = s.match(/^(\d{1,2})[/.\-\s](\d{1,2})[/.\-\s](\d{2}|\d{4})$/)
  if (m) return validDate(fullYear(m[3]!), Number(m[2]) - 1, Number(m[1]))
  m = s.match(/^(\d{1,2})[\s\-/.]*([a-z]{3,9})[\s\-/.,]*(\d{2}|\d{4})$/i)
  if (m && monthIndex(m[2]!) >= 0) return validDate(fullYear(m[3]!), monthIndex(m[2]!), Number(m[1]))
  m = s.match(/^([a-z]{3,9})[\s\-/.]*(\d{1,2})[\s,]+(\d{4})$/i)
  if (m && monthIndex(m[1]!) >= 0) return validDate(Number(m[3]), monthIndex(m[1]!), Number(m[2]))
  return null
}

/** Reads a month typed by a person: "2026-09", "09/2026", "Sep 2026", "September 2026", "Sep-26". Returns YYYY-MM or null. */
export function parseUserMonth(text: string): string | null {
  const s = text.trim()
  let m = s.match(/^(\d{4})-(\d{1,2})$/)
  if (!m) {
    const mm = s.match(/^(\d{1,2})[/.\-\s](\d{4})$/)
    if (mm) m = [s, mm[2]!, mm[1]!] as RegExpMatchArray
  }
  if (m) {
    const month = Number(m[2])
    return month >= 1 && month <= 12 ? `${m[1]}-${String(month).padStart(2, '0')}` : null
  }
  const w = s.match(/^([a-z]{3,9})[\s\-/.,']*(\d{2}|\d{4})$/i)
  if (w && monthIndex(w[1]!) >= 0) return `${fullYear(w[2]!)}-${String(monthIndex(w[1]!) + 1).padStart(2, '0')}`
  return null
}
