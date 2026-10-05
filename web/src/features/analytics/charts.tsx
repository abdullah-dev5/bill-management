import { useState } from 'react'
import { cn } from '@/lib/cn'

/** Rounds a maximum up to a clean axis top with 4 steps (0 / 1,000 / 2,000 …). */
export function niceScale(max: number, steps = 4): { top: number; step: number } {
  if (max <= 0) return { top: steps, step: 1 }
  const raw = max / steps
  const mag = 10 ** Math.floor(Math.log10(raw))
  const step = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((s) => s >= raw) ?? 10 * mag
  return { top: step * steps, step }
}

export interface StackSeries {
  key: string
  label: string
  color: string // a CSS colour, e.g. var(--series-1)
  values: number[]
}

/**
 * Columns per month, stacked by series (part-to-whole over time). Marks follow the
 * data-viz specs: ≤24px columns, 2px surface gaps, 4px rounded tops, hairline grid,
 * one tooltip per column listing every series.
 */
export function StackedColumns({
  labels,
  series,
  format,
  ariaLabel,
  axisFormat,
}: {
  labels: string[]
  series: StackSeries[]
  format: (v: number) => string
  axisFormat?: (v: number) => string
  ariaLabel: string
}) {
  const [hover, setHover] = useState<number>()
  const totals = labels.map((_, i) => series.reduce((sum, s) => sum + Math.max(0, s.values[i] ?? 0), 0))
  const { top, step } = niceScale(Math.max(...totals, 0))
  const ticks = Array.from({ length: Math.round(top / step) + 1 }, (_, i) => i * step)
  const fmtAxis = axisFormat ?? format

  return (
    <div className="flex flex-col gap-3">
      <div className="relative mt-4 flex h-56 gap-2" role="img" aria-label={ariaLabel}>
        {/* y-axis + gridlines */}
        <div className="relative w-14 shrink-0 text-right text-[11px] text-muted tabular">
          {ticks.map((tk) => (
            <span key={tk} className="absolute right-1 -translate-y-1/2" style={{ bottom: `${(tk / top) * 100}%` }}>
              {fmtAxis(tk)}
            </span>
          ))}
        </div>
        <div className="relative flex-1">
          {ticks.map((tk) => (
            <div key={tk} className="absolute inset-x-0 h-px bg-[var(--chart-grid)]" style={{ bottom: `${(tk / top) * 100}%` }} aria-hidden />
          ))}
          <div className="absolute inset-0 flex items-end justify-around gap-1">
            {labels.map((label, i) => {
              const parts = series.map((s) => ({ s, v: Math.max(0, s.values[i] ?? 0) })).filter((p) => p.v > 0)
              return (
                <div
                  key={label}
                  tabIndex={0}
                  className="relative flex h-full min-w-0 flex-1 cursor-default items-end justify-center outline-none focus-visible:bg-card-muted/60"
                  onPointerEnter={() => setHover(i)}
                  onPointerLeave={() => setHover(undefined)}
                  onFocus={() => setHover(i)}
                  onBlur={() => setHover(undefined)}
                  aria-label={`${label}: ${format(totals[i] ?? 0)}`}
                >
                  <div
                    className={cn('flex w-full max-w-6 flex-col-reverse gap-[2px] transition', hover === i && 'brightness-110')}
                    style={{ height: `${((totals[i] ?? 0) / top) * 100}%` }}
                  >
                    {parts.map((p, j) => (
                      <div
                        key={p.s.key}
                        className={cn(j === parts.length - 1 && 'rounded-t-[4px]')}
                        style={{ flexGrow: p.v, flexBasis: 0, backgroundColor: p.s.color, minHeight: 2 }}
                      />
                    ))}
                  </div>
                  {hover === i && (
                    <div
                      role="tooltip"
                      className={cn(
                        'pointer-events-none absolute bottom-full z-20 mb-2 min-w-40 rounded-field bg-card p-3 text-xs shadow-float ring-1 ring-line',
                        i > labels.length / 2 ? 'right-0' : 'left-0',
                      )}
                    >
                      <p className="mb-1 text-muted">{label}</p>
                      <p className="tabular text-sm font-semibold text-ink">{format(totals[i] ?? 0)}</p>
                      <ul className="mt-1 flex flex-col gap-0.5">
                        {series.map((s) => (
                          <li key={s.key} className="flex items-center justify-between gap-3">
                            <span className="flex items-center gap-1.5 text-body">
                              <span className="h-0.5 w-3 rounded-full" style={{ backgroundColor: s.color }} aria-hidden />
                              {s.label}
                            </span>
                            <span className="tabular text-ink">{format(s.values[i] ?? 0)}</span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      </div>
      {/* x labels */}
      <div className="flex gap-2">
        <div className="w-14 shrink-0" />
        <div className="flex flex-1 justify-around gap-1">
          {labels.map((label, i) => (
            <span key={label} className="min-w-0 flex-1 truncate text-center text-[11px] text-muted">
              {labels.length > 8 && i % 2 === 1 ? '' : label.split(' ')[0]}
            </span>
          ))}
        </div>
      </div>
      {series.length >= 2 && (
        <ul className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-body" aria-label="Legend">
          {series.map((s) => (
            <li key={s.key} className="flex items-center gap-1.5">
              <span className="size-2.5 rounded-sm" style={{ backgroundColor: s.color }} aria-hidden />
              {s.label}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

/** Horizontal bars for a ranked breakdown (one series, value at the tip). */
export function RankedBars({
  rows,
  format,
  color = 'var(--series-1)',
}: {
  rows: { key: string; label: string; value: number; sub?: string }[]
  format: (v: number) => string
  color?: string
}) {
  const max = Math.max(1, ...rows.map((r) => r.value))
  return (
    <ul className="flex flex-col gap-3">
      {rows.map((r) => (
        <li key={r.key} className="flex flex-col gap-1">
          <div className="flex items-baseline justify-between gap-3 text-sm">
            <span className="min-w-0 truncate text-ink">{r.label}</span>
            <span className="tabular shrink-0 font-medium text-ink">{format(r.value)}</span>
          </div>
          <div className="h-2.5 w-full" aria-hidden>
            <div className="h-full rounded-r-[4px]" style={{ width: `${Math.max(1, (r.value / max) * 100)}%`, backgroundColor: color }} />
          </div>
          {r.sub && <span className="text-xs text-muted">{r.sub}</span>}
        </li>
      ))}
    </ul>
  )
}

/** Small single-series columns (e.g. units per month), with a tooltip per column. */
export function MiniColumns({
  labels,
  values,
  format,
  ariaLabel,
  color = 'var(--series-1)',
}: {
  labels: string[]
  values: (number | null | undefined)[]
  format: (v: number) => string
  ariaLabel: string
  color?: string
}) {
  const [hover, setHover] = useState<number>()
  const max = Math.max(1, ...values.map((v) => v ?? 0))
  return (
    <div className="flex flex-col gap-1">
      <div className="flex h-20 items-end gap-[2px]" role="img" aria-label={ariaLabel}>
        {labels.map((label, i) => {
          const v = values[i]
          return (
            <div
              key={label}
              tabIndex={0}
              className="relative flex h-full min-w-0 flex-1 items-end justify-center outline-none"
              onPointerEnter={() => setHover(i)}
              onPointerLeave={() => setHover(undefined)}
              onFocus={() => setHover(i)}
              onBlur={() => setHover(undefined)}
              aria-label={`${label}: ${v != null ? format(v) : '—'}`}
            >
              <div
                className={cn('w-full max-w-6 rounded-t-[4px]', v == null && 'bg-card-muted')}
                style={{ height: v != null ? `${Math.max(3, (v / max) * 100)}%` : '3%', backgroundColor: v != null ? color : undefined }}
              />
              {hover === i && (
                <div
                  role="tooltip"
                  className="pointer-events-none absolute bottom-full z-20 mb-1 whitespace-nowrap rounded-field bg-card px-2 py-1 text-xs shadow-float ring-1 ring-line"
                >
                  <span className="tabular font-semibold text-ink">{v != null ? format(v) : '—'}</span> <span className="text-muted">{label}</span>
                </div>
              )}
            </div>
          )
        })}
      </div>
      <div className="flex gap-[2px]" aria-hidden>
        {labels.map((label, i) => (
          <span key={label} className="min-w-0 flex-1 truncate text-center text-[10px] text-muted">
            {labels.length > 6 && i % 3 !== 0 ? '' : label.split(' ')[0]}
          </span>
        ))}
      </div>
    </div>
  )
}
