import { Link, useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { Archive, LayoutGrid, Plus, RotateCcw } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { BillDetail, Profile } from '@/api/client'
import { useArchiveProfile, useBill, useCreateProfile, useProfile, useProfiles, useTypes, useUpdateProfile } from '@/api/hooks'
import { PageHeader } from '@/components/layout/app-shell'
import { Button } from '@/components/ui/button'
import { Toggle } from '@/components/ui/field'
import { Card, Chip, EmptyState, ErrorNotice, FeaturedCard, IconTile, SectionTitle, Skeleton, Stat, StatusBadge } from '@/components/ui/misc'
import { Sheet } from '@/components/ui/sheet'
import { cn } from '@/lib/cn'
import { formatDate, formatDayMonth } from '@/lib/dates'
import { formatMoney } from '@/lib/money'
import { isOpen } from '@/lib/status'
import { BackLink } from '../bills/bill-pages'
import { billTitle, BillsList } from '../bills/bill-row'
import { PaymentSheet } from '../bills/payment-sheet'
import { ProfileForm } from './profile-form'
import { HistoryCard } from './history-card'

// --- Profile tile ----------------------------------------------------------------------

export function ProfileTile({ profile }: { profile: Profile }) {
  const { t } = useTranslation()
  const latest = profile.summary.latestBill
  const meta = (() => {
    if (profile.kind === 'expense_log') return t('profiles.entries', { count: profile.summary.billCount })
    if (!latest) return t('profiles.noBills')
    const due = latest.dueDate && isOpen(latest) ? ` · ${t('bills.due', { date: formatDayMonth(latest.dueDate) })}` : ''
    return `${billTitle(latest)}${due}`
  })()
  return (
    <Link
      to="/profiles/$profileId"
      params={{ profileId: profile.id }}
      className={cn('flex items-center gap-3 rounded-card bg-card p-4 transition hover:brightness-[0.98]', profile.archived && 'opacity-60')}
    >
      <IconTile icon={profile.icon} color={profile.color} />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate font-medium text-ink">{profile.name}</span>
        <span className="truncate text-sm text-muted">{meta}</span>
      </span>
      {latest && (
        <span className="flex flex-col items-end gap-1">
          <span className="tabular font-medium text-ink">
            {formatMoney(profile.kind === 'expense_log' ? profile.summary.thisYearMinor : isOpen(latest) ? latest.outstandingMinor : latest.payableMinor, latest.currency)}
          </span>
          {profile.kind !== 'expense_log' && <StatusBadge bill={latest} />}
        </span>
      )}
    </Link>
  )
}

// --- List (S9) --------------------------------------------------------------------------

type GroupFilter = 'all' | 'utilities' | 'household' | 'other'

export function ProfilesPage() {
  const { t } = useTranslation()
  const [group, setGroup] = useState<GroupFilter>('all')
  const [showArchived, setShowArchived] = useState(false)
  const profiles = useProfiles(showArchived)
  const { data: types } = useTypes()
  const typeById = useMemo(() => new Map((types ?? []).map((ty) => [ty.id, ty])), [types])

  const sections = useMemo(() => {
    const visible = (profiles.data ?? []).filter((p) => group === 'all' || typeById.get(p.typeId)?.group === group)
    const byType = new Map<string, Profile[]>()
    for (const p of visible) byType.set(p.typeId, [...(byType.get(p.typeId) ?? []), p])
    return [...byType.entries()]
      .map(([typeId, items]) => ({ type: typeById.get(typeId), items }))
      .sort((a, b) => (a.items[0]?.sortOrder ?? 0) - (b.items[0]?.sortOrder ?? 0))
  }, [profiles.data, typeById, group])

  return (
    <div className="flex flex-col gap-5">
      <PageHeader
        title={t('profiles.title')}
        actions={
          <Link to="/profiles/new" className="inline-flex h-10 items-center gap-2 rounded-full bg-featured px-4 text-sm font-semibold text-on-featured">
            <Plus className="size-4" aria-hidden /> {t('profiles.new')}
          </Link>
        }
      />
      <div className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 sm:mx-0 sm:px-0" role="group">
        {(['all', 'utilities', 'household', 'other'] as const).map((g) => (
          <Chip key={g} selected={group === g} onClick={() => setGroup(g)}>
            {g === 'all' ? t('profiles.all') : t(`group.${g}`)}
          </Chip>
        ))}
      </div>

      {profiles.isPending ? (
        <div className="grid gap-3 sm:grid-cols-2">
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-20 rounded-card" />
          ))}
        </div>
      ) : profiles.isError ? (
        <ErrorNotice error={profiles.error} onRetry={() => void profiles.refetch()} />
      ) : sections.length === 0 ? (
        <Card>
          <EmptyState
            icon={<LayoutGrid className="size-10" />}
            title={t('profiles.emptyTitle')}
            body={t('profiles.emptyBody')}
            action={
              <Link to="/profiles/new" className="inline-flex h-12 items-center rounded-full bg-featured px-6 text-sm font-semibold text-on-featured">
                {t('profiles.new')}
              </Link>
            }
          />
        </Card>
      ) : (
        sections.map(({ type, items }) => (
          <section key={type?.id ?? 'unknown'}>
            <h2 className="mb-2 px-1 text-xs font-semibold tracking-wide text-muted">
              {(type?.name ?? '').toUpperCase()} · {items.length}
            </h2>
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
              {items.map((p) => (
                <ProfileTile key={p.id} profile={p} />
              ))}
            </div>
          </section>
        ))
      )}
      <Toggle label={t('profiles.showArchived')} checked={showArchived} onChange={setShowArchived} />
    </div>
  )
}

// --- Create (S16) ------------------------------------------------------------------------

export function NewProfilePage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { typeId } = useSearch({ from: '/app/profiles/new' })
  const create = useCreateProfile()
  return (
    <div className="mx-auto max-w-2xl">
      <PageHeader back={<BackLink to="/profiles" label={t('profiles.title')} />} title={t('profiles.createTitle')} />
      <Card>
        <ProfileForm
          defaultTypeId={typeId}
          submitLabel={t('common.save')}
          onCancel={() => window.history.back()}
          onSubmit={async (input) => {
            const p = await create.mutateAsync(input)
            toast.success(t('profiles.created'))
            await navigate({ to: '/profiles/$profileId', params: { profileId: p.id } })
          }}
        />
      </Card>
    </div>
  )
}

// --- Profile dashboard (S10) ----------------------------------------------------------------

type Tab = 'overview' | 'bills' | 'settings'

export function ProfilePage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { profileId } = useParams({ from: '/app/profiles/$profileId' })
  const { tab = 'overview' } = useSearch({ from: '/app/profiles/$profileId' })
  const profile = useProfile(profileId)
  const { data: types } = useTypes()

  if (profile.isError) return <ErrorNotice error={profile.error} onRetry={() => void profile.refetch()} />
  if (!profile.data) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-14 w-64" />
        <Skeleton className="h-48 w-full rounded-card" />
      </div>
    )
  }
  const p = profile.data
  const type = types?.find((ty) => ty.id === p.typeId)
  const setTab = (next: Tab) => void navigate({ to: '/profiles/$profileId', params: { profileId }, search: { tab: next === 'overview' ? undefined : next }, replace: true })

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-col gap-4">
        <BackLink to="/profiles" label={t('profiles.title')} />
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex min-w-0 items-center gap-4">
            <IconTile icon={p.icon} color={p.color} size="lg" />
            <div className="min-w-0">
              <h1 className="truncate text-2xl font-semibold sm:text-[28px]">{p.name}</h1>
              <p className="truncate text-sm text-muted">
                {[type?.name, t(`kind.${p.kind}`), p.referenceNumber].filter(Boolean).join(' · ')}
              </p>
            </div>
          </div>
          {!p.archived && (
            <Link
              to="/bills/new"
              search={{ profileId: p.id }}
              className="inline-flex h-11 items-center gap-2 rounded-full bg-featured px-5 text-sm font-semibold text-on-featured"
            >
              <Plus className="size-4" aria-hidden /> {p.kind === 'expense_log' ? t('profiles.addEntry') : t('profiles.addBill')}
            </Link>
          )}
        </div>
        <div className="sticky top-0 z-10 -mx-4 flex gap-2 overflow-x-auto bg-bg/95 px-4 py-2 backdrop-blur sm:mx-0 sm:px-0" role="tablist">
          {(['overview', 'bills', 'settings'] as const).map((key) => (
            <button
              key={key}
              type="button"
              role="tab"
              aria-selected={tab === key}
              onClick={() => setTab(key)}
              className={cn(
                'h-10 shrink-0 rounded-full px-4 text-sm font-semibold transition',
                tab === key ? 'bg-featured text-on-featured' : 'text-body hover:bg-card-muted',
              )}
            >
              {t(`profiles.tabs.${key}`)}
            </button>
          ))}
        </div>
      </header>

      {p.archived && <Card className="bg-card-muted text-sm font-medium">{t('profiles.archivedBanner')}</Card>}

      {tab === 'overview' && <ProfileOverview profile={p} onSeeAll={() => setTab('bills')} />}
      {tab === 'bills' && (
        <BillsList filters={{ profileId: p.id }} empty={<ProfileEmpty profile={p} />} />
      )}
      {tab === 'settings' && <ProfileSettings profile={p} />}
    </div>
  )
}

function ProfileEmpty({ profile }: { profile: Profile }) {
  const { t } = useTranslation()
  return (
    <Card>
      <EmptyState
        title={t('profiles.emptyProfileTitle', { name: profile.name })}
        action={
          !profile.archived && (
            <Link to="/bills/new" search={{ profileId: profile.id }} className="inline-flex h-12 items-center rounded-full bg-featured px-6 text-sm font-semibold text-on-featured">
              {t('profiles.firstBill')}
            </Link>
          )
        }
      />
    </Card>
  )
}

function ProfileOverview({ profile, onSeeAll }: { profile: Profile; onSeeAll: () => void }) {
  const { t } = useTranslation()
  const latest = profile.summary.latestBill
  const s = profile.summary
  if (s.billCount === 0) return <ProfileEmpty profile={profile} />
  const currency = latest?.currency ?? 'PKR'

  return (
    <div className="grid gap-5 lg:grid-cols-12">
      {latest && profile.kind !== 'expense_log' && (
        <div className="lg:col-span-5">
          <LatestBillCard billId={latest.id} />
        </div>
      )}
      <Card className={cn('grid grid-cols-2 gap-5 sm:grid-cols-3', profile.kind === 'expense_log' ? 'lg:col-span-12' : 'lg:col-span-7')}>
        <Stat label={t('profiles.thisYear')} value={formatMoney(s.thisYearMinor, currency)} />
        <Stat label={t('profiles.outstanding')} value={formatMoney(s.outstandingMinor, currency)} />
        <Stat label={t('profiles.billCount')} value={s.billCount} />
      </Card>
      {profile.kind !== 'expense_log' && (
        <div className="lg:col-span-12">
          <HistoryCard profileId={profile.id} />
        </div>
      )}
      <section className="lg:col-span-12">
        <SectionTitle
          action={
            <button type="button" onClick={onSeeAll} className="text-sm font-semibold text-ink underline-offset-4 hover:underline">
              {t('common.seeAll')}
            </button>
          }
        >
          {t('profiles.recentBills')}
        </SectionTitle>
        <BillsList filters={{ profileId: profile.id, limit: 5 }} />
      </section>
    </div>
  )
}

/** The dark featured card for the profile's latest bill, with a quick "Mark as paid". */
function LatestBillCard({ billId }: { billId: string }) {
  const { t } = useTranslation()
  const bill = useBill(billId)
  const [payOpen, setPayOpen] = useState(false)
  if (!bill.data) return <Skeleton className="h-52 rounded-card" />
  const b: BillDetail = bill.data
  return (
    <FeaturedCard className="flex h-full flex-col">
      <div className="flex items-center justify-between gap-3">
        <span className="text-xs font-semibold tracking-wide text-on-featured/70">{billTitle(b).toUpperCase()}</span>
        <StatusBadge bill={b} />
      </div>
      <p className="tabular mt-5 text-[34px] leading-tight font-medium sm:text-amount-hero">
        {formatMoney(isOpen(b) ? b.outstandingMinor : b.payableMinor, b.currency)}
      </p>
      <p className="mt-1 text-sm text-on-featured/70">{b.dueDate ? t('bills.due', { date: formatDate(b.dueDate) }) : formatDate(b.expenseDate)}</p>
      <div className="mt-auto flex flex-wrap gap-2 pt-5">
        {isOpen(b) && (
          <Button variant="accent" size="sm" onClick={() => setPayOpen(true)}>
            {t('bills.markPaid')}
          </Button>
        )}
        <Link to="/bills/$billId" params={{ billId: b.id }} className="inline-flex h-10 items-center rounded-full border border-white/60 px-4 text-sm font-semibold">
          {t('common.view')}
        </Link>
      </div>
      <PaymentSheet bill={b} open={payOpen} onOpenChange={setPayOpen} />
    </FeaturedCard>
  )
}

function ProfileSettings({ profile }: { profile: Profile }) {
  const { t } = useTranslation()
  const update = useUpdateProfile(profile.id)
  const archive = useArchiveProfile()
  const [confirm, setConfirm] = useState(false)

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-5">
      <Card>
        <ProfileForm
          key={profile.id}
          profile={profile}
          submitLabel={t('common.save')}
          onSubmit={async (input) => {
            await update.mutateAsync(input)
            toast.success(t('profiles.saved'))
          }}
        />
      </Card>
      {!profile.isDefault &&
        (profile.archived ? (
          <Button
            variant="secondary"
            icon={<RotateCcw className="size-4" />}
            onClick={async () => {
              await archive.mutateAsync({ id: profile.id, restore: true })
              toast.success(t('profiles.restoredToast'))
            }}
          >
            {t('common.restore')}
          </Button>
        ) : (
          <Button variant="outline" icon={<Archive className="size-4" />} onClick={() => setConfirm(true)}>
            {t('common.archive')}
          </Button>
        ))}
      <Sheet
        open={confirm}
        onOpenChange={setConfirm}
        title={t('profiles.archiveConfirmTitle')}
        description={t('profiles.archiveConfirmBody')}
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirm(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              loading={archive.isPending}
              onClick={async () => {
                await archive.mutateAsync({ id: profile.id })
                setConfirm(false)
                toast.success(t('profiles.archivedToast'))
              }}
            >
              {t('common.archive')}
            </Button>
          </>
        }
      >
        {null}
      </Sheet>
    </div>
  )
}
