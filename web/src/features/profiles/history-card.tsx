import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Schemas } from '@/api/client'
import { useProfileHistory } from '@/api/hooks'
import { Card, Chip, SectionTitle, Skeleton } from '@/components/ui/misc'
import { Link } from '@tanstack/react-router'
import { cn } from '@/lib/cn'
import { formatMonthShort } from '@/lib/dates'
import { formatMoney } from '@/lib/money'

type Month = Schemas['HistoryMonth']

const SHOWN = 12

const STATUS_KEY = {
  paid: 'paid', unpaid: 'unpaid', overdue: 'overdue', partially_paid: 'partial', draft: 'review', cancelled: 'cancelled',
} as const

export type Metric = 'amount' | 'units'

const valueOf = (m: Month, metric: Metric) => (metric === 'amount' ? m.billedMinor : m.units) ?? 0

/** Bars for the last months' bill amounts or units; bills and printed history are told apart. */
export function HistoryChart({ months, metric = 'amount', unit = '' }: { months: Month[]; metric?: Metric; unit?: string }) {
  const { t } = useTranslation()
  const rows = months.slice(-SHOWN)
  const max = Math.max(1, ...rows.map((m) => valueOf(m, metric)))
  const label = (m: Month) =>
    metric === 'amount' ? (m.billedMinor != null ? formatMoney(m.billedMinor) : '—') : m.units != null ? `${m.units} ${unit}` : '—'
  return (
    <div className="flex h-40 items-end gap-1.5 sm:gap-2" role="img" aria-label={metric === 'amount' ? t('profiles.historyChart') : t('profiles.unitsChart')}>
      {rows.map((m) => {
        const pct = Math.max(2, (valueOf(m, metric) / max) * 100)
        return (
          <div key={m.month} className="flex min-w-0 flex-1 flex-col items-center gap-1">
            <div className="flex h-32 w-full items-end">
              <div
                title={`${formatMonthShort(m.month)}: ${label(m)}`}
                className={cn('w-full rounded-t-[6px]', m.source === 'bill' ? 'bg-featured' : 'bg-featured/35')}
                style={{ height: `${pct}%` }}
              />
            </div>
            <span className="truncate text-[11px] text-muted">{formatMonthShort(m.month).split(' ')[0]}</span>
          </div>
        )
      })}
    </div>
  )
}

/** Month-by-month history on the profile dashboard: the profile's bills plus history printed on them. */
export function HistoryCard({ profileId, unit }: { profileId: string; unit?: string | null }) {
  const { t } = useTranslation()
  const history = useProfileHistory(profileId)
  const [metric, setMetric] = useState<Metric>('amount')
  if (history.isLoading) return <Skeleton className="h-64 rounded-card" />
  const months = history.data ?? []
  if (months.length < 2) return null
  const recent = [...months].reverse().slice(0, SHOWN)
  const hasUnits = months.some((m) => m.units != null)

  return (
    <Card>
      <SectionTitle
        action={
          hasUnits ? (
            <div className="flex gap-1" role="group" aria-label={t('profiles.chartShows')}>
              <Chip selected={metric === 'amount'} onClick={() => setMetric('amount')} className="h-9 px-3">
                {t('profiles.chartAmount')}
              </Chip>
              <Chip selected={metric === 'units'} onClick={() => setMetric('units')} className="h-9 px-3">
                {t('profiles.chartUnits')}
              </Chip>
            </div>
          ) : undefined
        }
      >
        {t('profiles.history')}
      </SectionTitle>
      <HistoryChart months={months} metric={metric} unit={unit || t('bills.unitsDefault')} />
      <p className="mt-2 flex flex-wrap gap-4 text-xs text-muted">
        <span className="flex items-center gap-1.5">
          <span className="size-2.5 rounded-sm bg-featured" aria-hidden /> {t('profiles.historyFromBills')}
        </span>
        <span className="flex items-center gap-1.5">
          <span className="size-2.5 rounded-sm bg-featured/35" aria-hidden /> {t('profiles.historyPrinted')}
        </span>
      </p>
      <div className="mt-4 overflow-x-auto">
        <table className="tabular w-full text-sm">
          <thead className="text-left text-muted">
            <tr>
              <th className="py-1.5 font-medium">{t('extraction.month')}</th>
              {hasUnits && <th className="py-1.5 text-right font-medium">{t('extraction.units')}</th>}
              <th className="py-1.5 text-right font-medium">{t('extraction.billed')}</th>
              <th className="py-1.5 text-right font-medium">{t('extraction.paid')}</th>
              <th className="py-1.5 text-right font-medium">{t('profiles.historyStatus')}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {recent.map((m) => (
              <tr key={m.month}>
                <td className="py-1.5 text-ink">
                  {m.billId ? (
                    <Link to="/bills/$billId" params={{ billId: m.billId }} className="underline-offset-4 hover:underline">
                      {formatMonthShort(m.month)}
                    </Link>
                  ) : (
                    formatMonthShort(m.month)
                  )}
                </td>
                {hasUnits && <td className="py-1.5 text-right">{m.units ?? '—'}</td>}
                <td className="py-1.5 text-right text-ink">{m.billedMinor != null ? formatMoney(m.billedMinor) : '—'}</td>
                <td className="py-1.5 text-right">{m.paidMinor != null ? formatMoney(m.paidMinor) : '—'}</td>
                <td className={cn('py-1.5 text-right text-xs', m.status === 'paid' && 'text-[var(--st-paid-fg)]', m.status === 'overdue' && 'text-[var(--st-overdue-fg)]')}>
                  {m.status ? t(`status.${STATUS_KEY[m.status]}`) : t('profiles.historyPrintedShort')}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  )
}
