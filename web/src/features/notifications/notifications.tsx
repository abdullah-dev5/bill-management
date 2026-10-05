import { Link } from '@tanstack/react-router'
import { AlarmClock, Bell, BellOff, CalendarClock, CircleAlert, FileQuestion, Repeat } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { Schemas } from '@/api/client'
import { useMarkNotificationsRead, useNotifications } from '@/api/hooks'
import { PageHeader } from '@/components/layout/app-shell'
import { Button } from '@/components/ui/button'
import { Card, EmptyState, ErrorNotice, Skeleton } from '@/components/ui/misc'
import { cn } from '@/lib/cn'
import { daysUntil, formatDayMonth, formatMonth } from '@/lib/dates'
import { formatMoney } from '@/lib/money'
import { relativeTime } from '../home/activity'

type Notification = Schemas['Notification']

const ICONS = {
  due_soon: CalendarClock,
  due_today: AlarmClock,
  overdue: CircleAlert,
  missing_bill: FileQuestion,
  bill_created: Repeat,
} as const

/** Bell with the unread count; opens the notifications page. */
export function NotificationBell({ className }: { className?: string }) {
  const { t } = useTranslation()
  const { data } = useNotifications()
  const unread = data?.unreadCount ?? 0
  return (
    <Link
      to="/notifications"
      aria-label={unread ? t('notifications.bellUnread', { count: unread }) : t('notifications.title')}
      className={cn('relative grid size-11 place-items-center rounded-full bg-card text-ink hover:bg-card-muted', className)}
    >
      <Bell className="size-5" aria-hidden />
      {unread > 0 && (
        <span className="absolute -top-0.5 -right-0.5 grid h-5 min-w-5 place-items-center rounded-full bg-[var(--danger)] px-1 text-[11px] font-semibold text-white tabular" aria-hidden>
          {unread > 9 ? '9+' : unread}
        </span>
      )}
    </Link>
  )
}

/** The reminders inbox (FR-REM-01/04). */
export function NotificationsPage() {
  const { t } = useTranslation()
  const list = useNotifications()
  const mark = useMarkNotificationsRead()
  const items = list.data?.items ?? []

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-5">
      <PageHeader
        title={t('notifications.title')}
        actions={
          (list.data?.unreadCount ?? 0) > 0 && (
            <Button variant="secondary" className="h-11" loading={mark.isPending} onClick={() => mark.mutate({ all: true })}>
              {t('notifications.markAll')}
            </Button>
          )
        }
      />
      {list.isError ? (
        <ErrorNotice error={list.error} onRetry={() => void list.refetch()} />
      ) : !list.data ? (
        <Skeleton className="h-48 rounded-card" />
      ) : items.length === 0 ? (
        <Card>
          <EmptyState icon={<BellOff className="size-10" />} title={t('notifications.noneTitle')} body={t('notifications.noneBody')} />
        </Card>
      ) : (
        <ul className="divide-y divide-line overflow-hidden rounded-card bg-card" data-testid="notifications">
          {items.map((n) => (
            <li key={n.id}>
              <NotificationRow n={n} onOpen={() => !n.read && mark.mutate({ ids: [n.id] })} />
            </li>
          ))}
        </ul>
      )}
      <p className="text-sm text-muted">
        {t('notifications.settingsHint')}{' '}
        <Link to="/settings" hash="reminders" className="font-semibold text-ink underline-offset-4 hover:underline">
          {t('notifications.settingsLink')}
        </Link>
      </p>
    </div>
  )
}

/** What the reminder says, e.g. "Due in 2 days · Rs 3,266". */
export function notificationText(n: Notification, t: ReturnType<typeof useTranslation>['t']): { title: string; line: string } {
  const what = n.periodKey ? formatMonth(n.periodKey) : (n.description ?? '')
  const title = [n.profileName, what].filter(Boolean).join(' · ')
  const amount = n.amountMinor != null ? formatMoney(n.amountMinor) : ''
  const due = n.dueDate ?? ''
  switch (n.kind) {
    case 'due_soon': {
      const days = daysUntil(due)
      return { title, line: t('notifications.dueSoon', { count: Math.max(days, 1), date: formatDayMonth(due), amount }) }
    }
    case 'due_today':
      return { title, line: t('notifications.dueToday', { amount }) }
    case 'overdue':
      return { title, line: t('notifications.overdue', { date: formatDayMonth(due), amount }) }
    case 'missing_bill':
      return { title, line: t('notifications.missing', { month: n.periodKey ? formatMonth(n.periodKey) : '' }) }
    case 'bill_created':
      return { title, line: due ? t('notifications.createdDue', { amount, date: formatDayMonth(due) }) : t('notifications.created', { amount }) }
  }
}

function NotificationRow({ n, onOpen }: { n: Notification; onOpen: () => void }) {
  const { t } = useTranslation()
  const Icon = ICONS[n.kind]
  const { title, line } = notificationText(n, t)
  const body = (
    <>
      <span
        className={cn(
          'grid size-10 shrink-0 place-items-center rounded-tile',
          n.kind === 'overdue' ? 'bg-[var(--st-overdue-bg)] text-[var(--st-overdue-fg)]' : n.kind === 'missing_bill' ? 'bg-[var(--st-soon-bg)] text-[var(--st-soon-fg)]' : 'bg-card-muted text-ink',
        )}
      >
        <Icon className="size-5" aria-hidden />
      </span>
      <span className="flex min-w-0 flex-1 flex-col">
        <span className={cn('truncate text-sm text-ink', !n.read && 'font-semibold')}>{title}</span>
        <span className="text-sm text-body">{line}</span>
        <span className="text-xs text-muted">{relativeTime(n.createdAt, t)}</span>
      </span>
      {!n.read && <span className="mt-1.5 size-2.5 shrink-0 rounded-full bg-[var(--danger)]" aria-label={t('notifications.unread')} />}
    </>
  )
  const cls = cn('flex items-start gap-3 px-4 py-3 transition hover:bg-card-muted/60 sm:px-5', !n.read && 'bg-card-muted/30')
  if (n.kind === 'missing_bill' && n.profileId)
    return (
      <Link to="/bills/new" search={{ profileId: n.profileId, period: n.periodKey ?? undefined }} onClick={onOpen} className={cls}>
        {body}
      </Link>
    )
  if (n.billId)
    return (
      <Link to="/bills/$billId" params={{ billId: n.billId }} onClick={onOpen} className={cls}>
        {body}
      </Link>
    )
  return (
    <button type="button" onClick={onOpen} className={cn(cls, 'w-full text-left')}>
      {body}
    </button>
  )
}
