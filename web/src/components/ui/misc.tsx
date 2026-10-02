import { AlertTriangle, Check, CircleDashed, Clock, Minus, PenLine } from 'lucide-react'
import type { HTMLAttributes, ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import type { Bill } from '@/api/client'
import { cn } from '@/lib/cn'
import { NamedIcon } from '@/lib/icons'
import { statusTone, type StatusTone } from '@/lib/status'

export function Card({ className, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('rounded-card bg-card p-5 sm:p-6', className)} {...rest} />
}

/** The dark featured card — at most one per screen (docs/10 §5). */
export function FeaturedCard({ className, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('rounded-card bg-featured p-6 text-on-featured', className)} {...rest} />
}

const toneStyle: Record<StatusTone, { fg: string; bg: string; Icon: typeof Check }> = {
  paid: { fg: '--st-paid-fg', bg: '--st-paid-bg', Icon: Check },
  unpaid: { fg: '--st-unpaid-fg', bg: '--st-unpaid-bg', Icon: CircleDashed },
  dueSoon: { fg: '--st-soon-fg', bg: '--st-soon-bg', Icon: Clock },
  overdue: { fg: '--st-overdue-fg', bg: '--st-overdue-bg', Icon: AlertTriangle },
  partial: { fg: '--st-partial-fg', bg: '--st-partial-bg', Icon: CircleDashed },
  review: { fg: '--st-review-fg', bg: '--st-review-bg', Icon: PenLine },
  cancelled: { fg: '--st-cancel-fg', bg: '--st-cancel-bg', Icon: Minus },
}

/** Icon + word + colour — status is never colour alone. */
export function StatusBadge({ bill, tone: forced }: { bill?: Pick<Bill, 'status' | 'dueDate'>; tone?: StatusTone }) {
  const { t } = useTranslation()
  const tone = forced ?? (bill ? statusTone(bill) : 'unpaid')
  const { fg, bg, Icon } = toneStyle[tone]
  return (
    <span
      className="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-semibold whitespace-nowrap"
      style={{ color: `var(${fg})`, backgroundColor: `var(${bg})` }}
    >
      <Icon className="size-3.5" aria-hidden strokeWidth={2.25} />
      {t(`status.${tone}`)}
    </span>
  )
}

/** Profile icon on a 16% tint of its palette colour. */
export function IconTile({ icon, color, size = 'md' }: { icon: string; color: number; size?: 'sm' | 'md' | 'lg' }) {
  const dims = size === 'lg' ? 'size-14' : size === 'sm' ? 'size-9' : 'size-11'
  const iconSize = size === 'lg' ? 'size-7' : size === 'sm' ? 'size-4.5' : 'size-5'
  return (
    <span
      className={cn('grid shrink-0 place-items-center rounded-tile text-ink', dims)}
      style={{ backgroundColor: `color-mix(in srgb, var(--p${color}) 16%, transparent)` }}
    >
      <NamedIcon name={icon} className={iconSize} style={{ color: `var(--p${color})` }} />
    </span>
  )
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cn('animate-pulse rounded-tile bg-card-muted', className)} aria-hidden />
}

export function EmptyState({ icon, title, body, action }: { icon?: ReactNode; title: string; body?: string; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-3 px-4 py-10 text-center">
      {icon && <div className="text-muted">{icon}</div>}
      <h3 className="text-lg font-semibold">{title}</h3>
      {body && <p className="max-w-sm text-sm text-muted">{body}</p>}
      {action && <div className="mt-2">{action}</div>}
    </div>
  )
}

export function Stat({ label, value, sub }: { label: string; value: ReactNode; sub?: ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs font-medium text-muted">{label}</span>
      <span className="tabular text-2xl font-medium text-ink">{value}</span>
      {sub && <span className="text-xs text-body">{sub}</span>}
    </div>
  )
}

export function SectionTitle({ children, action }: { children: ReactNode; action?: ReactNode }) {
  return (
    <div className="mb-3 flex items-center justify-between gap-2">
      <h2 className="text-lg font-semibold">{children}</h2>
      {action}
    </div>
  )
}

/** Pill chip used for filters and segmented choices. */
export function Chip({ selected, className, children, ...rest }: { selected?: boolean } & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      aria-pressed={selected}
      className={cn(
        'inline-flex h-10 shrink-0 items-center gap-1.5 rounded-full px-4 text-sm font-semibold transition',
        selected ? 'bg-featured text-on-featured' : 'bg-card-muted text-ink hover:brightness-95',
        className,
      )}
      {...rest}
    >
      {selected && <Check className="size-4" aria-hidden />}
      {children}
    </button>
  )
}

export function ErrorNotice({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const { t } = useTranslation()
  const message = error instanceof Error ? error.message : t('common.errorTitle')
  return (
    <div role="alert" className="flex flex-col items-start gap-3 rounded-card bg-[var(--st-overdue-bg)] p-5 text-[var(--st-overdue-fg)]">
      <p className="flex items-center gap-2 font-semibold">
        <AlertTriangle className="size-5" aria-hidden />
        {message}
      </p>
      {onRetry && (
        <button type="button" onClick={onRetry} className="text-sm font-semibold underline">
          {t('common.retry')}
        </button>
      )}
    </div>
  )
}
