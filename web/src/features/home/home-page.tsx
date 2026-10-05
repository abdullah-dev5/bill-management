import { Link } from '@tanstack/react-router'
import { AlertTriangle, ChevronLeft, ChevronRight, LayoutGrid } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useHome, useMe, useProfiles } from '@/api/hooks'
import { SearchButton } from '@/components/layout/app-shell'
import { Button } from '@/components/ui/button'
import { Card, EmptyState, ErrorNotice, IconTile, SectionTitle, Skeleton } from '@/components/ui/misc'
import { currentMonthKey, formatMonth, shiftMonth } from '@/lib/dates'
import { BillRow } from '../bills/bill-row'
import { ProfileTile } from '../profiles/profile-pages'
import { RecentActivity } from './activity'
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
        <div className="flex items-center gap-2">
        <SearchButton />
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

          {home.data.reviewCount > 0 && (
            <Link
              to="/bills"
              search={{ review: true }}
              data-testid="review-tile"
              className="flex items-center gap-3 rounded-card bg-[var(--st-soon-bg)] p-4 text-[var(--st-soon-fg)] transition hover:brightness-95"
            >
              <AlertTriangle className="size-5 shrink-0" aria-hidden />
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="font-semibold">{t('reconcile.review', { count: home.data.reviewCount })}</span>
                <span className="text-sm">{t('reconcile.reviewHint')}</span>
              </span>
              <ChevronRight className="size-5 shrink-0" aria-hidden />
            </Link>
          )}

          {home.data.expectedMissing.length > 0 && (
            <section data-testid="expected-missing">
              <SectionTitle>{t('expected.title')}</SectionTitle>
              <ul className="divide-y divide-line overflow-hidden rounded-card bg-card">
                {home.data.expectedMissing.map((m) => (
                  <li key={m.profileId} className="flex items-center gap-3 px-4 py-3 sm:px-5">
                    <IconTile icon={m.icon} color={m.color} size="sm" />
                    <span className="flex min-w-0 flex-1 flex-col">
                      <span className="truncate font-medium text-ink">{m.profileName}</span>
                      <span className="truncate text-sm text-[var(--st-soon-fg)]">{t('expected.line', { month: formatMonth(m.periodKey) })}</span>
                    </span>
                    <Link
                      to="/bills/new"
                      search={{ profileId: m.profileId, period: m.periodKey }}
                      className="inline-flex h-10 shrink-0 items-center rounded-full bg-featured px-4 text-sm font-semibold text-on-featured"
                      aria-label={`${t('expected.add')}: ${m.profileName} ${formatMonth(m.periodKey)}`}
                    >
                      {t('expected.add')}
                    </Link>
                  </li>
                ))}
              </ul>
              <p className="mt-2 text-sm text-muted">{t('expected.hint')}</p>
            </section>
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

          {hasRealProfiles && <RecentActivity />}
        </>
      )}
    </div>
  )
}
