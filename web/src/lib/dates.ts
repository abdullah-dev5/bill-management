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

const dayMonth = new Intl.DateTimeFormat('en-GB', { day: '2-digit', month: 'short' })
const full = new Intl.DateTimeFormat('en-GB', { day: '2-digit', month: 'short', year: 'numeric' })
const monthYear = new Intl.DateTimeFormat('en-GB', { month: 'long', year: 'numeric' })
const monthShort = new Intl.DateTimeFormat('en-GB', { month: 'short', year: 'numeric' })

/** "08 Oct" */
export const formatDayMonth = (iso: string) => dayMonth.format(parseDate(iso))
/** "08 Oct 2026" */
export const formatDate = (iso: string) => full.format(parseDate(iso))
/** "2026-09" → "September 2026" */
export const formatMonth = (key: string) => monthYear.format(parseDate(key + '-01'))
/** "2026-09" → "Sep 2026" */
export const formatMonthShort = (key: string) => monthShort.format(parseDate(key + '-01'))

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
