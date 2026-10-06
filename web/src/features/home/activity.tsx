import { Link } from '@tanstack/react-router'
import { CircleMinus, FilePlus2, LayoutGrid, ReceiptText, Wallet } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { Schemas } from '@/api/client'
import { useActivity } from '@/api/hooks'
import { Card, SectionTitle, Skeleton } from '@/components/ui/misc'
import { formatDate, formatMonth, toISODate } from '@/lib/dates'
import { formatMoney } from '@/lib/money'
import { tr } from '@/lib/tr'

type Item = Schemas['ActivityItem']

const ICONS = {
  bill_added: ReceiptText,
  payment_added: Wallet,
  payment_removed: CircleMinus,
  document_added: FilePlus2,
  profile_added: LayoutGrid,
} as const

/** Recent activity on Home (FR-DASH-05): bills added, payments, documents and new profiles. */
export function RecentActivity() {
  const { t } = useTranslation()
  const activity = useActivity(8)
  // Say who did it only when more than one person is adding things.
  const showActor = new Set((activity.data ?? []).map((i) => i.actorName).filter(Boolean)).size > 1
  return (
    <section>
      <SectionTitle>{t('activity.title')}</SectionTitle>
      {!activity.data ? (
        <Skeleton className="h-40 rounded-card" />
      ) : activity.data.length === 0 ? (
        <Card className="text-sm text-muted">{t('activity.none')}</Card>
      ) : (
        <ul className="divide-y divide-line overflow-hidden rounded-card bg-card" data-testid="recent-activity">
          {activity.data.map((item) => (
            <li key={item.id}>
              <ActivityRow item={item} showActor={showActor} />
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function ActivityRow({ item, showActor }: { item: Item; showActor: boolean }) {
  const { t } = useTranslation()
  const Icon = ICONS[item.action]
  const amount = item.amountMinor != null ? formatMoney(item.amountMinor, item.currency ?? 'PKR') : ''
  const what = item.periodKey ? formatMonth(item.periodKey) : item.description
  const title =
    item.action === 'payment_added' || item.action === 'payment_removed'
      ? t(`activity.${item.action}`, { amount })
      : t(`activity.${item.action}`)
  const subject = item.action === 'profile_added' ? item.profileName : [item.profileName, what].filter(Boolean).join(' · ')
  const extra = [
    item.action === 'bill_added' ? amount : undefined,
    item.action === 'document_added' ? item.detail : undefined,
    (item.action === 'payment_added' || item.action === 'payment_removed') && item.detail ? t(`method.${item.detail as 'cash'}`) : undefined,
  ].filter(Boolean)
  const meta = [relativeTime(item.at), showActor && item.actorName ? t('activity.by', { name: item.actorName.split(' ')[0] ?? item.actorName }) : undefined]
    .filter(Boolean)
    .join(' · ')

  const body = (
    <>
      <span className="grid size-9 shrink-0 place-items-center rounded-tile bg-card-muted text-ink">
        <Icon className="size-4.5" aria-hidden />
      </span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-sm text-ink">
          <span className="font-semibold">{title}</span> · {subject}
        </span>
        <span className="truncate text-xs text-muted">{[...extra, meta].join(' · ')}</span>
      </span>
    </>
  )
  const cls = 'flex items-center gap-3 px-4 py-3 sm:px-5'
  if (item.billId)
    return (
      <Link to="/bills/$billId" params={{ billId: item.billId }} className={`${cls} transition hover:bg-card-muted/60`}>
        {body}
      </Link>
    )
  return (
    <Link to="/profiles/$profileId" params={{ profileId: item.profileId }} className={`${cls} transition hover:bg-card-muted/60`}>
      {body}
    </Link>
  )
}

/** "just now", "5 min ago", "3 hours ago", "yesterday", else the date. */
export function relativeTime(at: string, now = new Date()): string {
  const t = tr
  const then = new Date(at)
  const minutes = Math.round((now.getTime() - then.getTime()) / 60_000)
  if (minutes < 1) return t('activity.justNow')
  if (minutes < 60) return t('activity.minutesAgo', { count: minutes })
  if (toISODate(then) === toISODate(now)) return t('activity.hoursAgo', { count: Math.round(minutes / 60) })
  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  if (toISODate(then) === toISODate(yesterday)) return t('activity.yesterday')
  return formatDate(toISODate(then))
}
