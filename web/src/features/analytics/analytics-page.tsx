import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { ArrowDownRight, ArrowUpRight, BarChart3, Minus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Schemas } from '@/api/client'
import { useAnalytics } from '@/api/hooks'
import { PageHeader } from '@/components/layout/app-shell'
import { DateInput } from '@/components/ui/date-input'
import { Field } from '@/components/ui/field'
import { Card, Chip, EmptyState, ErrorNotice, IconTile, SectionTitle, Skeleton } from '@/components/ui/misc'
import { cn } from '@/lib/cn'
import { currentMonthKey, formatMonth, formatMonthShort, shiftMonth } from '@/lib/dates'
import { formatMoney } from '@/lib/money'
import { MiniColumns, RankedBars, StackedColumns } from './charts'

type Series = Schemas['AnalyticsSeries']
export type Preset = 'month' | '3m' | 'year' | '12m' | 'custom'

const GROUP_COLORS: Record<string, string> = { utilities: 'var(--series-1)', household: 'var(--series-2)', other: 'var(--series-3)' }
const GROUPS = ['utilities', 'household', 'other'] as const

/** The month range for a preset, ending with the current month. */
export function presetRange(preset: Preset, now = currentMonthKey()): { from: string; to: string } {
  switch (preset) {
    case 'month':
      return { from: now, to: now }
    case '3m':
      return { from: shiftMonth(now, -2), to: now }
    case 'year':
      return { from: `${now.slice(0, 4)}-01`, to: now }
    default:
      return { from: shiftMonth(now, -11), to: now }
  }
}

/** Change against a comparison amount: the absolute difference and the percentage. */
export function change(current: number, before: number): { diff: number; pct: number | null } {
  return { diff: current - before, pct: before > 0 ? Math.round(((current - before) / before) * 100) : null }
}

function ChangeLine({ current, before, label, format }: { current: number; before: number; label: string; format: (v: number) => string }) {
  const { t } = useTranslation()
  const { diff, pct } = change(current, before)
  const Icon = diff > 0 ? ArrowUpRight : diff < 0 ? ArrowDownRight : Minus
  if (before === 0 && current === 0) return null
  return (
    <p className="flex items-center gap-1.5 text-sm text-body">
      <Icon className="size-4 shrink-0 text-muted" aria-hidden />
      <span className="tabular">
        {before === 0
          ? t('analytics.noComparison', { label })
          : t(diff >= 0 ? 'analytics.up' : 'analytics.down', { pct: pct == null ? '' : Math.abs(pct), amount: format(Math.abs(diff)), label })}
      </span>
    </p>
  )
}

export function AnalyticsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const search = useSearch({ from: '/app/analytics' })
  const preset: Preset = search.preset ?? (search.from ? 'custom' : '12m')
  const range = preset === 'custom' ? { from: search.from ?? presetRange('12m').from, to: search.to ?? currentMonthKey() } : presetRange(preset)
  const basis = search.basis ?? 'spend'
  const analytics = useAnalytics(range.from, range.to, basis)
  const set = (patch: Partial<typeof search>) => void navigate({ to: '/analytics', search: { ...search, ...patch }, replace: true })
  const money = (v: number) => formatMoney(v)
  const compact = (v: number) =>
    v >= 100_000_00 ? `${Math.round(v / 100_000_00)}L` : v >= 100_000 ? `${Math.round(v / 100_000)}k` : String(Math.round(v / 100))
  const rangeLabel = range.from === range.to ? formatMonth(range.from) : `${formatMonthShort(range.from)} – ${formatMonthShort(range.to)}`

  return (
    <div className="flex flex-col gap-5">
      <PageHeader
        title={t('analytics.title')}
        actions={
          <Link to="/bills" className="inline-flex h-10 items-center rounded-full bg-card-muted px-4 text-sm font-semibold text-ink">
            {t('analytics.allBills')}
          </Link>
        }
      />

      <div className="flex flex-col gap-3">
        <div className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 sm:mx-0 sm:px-0" role="group" aria-label={t('analytics.range')}>
          {(['month', '3m', 'year', '12m', 'custom'] as const).map((p) => (
            <Chip
              key={p}
              selected={preset === p}
              onClick={() => set(p === 'custom' ? { preset: 'custom', ...range } : { preset: p, from: undefined, to: undefined })}
            >
              {t(`analytics.presets.${p}`)}
            </Chip>
          ))}
        </div>
        {preset === 'custom' && (
          <div className="flex flex-wrap items-end gap-3">
            <Field label={t('analytics.from')} className="w-44">
              {(p) => (
                <DateInput {...p} mode="month" required className="h-11" value={range.from} max={range.to} onChange={(v) => v && set({ preset: 'custom', from: v, to: range.to })} />
              )}
            </Field>
            <Field label={t('analytics.to')} className="w-44">
              {(p) => (
                <DateInput {...p} mode="month" required className="h-11" value={range.to} min={range.from} onChange={(v) => v && set({ preset: 'custom', from: range.from, to: v })} />
              )}
            </Field>
          </div>
        )}
        <div className="flex flex-wrap items-center gap-2" role="group" aria-label={t('analytics.basis')}>
          <Chip selected={basis === 'spend'} onClick={() => set({ basis: undefined })} className="h-9 px-3">
            {t('analytics.spend')}
          </Chip>
          <Chip selected={basis === 'cash'} onClick={() => set({ basis: 'cash' })} className="h-9 px-3">
            {t('analytics.cash')}
          </Chip>
          <span className="text-xs text-muted">{basis === 'spend' ? t('analytics.spendHint') : t('analytics.cashHint')}</span>
        </div>
      </div>

      {analytics.isError ? (
        <ErrorNotice error={analytics.error} onRetry={() => void analytics.refetch()} />
      ) : !analytics.data ? (
        <div className="grid gap-4 lg:grid-cols-3">
          <Skeleton className="h-32 rounded-card" />
          <Skeleton className="h-32 rounded-card" />
          <Skeleton className="h-32 rounded-card" />
          <Skeleton className="h-72 rounded-card lg:col-span-3" />
        </div>
      ) : analytics.data.total.totalMinor === 0 && analytics.data.total.previousMinor === 0 && analytics.data.total.lastYearMinor === 0 ? (
        <Card>
          <EmptyState
            icon={<BarChart3 className="size-10" />}
            title={t('analytics.emptyTitle')}
            body={t('analytics.emptyBody', { range: rangeLabel })}
          />
        </Card>
      ) : (
        <AnalyticsBody data={analytics.data} money={money} compact={compact} rangeLabel={rangeLabel} />
      )}
    </div>
  )
}

function AnalyticsBody({
  data,
  money,
  compact,
  rangeLabel,
}: {
  data: Schemas['Analytics']
  money: (v: number) => string
  compact: (v: number) => string
  rangeLabel: string
}) {
  const { t } = useTranslation()
  const [showData, setShowData] = useState(false)
  const labels = data.months.map(formatMonthShort)
  const groupSeries = GROUPS.map((g) => {
    const s = data.groups.find((x) => x.key === g)
    return { key: g, label: t(`group.${g}`), color: GROUP_COLORS[g]!, values: s?.months ?? data.months.map(() => 0) }
  }).filter((s) => s.values.some((v) => v > 0))
  const metered = data.profiles.filter((p) => p.unitsTotal != null && p.unitsMonths)
  const typeSub = (s: Series): string | undefined => {
    if (s.lastYearMinor <= 0) return undefined
    const c = change(s.totalMinor, s.lastYearMinor)
    return t('analytics.typeChange', { sign: c.diff >= 0 ? '+' : '−', pct: Math.abs(c.pct ?? 0), amount: money(s.lastYearMinor) })
  }

  return (
    <>
      <div className="grid gap-4 sm:grid-cols-3">
        <Card className="flex flex-col gap-1">
          <p className="text-sm text-muted">{data.basis === 'spend' ? t('analytics.totalSpend') : t('analytics.totalCash')}</p>
          <p className="tabular text-[28px] leading-tight font-medium text-ink" data-testid="analytics-total">
            {money(data.total.totalMinor)}
          </p>
          <p className="text-sm text-muted">{rangeLabel}</p>
        </Card>
        <Card className="flex flex-col gap-2">
          <p className="text-sm text-muted">{t('analytics.vsPrevious', { count: data.months.length })}</p>
          <p className="tabular text-xl font-medium text-ink">{money(data.total.previousMinor)}</p>
          <ChangeLine current={data.total.totalMinor} before={data.total.previousMinor} label={t('analytics.previousShort')} format={money} />
        </Card>
        <Card className="flex flex-col gap-2">
          <p className="text-sm text-muted">{t('analytics.vsLastYear')}</p>
          <p className="tabular text-xl font-medium text-ink">{money(data.total.lastYearMinor)}</p>
          <ChangeLine current={data.total.totalMinor} before={data.total.lastYearMinor} label={t('analytics.lastYearShort')} format={money} />
        </Card>
      </div>

      <Card>
        <SectionTitle
          action={
            <button
              type="button"
              className="text-sm font-semibold text-ink underline-offset-4 hover:underline"
              onClick={() => setShowData((v) => !v)}
              aria-expanded={showData}
            >
              {showData ? t('analytics.hideData') : t('analytics.showData')}
            </button>
          }
        >
          {t('analytics.byMonth')}
        </SectionTitle>
        <StackedColumns labels={labels} series={groupSeries} format={money} axisFormat={compact} ariaLabel={t('analytics.byMonthChart')} />
        {showData && (
          <div className="mt-4 relative overflow-x-auto">
            <table className="tabular w-full text-sm">
              <thead className="text-left text-muted">
                <tr>
                  <th className="py-1.5 font-medium">{t('extraction.month')}</th>
                  {groupSeries.map((g) => (
                    <th key={g.key} className="py-1.5 text-right font-medium">
                      {g.label}
                    </th>
                  ))}
                  <th className="py-1.5 text-right font-medium">{t('analytics.total')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {data.months.map((m, i) => (
                  <tr key={m}>
                    <td className="py-1.5 text-ink">{formatMonthShort(m)}</td>
                    {groupSeries.map((g) => (
                      <td key={g.key} className="py-1.5 text-right">
                        {money(g.values[i] ?? 0)}
                      </td>
                    ))}
                    <td className="py-1.5 text-right font-medium text-ink">{money(data.total.months[i] ?? 0)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      <div className="grid gap-5 lg:grid-cols-2">
        <Card data-testid="by-type">
          <SectionTitle>{t('analytics.byType')}</SectionTitle>
          <RankedBars
            rows={data.types.filter((s) => s.totalMinor > 0).map((s) => ({ key: s.key, label: s.name, value: s.totalMinor, sub: typeSub(s) }))}
            format={money}
          />
        </Card>
        <Card>
          <SectionTitle>{t('analytics.byProvider')}</SectionTitle>
          <RankedBars
            rows={data.providers
              .filter((s) => s.totalMinor > 0)
              .map((s) => ({ key: s.key, label: s.key === 'none' ? t('analytics.noProvider') : s.name, value: s.totalMinor }))}
            format={money}
          />
        </Card>
      </div>

      <Card>
        <SectionTitle>{t('analytics.byProfile')}</SectionTitle>
        <div className="relative overflow-x-auto">
          <table className="tabular w-full min-w-[560px] text-sm">
            <thead className="text-left text-muted">
              <tr>
                <th className="py-2 font-medium">{t('analytics.profile')}</th>
                <th className="py-2 text-right font-medium">{t('analytics.total')}</th>
                {data.basis === 'spend' && (
                  <>
                    <th className="py-2 text-right font-medium">{t('analytics.average')}</th>
                    <th className="py-2 text-right font-medium">{t('analytics.highest')}</th>
                    <th className="py-2 text-right font-medium">{t('analytics.lowest')}</th>
                  </>
                )}
                <th className="py-2 text-right font-medium">{t('analytics.vsPreviousShort')}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {data.profiles
                .filter((p) => p.totalMinor > 0 || p.previousMinor > 0)
                .map((p) => {
                  const c = change(p.totalMinor, p.previousMinor)
                  return (
                    <tr key={p.key}>
                      <td className="py-2">
                        <Link to="/profiles/$profileId" params={{ profileId: p.key }} className="flex items-center gap-2 text-ink hover:underline">
                          {p.icon && p.color != null && <IconTile icon={p.icon} color={p.color} size="sm" />}
                          <span className="truncate">{p.name}</span>
                        </Link>
                      </td>
                      <td className="py-2 text-right font-medium text-ink">{money(p.totalMinor)}</td>
                      {data.basis === 'spend' && (
                        <>
                          <td className="py-2 text-right">{p.avgMinor != null ? money(Math.round(p.avgMinor / 100) * 100) : '—'}</td>
                          <td className="py-2 text-right">
                            {p.maxMinor != null && p.maxBillId ? (
                              <Link to="/bills/$billId" params={{ billId: p.maxBillId }} className="hover:underline">
                                {money(p.maxMinor)}
                              </Link>
                            ) : (
                              '—'
                            )}
                          </td>
                          <td className="py-2 text-right">{p.minMinor != null ? money(p.minMinor) : '—'}</td>
                        </>
                      )}
                      <td className={cn('py-2 text-right text-body')}>
                        {p.previousMinor > 0 ? `${c.diff >= 0 ? '+' : '−'}${Math.abs(c.pct ?? 0)}%` : '—'}
                      </td>
                    </tr>
                  )
                })}
            </tbody>
          </table>
        </div>
      </Card>

      {metered.length > 0 && (
        <section>
          <SectionTitle>{t('analytics.consumption')}</SectionTitle>
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {metered.map((p) => {
              const unit = p.unit || t('bills.unitsDefault')
              const fmt = (v: number) => `${v.toLocaleString('en-PK', { maximumFractionDigits: 1 })} ${unit}`
              return (
                <Card key={p.key} className="flex flex-col gap-3" data-testid={`consumption-${p.name}`}>
                  <div className="flex items-center gap-2">
                    {p.icon && p.color != null && <IconTile icon={p.icon} color={p.color} size="sm" />}
                    <span className="font-medium text-ink">{p.name}</span>
                  </div>
                  <p className="tabular text-xl font-medium text-ink">{fmt(p.unitsTotal ?? 0)}</p>
                  <MiniColumns labels={labels} values={p.unitsMonths ?? []} format={fmt} ariaLabel={t('analytics.unitsChart', { name: p.name })} />
                  {p.unitsLastYear != null && p.unitsLastYear > 0 && (
                    <ChangeLine current={p.unitsTotal ?? 0} before={p.unitsLastYear} label={t('analytics.lastYearShort')} format={fmt} />
                  )}
                </Card>
              )
            })}
          </div>
        </section>
      )}
    </>
  )
}
