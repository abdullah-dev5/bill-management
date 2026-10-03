import { useInfiniteQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import type { Bill, Profile } from '@/api/client'
import { fetchBillsPage, keys, type BillFilters } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { EmptyState, ErrorNotice, IconTile, Skeleton, StatusBadge } from '@/components/ui/misc'
import { formatDate, formatDayMonth, formatMonth } from '@/lib/dates'
import { formatMoney } from '@/lib/money'

/** "September 2026" for monthly bills, the description for expense entries. */
export function billTitle(bill: Bill): string {
  return bill.periodKey ? formatMonth(bill.periodKey) : (bill.description ?? formatDate(bill.expenseDate))
}

/** Two-line record row (docs/10 §5 Records row). Shows the profile when listing across profiles. */
export function BillRow({ bill, profile }: { bill: Bill; profile?: Profile }) {
  const { t } = useTranslation()
  const open = bill.status !== 'paid' && bill.status !== 'cancelled' && bill.outstandingMinor > 0
  const meta = bill.periodKey
    ? bill.dueDate
      ? t('bills.due', { date: formatDayMonth(bill.dueDate) })
      : formatDate(bill.expenseDate)
    : formatDate(bill.expenseDate)
  return (
    <Link
      to="/bills/$billId"
      params={{ billId: bill.id }}
      className="flex items-center gap-3 px-4 py-3 transition hover:bg-card-muted/60 sm:px-5"
    >
      {profile && <IconTile icon={profile.icon} color={profile.color} size="sm" />}
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate font-medium text-ink">{profile ? `${profile.name} · ${billTitle(bill)}` : billTitle(bill)}</span>
        <span className="truncate text-sm text-muted">{meta}</span>
      </span>
      <span className="flex flex-col items-end gap-1">
        <span className="tabular font-medium text-ink">{formatMoney(open ? bill.outstandingMinor : bill.payableMinor, bill.currency)}</span>
        <StatusBadge bill={bill} />
      </span>
    </Link>
  )
}

export function BillListSkeleton({ rows = 4 }: { rows?: number }) {
  return (
    <div className="flex flex-col divide-y divide-line">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="flex items-center gap-3 px-5 py-4">
          <Skeleton className="h-4 flex-1" />
          <Skeleton className="h-4 w-20" />
        </div>
      ))}
    </div>
  )
}

/** A paged list of bills inside a card; loads more on demand. */
export function BillsList({
  filters,
  profilesById,
  empty,
}: {
  filters: BillFilters
  profilesById?: Map<string, Profile>
  empty?: React.ReactNode
}) {
  const { t } = useTranslation()
  const query = useInfiniteQuery({
    queryKey: keys.bills(filters),
    queryFn: ({ pageParam }) => fetchBillsPage(filters, pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.nextCursor ?? undefined,
  })

  if (query.isPending) return <BillListSkeleton />
  if (query.isError) return <ErrorNotice error={query.error} onRetry={() => void query.refetch()} />
  const bills = query.data.pages.flatMap((p) => p.items)
  if (bills.length === 0) return <>{empty ?? <EmptyState title={t('bills.emptyTitle')} body={t('bills.emptyBody')} />}</>

  return (
    <div className="overflow-hidden rounded-card bg-card">
      <ul className="divide-y divide-line">
        {bills.map((b) => (
          <li key={b.id}>
            <BillRow bill={b} profile={profilesById?.get(b.profileId)} />
          </li>
        ))}
      </ul>
      {query.hasNextPage && (
        <div className="border-t border-line p-3 text-center">
          <Button variant="ghost" size="sm" loading={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>
            {t('bills.loadMore')}
          </Button>
        </div>
      )}
    </div>
  )
}
