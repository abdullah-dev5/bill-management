import { Link } from '@tanstack/react-router'
import { ChevronLeft, ChevronRight, LayoutGrid } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useHome, useMe, useProfiles } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { Card, EmptyState, ErrorNotice, SectionTitle, Skeleton } from '@/components/ui/misc'
import { currentMonthKey, formatMonth, shiftMonth } from '@/lib/dates'
import { BillRow } from '../bills/bill-row'
import { ProfileTile } from '../profiles/profile-pages'
import { MonthProfiles, MonthTotalCard } from './month-total'

export function HomePage() {
  const { t } = useTranslation()
  const { data: me } = useMe()
  const [month, setMonth] = useState(currentMonthKey())
  const home = useHome(month)
  const { data: profiles } = useProfiles()
  const byId = useMemo(() => new Map((profiles ?? []).map((p) => [p.id, p])), [profiles])
  const currency = me?.household.currency ?? 'PKR'
  const hasRealProfiles = (profiles ?? []).some((p) => !p.isDefault)

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold sm:text-[28px]">{t('home.greeting', { name: me?.user.displayName.split(' ')[0] ?? '' })}</h1>
        <div className="flex items-center gap-1 rounded-full bg-card p-1">
          <Button size="icon" variant="ghost" className="rounded-full" aria-label={t('home.prevMonth')} onClick={() => setMonth((m) => shiftMonth(m, -1))}>
            <ChevronLeft className="size-5" />
          </Button>
          <span className="min-w-36 text-center text-sm font-semibold text-ink">{formatMonth(month)}</span>
          <Button
            size="icon"
            variant="ghost"
            className="rounded-full"
            aria-label={t('home.nextMonth')}
            disabled={month >= currentMonthKey()}
            onClick={() => setMonth((m) => shiftMonth(m, 1))}
          >
            <ChevronRight className="size-5" />
          </Button>
        </div>
      </header>

      {home.isError ? (
        <ErrorNotice error={home.error} onRetry={() => void home.refetch()} />
      ) : !home.data ? (
        <div className="grid gap-4 lg:grid-cols-12">
          <Skeleton className="h-32 rounded-card lg:col-span-4" />
          <Skeleton className="h-32 rounded-card lg:col-span-8" />
          <Skeleton className="h-48 rounded-card lg:col-span-12" />
        </div>
      ) : (
        <>
          <div className="grid gap-4 lg:grid-cols-12">
            <div className="lg:col-span-7">
              <MonthTotalCard month={month} total={home.data.monthTotal} currency={currency} />
            </div>
            <div className="lg:col-span-5">
              <MonthProfiles total={home.data.monthTotal} currency={currency} />
            </div>
          </div>

          {!hasRealProfiles && profiles && (
            <Card>
              <EmptyState
                icon={<LayoutGrid className="size-10" />}
                title={t('home.emptyTitle')}
                body={t('home.emptyBody')}
                action={
                  <Link to="/onboarding" className="inline-flex h-12 items-center rounded-full bg-featured px-6 text-sm font-semibold text-on-featured">
                    {t('home.emptyAction')}
                  </Link>
                }
              />
            </Card>
          )}

          {(home.data.overdue.length > 0 || home.data.dueSoon.length > 0 || hasRealProfiles) && (
            <div className="grid gap-5 lg:grid-cols-2">
              {home.data.overdue.length > 0 && (
                <section>
                  <SectionTitle>{t('home.overdue')}</SectionTitle>
                  <ul className="divide-y divide-line overflow-hidden rounded-card bg-card">
                    {home.data.overdue.map((b) => (
                      <li key={b.id}>
                        <BillRow bill={b} profile={byId.get(b.profileId)} />
                      </li>
                    ))}
                  </ul>
                </section>
              )}
              <section>
                <SectionTitle>{t('home.dueSoon')}</SectionTitle>
                {home.data.dueSoon.length === 0 ? (
                  <Card className="text-sm text-muted">{t('home.nothingDue')}</Card>
                ) : (
                  <ul className="divide-y divide-line overflow-hidden rounded-card bg-card">
                    {home.data.dueSoon.map((b) => (
                      <li key={b.id}>
                        <BillRow bill={b} profile={byId.get(b.profileId)} />
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            </div>
          )}

          {home.data.pinned.length > 0 && (
            <section>
              <SectionTitle
                action={
                  <Link to="/profiles" className="text-sm font-semibold text-ink underline-offset-4 hover:underline">
                    {t('common.seeAll')}
                  </Link>
                }
              >
                {t('home.pinned')}
              </SectionTitle>
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                {home.data.pinned.map((p) => (
                  <ProfileTile key={p.id} profile={p} />
                ))}
              </div>
            </section>
          )}

          {home.data.recent.length > 0 && (
            <section>
              <SectionTitle>{t('home.recent')}</SectionTitle>
              <ul className="divide-y divide-line overflow-hidden rounded-card bg-card">
                {home.data.recent.map((b) => (
                  <li key={b.id}>
                    <BillRow bill={b} profile={byId.get(b.profileId)} />
                  </li>
                ))}
              </ul>
            </section>
          )}
        </>
      )}
    </div>
  )
}
