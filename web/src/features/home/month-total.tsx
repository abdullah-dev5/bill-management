import { Link } from '@tanstack/react-router'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Schemas } from '@/api/client'
import { Card, FeaturedCard, IconTile } from '@/components/ui/misc'
import { formatDate, formatMonth } from '@/lib/dates'
import { formatMoney } from '@/lib/money'

type MonthTotal = Schemas['MonthTotal']

/** FR-DASH-07: what the household has to pay for the month's bills, all profiles together. */
export function MonthTotalCard({ month, total, currency }: { month: string; total: MonthTotal; currency: string }) {
  const { t } = useTranslation()
  const money = (v: number) => formatMoney(v, currency)
  const paidPct = total.totalMinor > 0 ? Math.min(100, (total.paidMinor / total.totalMinor) * 100) : 0

  return (
    <FeaturedCard className="flex flex-col gap-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold tracking-wide text-on-featured/70">{t('home.monthBills', { month: formatMonth(month) }).toUpperCase()}</p>
          <p className="tabular mt-2 text-[34px] leading-tight font-medium sm:text-amount-hero" data-testid="month-total">
            {money(total.totalMinor)}
          </p>
          <p className="mt-1 text-sm text-on-featured/70">{t('home.monthBillCount', { count: total.billCount })}</p>
        </div>
        {total.nextDue && (
          <div className="rounded-tile bg-on-featured/10 px-4 py-3 text-sm">
            <p className="text-on-featured/70">{t('home.nextDue')}</p>
            <p className="font-medium">
              {total.profiles.find((p) => p.profileId === total.nextDue!.profileId)?.name}
              {' · '}
              {money(total.nextDue.outstandingMinor)}
            </p>
            {total.nextDue.dueDate && <p className="text-on-featured/70">{t('bills.due', { date: formatDate(total.nextDue.dueDate) })}</p>}
          </div>
        )}
      </div>

      {total.billCount > 0 && (
        <div className="flex flex-col gap-2">
          <div
            className="h-2.5 overflow-hidden rounded-full bg-on-featured/15"
            role="meter"
            aria-label={t('home.paidOfTotal')}
            aria-valuemin={0}
            aria-valuemax={total.totalMinor}
            aria-valuenow={total.paidMinor}
          >
            <div className="h-full rounded-full bg-accent" style={{ width: `${paidPct}%` }} />
          </div>
          <div className="tabular flex flex-wrap justify-between gap-2 text-sm">
            <span>
              {t('home.paidSoFar')}: <strong>{money(total.paidMinor)}</strong>
            </span>
            <span>
              {t('home.leftToPay')}: <strong>{money(total.remainingMinor)}</strong>
            </span>
          </div>
        </div>
      )}

      {total.groups.length > 0 && (
        <ul className="grid gap-2 sm:grid-cols-3">
          {total.groups.map((g) => (
            <li key={g.group} className="rounded-tile bg-on-featured/10 px-4 py-3">
              <p className="text-sm text-on-featured/70">{t(`group.${g.group}`)}</p>
              <p className="tabular font-medium">{money(g.totalMinor)}</p>
              <p className="tabular text-xs text-on-featured/70">
                {g.remainingMinor > 0 ? t('home.leftAmount', { amount: money(g.remainingMinor) }) : t('home.allPaid')}
              </p>
            </li>
          ))}
        </ul>
      )}

      {total.earlierOutstandingMinor > 0 && (
        <p className="text-sm text-on-featured/70">{t('home.olderOutstanding', { amount: money(total.earlierOutstandingMinor) })}</p>
      )}
    </FeaturedCard>
  )
}

/** The month's total per profile, largest first. */
export function MonthProfiles({ total, currency }: { total: MonthTotal; currency: string }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  if (total.profiles.length === 0) return null
  const rows = [...total.profiles].sort((a, b) => b.totalMinor - a.totalMinor)
  const shown = open ? rows : rows.slice(0, 4)
  return (
    <Card className="flex flex-col gap-1">
      <ul className="divide-y divide-line">
        {shown.map((p) => (
          <li key={p.profileId}>
            <Link to="/profiles/$profileId" params={{ profileId: p.profileId }} className="flex items-center gap-3 py-3">
              <IconTile icon={p.icon} color={p.color} size="sm" />
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="truncate font-medium text-ink">{p.name}</span>
                <span className="text-sm text-muted">{t(`group.${p.group}`)}</span>
              </span>
              <span className="tabular flex flex-col items-end">
                <span className="font-medium text-ink">{formatMoney(p.totalMinor, currency)}</span>
                <span className={p.remainingMinor > 0 ? 'text-sm text-[var(--st-soon-fg)]' : 'text-sm text-[var(--st-paid-fg)]'}>
                  {p.remainingMinor > 0 ? t('home.leftAmount', { amount: formatMoney(p.remainingMinor, currency) }) : t('home.allPaid')}
                </span>
              </span>
            </Link>
          </li>
        ))}
      </ul>
      {rows.length > 4 && (
        <button type="button" onClick={() => setOpen((o) => !o)} className="flex items-center gap-1 self-start pt-1 text-sm font-semibold text-ink" aria-expanded={open}>
          {open ? <ChevronUp className="size-4" aria-hidden /> : <ChevronDown className="size-4" aria-hidden />}
          {open ? t('common.showLess') : t('home.showAllProfiles', { count: rows.length })}
        </button>
      )}
    </Card>
  )
}
