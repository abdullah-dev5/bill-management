import { Link, useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { Archive, ArrowLeft, Paperclip, Pencil, RotateCcw, Trash2, X } from 'lucide-react'
import { useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { BillStatus, Profile } from '@/api/client'
import { useBill, useBillAction, useCreateBill, useDeletePayment, useProfile, useProfiles, useUpdateBill } from '@/api/hooks'
import { PageHeader } from '@/components/layout/app-shell'
import { DocumentsSection, DocumentViewer } from '@/features/documents/documents-section'
import { Button } from '@/components/ui/button'
import { Toggle } from '@/components/ui/field'
import { Card, Chip, ErrorNotice, FeaturedCard, IconTile, SectionTitle, Skeleton, StatusBadge } from '@/components/ui/misc'
import { Sheet } from '@/components/ui/sheet'
import { Select } from '@/components/ui/field'
import { formatDate } from '@/lib/dates'
import { formatMoney } from '@/lib/money'
import { isOpen } from '@/lib/status'
import { BillForm } from './bill-form'
import { billTitle, BillsList } from './bill-row'
import { PaymentSheet } from './payment-sheet'

export function BackLink({ to, params, label }: { to: string; params?: Record<string, string>; label: string }) {
  return (
    <Link to={to} params={params} className="flex items-center gap-1 self-start text-sm font-semibold text-body hover:text-ink">
      <ArrowLeft className="size-4" aria-hidden /> {label}
    </Link>
  )
}

function PageSkeleton() {
  return (
    <div className="flex flex-col gap-4">
      <Skeleton className="h-8 w-48" />
      <Skeleton className="h-40 w-full rounded-card" />
      <Skeleton className="h-64 w-full rounded-card" />
    </div>
  )
}

// --- New / edit ------------------------------------------------------------------

export function NewBillPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { profileId } = useSearch({ from: '/app/bills/new' })
  const { data: profiles } = useProfiles()
  const create = useCreateBill()
  const profile = profiles?.find((p) => p.id === profileId) ?? profiles?.find((p) => p.isDefault)

  if (!profiles) return <PageSkeleton />
  if (!profile) return <ErrorNotice error={new Error(t('common.notFound'))} />

  return (
    <div className="mx-auto max-w-2xl">
      <PageHeader
        back={<BackLink to="/profiles/$profileId" params={{ profileId: profile.id }} label={profile.name} />}
        title={profile.kind === 'expense_log' ? t('bills.newEntry') : t('bills.newBill')}
      >
        <ProfileSwitcher profiles={profiles} current={profile} onChange={(id) => void navigate({ to: '/bills/new', search: { profileId: id }, replace: true })} />
      </PageHeader>
      <Card>
        <BillForm
          key={profile.id}
          profile={profile}
          onCancel={() => window.history.back()}
          onSubmit={async (body) => {
            const bill = await create.mutateAsync(body)
            toast.success(t('bills.created'))
            await navigate({ to: '/profiles/$profileId', params: { profileId: profile.id } })
            return bill
          }}
        />
      </Card>
    </div>
  )
}

function ProfileSwitcher({ profiles, current, onChange }: { profiles: Profile[]; current: Profile; onChange: (id: string) => void }) {
  const { t } = useTranslation()
  return (
    <div className="flex items-center gap-3">
      <IconTile icon={current.icon} color={current.color} size="sm" />
      <label className="sr-only" htmlFor="profile-switch">
        {t('bills.profile')}
      </label>
      <Select id="profile-switch" value={current.id} onChange={(e) => onChange(e.target.value)} className="h-11 max-w-xs">
        {profiles.map((p) => (
          <option key={p.id} value={p.id}>
            {p.name}
          </option>
        ))}
      </Select>
    </div>
  )
}

export function EditBillPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { billId } = useParams({ from: '/app/bills/$billId/edit' })
  const bill = useBill(billId)
  const profile = useProfile(bill.data?.profileId ?? '')
  const update = useUpdateBill(billId)

  if (bill.isError) return <ErrorNotice error={bill.error} />
  if (!bill.data || !profile.data) return <PageSkeleton />

  return (
    <div className="mx-auto max-w-2xl">
      <PageHeader back={<BackLink to="/bills/$billId" params={{ billId }} label={billTitle(bill.data)} />} title={t('bills.editBill')} />
      <Card>
        <BillForm
          profile={profile.data}
          bill={bill.data}
          onCancel={() => void navigate({ to: '/bills/$billId', params: { billId } })}
          onSubmit={async (body) => {
            const { paidNow: _ignored, ...input } = body
            await update.mutateAsync(input)
            toast.success(t('bills.updated'))
            await navigate({ to: '/bills/$billId', params: { billId } })
          }}
        />
      </Card>
    </div>
  )
}

// --- Detail ------------------------------------------------------------------------

export function BillDetailPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { billId } = useParams({ from: '/app/bills/$billId' })
  const bill = useBill(billId)
  const profile = useProfile(bill.data?.profileId ?? '')
  const action = useBillAction()
  const deletePayment = useDeletePayment()
  const [payOpen, setPayOpen] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [paymentToDelete, setPaymentToDelete] = useState<string>()
  const linksRefreshedAt = useRef(0)
  const [receiptId, setReceiptId] = useState<string>()

  if (bill.isError) return <ErrorNotice error={bill.error} onRetry={() => void bill.refetch()} />
  if (!bill.data) return <PageSkeleton />
  const b = bill.data
  const p = profile.data
  const money = (v: number) => formatMoney(v, b.currency)

  const run = async (kind: 'archive' | 'restore' | 'delete') => {
    try {
      await action.mutateAsync({ id: b.id, action: kind })
      if (kind === 'archive') {
        toast.success(t('bills.archivedToast'), {
          action: { label: t('common.undo'), onClick: () => void action.mutateAsync({ id: b.id, action: 'restore' }) },
        })
      } else if (kind === 'restore') {
        toast.success(t('bills.restoredToast'))
      } else {
        toast.success(t('bills.deletedToast'))
        await navigate({ to: '/profiles/$profileId', params: { profileId: b.profileId } })
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errorTitle'))
    }
  }

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-5">
      <PageHeader
        back={p && <BackLink to="/profiles/$profileId" params={{ profileId: p.id }} label={p.name} />}
        title={billTitle(b)}
        actions={
          !b.archived && (
            <>
              <Link to="/bills/$billId/edit" params={{ billId: b.id }} className="inline-flex h-10 items-center gap-2 rounded-full bg-card-muted px-4 text-sm font-semibold text-ink">
                <Pencil className="size-4" aria-hidden /> {t('common.edit')}
              </Link>
              <Button variant="ghost" size="sm" icon={<Archive className="size-4" />} onClick={() => void run('archive')}>
                {t('common.archive')}
              </Button>
            </>
          )
        }
      />

      {b.archived && (
        <Card className="flex flex-wrap items-center justify-between gap-3 bg-card-muted">
          <p className="text-sm font-medium">{t('bills.archivedBanner')}</p>
          <div className="flex gap-2">
            <Button size="sm" variant="secondary" icon={<RotateCcw className="size-4" />} onClick={() => void run('restore')}>
              {t('common.restore')}
            </Button>
            <Button size="sm" variant="danger" icon={<Trash2 className="size-4" />} onClick={() => setConfirmDelete(true)}>
              {t('common.delete')}
            </Button>
          </div>
        </Card>
      )}
      {b.cancelled && <Card className="bg-card-muted text-sm font-medium">{t('bills.cancelledBanner')}</Card>}
      {b.settledByBillId && (
        <Card className="flex flex-wrap items-center justify-between gap-3 bg-[var(--st-paid-bg)] text-[var(--st-paid-fg)]">
          <p className="text-sm font-medium">{t('bills.settledBanner')}</p>
          <Link to="/bills/$billId" params={{ billId: b.settledByBillId }} className="text-sm font-semibold underline">
            {t('bills.viewLaterBill')}
          </Link>
        </Card>
      )}

      <FeaturedCard>
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-center gap-3">
            {p && <IconTile icon={p.icon} color={p.color} />}
            <div className="min-w-0">
              <p className="truncate font-medium">{p?.name}</p>
              {p?.referenceNumber && <p className="tabular text-sm text-on-featured/70">{p.referenceNumber}</p>}
            </div>
          </div>
          <StatusBadge bill={b} />
        </div>
        <p className="tabular mt-6 text-[34px] leading-tight font-medium sm:text-amount-hero">
          {money(isOpen(b) ? b.outstandingMinor : b.payableMinor)}
        </p>
        <p className="mt-1 text-sm text-on-featured/70">
          {b.dueDate ? t('bills.due', { date: formatDate(b.dueDate) }) : formatDate(b.expenseDate)}
          {b.payableAfterDueMinor != null && ` · ${t('bills.payableAfterDue')}: ${money(b.payableAfterDueMinor)}`}
        </p>
        {isOpen(b) && !b.archived && (
          <div className="mt-5 flex flex-wrap gap-2">
            <Button variant="accent" onClick={() => setPayOpen(true)}>
              {t('bills.markPaid')}
            </Button>
          </div>
        )}
      </FeaturedCard>

      <Card>
        <SectionTitle>{t('bills.breakdown')}</SectionTitle>
        <dl className="tabular divide-y divide-line text-sm">
          {b.description && b.periodKey && <Row label={t('bills.description')} value={b.description} />}
          <Row label={b.periodKey ? t('bills.currentCharges') : t('bills.amount')} value={money(b.currentChargesMinor)} />
          {b.arrearsMinor !== 0 && <Row label={t('bills.arrears')} value={money(b.arrearsMinor)} />}
          {b.adjustmentsMinor !== 0 && <Row label={t('bills.adjustments')} value={money(b.adjustmentsMinor)} />}
          {b.periodKey && <Row label={t('bills.payable')} value={money(b.payableMinor)} strong />}
          {b.payableAfterDueMinor != null && (
            <Row
              label={b.lateStepUntil ? t('bills.afterDueUntil', { date: formatDate(b.lateStepUntil) }) : t('bills.payableAfterDue')}
              value={money(b.payableAfterDueMinor)}
            />
          )}
          {b.lateStepUntil && b.payableFinalMinor != null && (
            <Row label={t('bills.afterDate', { date: formatDate(b.lateStepUntil) })} value={money(b.payableFinalMinor)} />
          )}
          <Row label={t('bills.paidLabel')} value={money(b.paidMinor)} />
          {isOpen(b) && <Row label={t('bills.outstanding')} value={money(b.outstandingMinor)} strong />}
          {b.units != null && (
            <Row
              label={t('bills.consumption')}
              value={
                (b.previousReading != null && b.currentReading != null ? `${fmtNum(b.previousReading)} → ${fmtNum(b.currentReading)} · ` : '') +
                `${fmtNum(b.units)} ${p?.consumptionUnit || t('bills.unitsDefault')}` +
                (b.multiplyingFactor && b.multiplyingFactor !== 1 ? ` (×${fmtNum(b.multiplyingFactor)})` : '')
              }
            />
          )}
          {b.meterReplaced && <Row label={t('bills.meterReplaced')} value={t('common.yes')} />}
          {b.isEstimated && <Row label={t('bills.estimated')} value={t('common.yes')} />}
          {b.issueDate && <Row label={t('bills.issueDate')} value={formatDate(b.issueDate)} />}
          {b.billNumber && <Row label={t('bills.billNumber')} value={b.billNumber} />}
          {b.notes && <Row label={t('bills.notes')} value={b.notes} />}
        </dl>
      </Card>

      <Card>
        <SectionTitle
          action={
            isOpen(b) && !b.archived ? (
              <Button size="sm" variant="secondary" onClick={() => setPayOpen(true)}>
                {t('bills.addPayment')}
              </Button>
            ) : undefined
          }
        >
          {t('bills.payments')}
        </SectionTitle>
        {b.payments.length === 0 ? (
          <p className="text-sm text-muted">{t('bills.noPayments')}</p>
        ) : (
          <ul className="divide-y divide-line">
            {b.payments.map((pm) => (
              <li key={pm.id} className="flex items-center gap-3 py-3">
                <span className="flex flex-1 flex-col">
                  <span className="font-medium text-ink">{formatDate(pm.paidOn)}</span>
                  <span className="text-sm text-muted">
                    {t(`method.${pm.method}`)}
                    {pm.reference && ` · ${pm.reference}`}
                  </span>
                </span>
                <span className="tabular font-medium text-ink">{money(pm.amountMinor)}</span>
                {pm.receiptDocumentId && (
                  <Button size="icon" variant="ghost" aria-label={t('bills.viewReceipt')} onClick={() => setReceiptId(pm.receiptDocumentId!)}>
                    <Paperclip className="size-4" />
                  </Button>
                )}
                {!b.archived && (
                  <Button size="icon" variant="ghost" aria-label={t('common.delete')} onClick={() => setPaymentToDelete(pm.id)}>
                    <X className="size-4" />
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
      </Card>

      <DocumentsSection
        billId={b.id}
        documents={b.documents}
        readOnly={b.archived}
        onLinksExpired={() => {
          // Signed links last 5 minutes; refresh them at most once a minute.
          if (Date.now() - linksRefreshedAt.current > 60_000) {
            linksRefreshedAt.current = Date.now()
            void bill.refetch()
          }
        }}
      />

      <Card>
        <SectionTitle>{t('bills.history')}</SectionTitle>
        <ol className="flex flex-col gap-3 text-sm">
          {b.history.map((h) => (
            <li key={h.id} className="flex gap-3">
              <span className="mt-1.5 size-2 shrink-0 rounded-full bg-line" aria-hidden />
              <span>
                <span className="font-medium text-ink">{h.actorName ?? t('bills.someone')}</span>{' '}
                {t(`bills.actions.${h.action as 'update'}`, { defaultValue: h.action })}
                <span className="block text-muted">{new Date(h.at).toLocaleString('en-GB', { dateStyle: 'medium', timeStyle: 'short' })}</span>
                {h.action === 'update' && <ChangeList changes={h.changes} />}
                {(h.action === 'attach_document' || h.action === 'remove_document') && (
                  <span className="block text-muted">{documentName(h.changes, 'document')}</span>
                )}
                {(h.action === 'settled' || h.action === 'unsettled') && (
                  <span className="block text-muted">{t('bills.viaBill', { bill: documentName(h.changes, 'settledBy') })}</span>
                )}
              </span>
            </li>
          ))}
        </ol>
      </Card>

      <PaymentSheet bill={b} open={payOpen} onOpenChange={setPayOpen} />
      <DocumentViewer id={receiptId} billId={b.id} readOnly={b.archived} onClose={() => setReceiptId(undefined)} />

      <Sheet
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title={t('bills.deleteConfirmTitle')}
        description={t('bills.deleteConfirmBody')}
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirmDelete(false)}>
              {t('common.cancel')}
            </Button>
            <Button variant="danger" loading={action.isPending} onClick={() => void run('delete')}>
              {t('common.delete')}
            </Button>
          </>
        }
      >
        {null}
      </Sheet>

      <Sheet
        open={paymentToDelete != null}
        onOpenChange={(o) => !o && setPaymentToDelete(undefined)}
        title={t('bills.deletePaymentConfirm')}
        footer={
          <>
            <Button variant="secondary" onClick={() => setPaymentToDelete(undefined)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="danger"
              loading={deletePayment.isPending}
              onClick={async () => {
                if (!paymentToDelete) return
                await deletePayment.mutateAsync(paymentToDelete)
                setPaymentToDelete(undefined)
                toast.success(t('bills.paymentDeleted'))
              }}
            >
              {t('common.delete')}
            </Button>
          </>
        }
      >
        {null}
      </Sheet>
    </div>
  )
}

const fmtNum = (n: number) => n.toLocaleString('en-PK', { maximumFractionDigits: 2 })

function Row({ label, value, strong }: { label: string; value: string; strong?: boolean }) {
  return (
    <div className="flex justify-between gap-4 py-2.5">
      <dt className="text-muted">{label}</dt>
      <dd className={strong ? 'font-semibold text-ink' : 'text-ink'}>{value}</dd>
    </div>
  )
}

function documentName(changes: Record<string, unknown>, key: string): string {
  const pair = changes[key]
  return Array.isArray(pair) ? String(pair.find((v) => typeof v === 'string') ?? '') : ''
}

function ChangeList({ changes }: { changes: Record<string, unknown> }) {
  const entries = Object.entries(changes)
  if (entries.length === 0) return null
  const show = (v: unknown, field: string) =>
    v == null || v === '' ? '—' : field.endsWith('Minor') && typeof v === 'number' ? formatMoney(v) : String(v)
  return (
    <ul className="mt-1 text-muted">
      {entries.map(([field, pair]) => {
        const [from, to] = Array.isArray(pair) ? pair : [undefined, pair]
        return (
          <li key={field}>
            {field}: {show(from, field)} → {show(to, field)}
          </li>
        )
      })}
    </ul>
  )
}

// --- All bills -------------------------------------------------------------------

const STATUS_FILTERS: { key: string; statuses: BillStatus[] }[] = [
  { key: 'all', statuses: [] },
  { key: 'unpaid', statuses: ['unpaid', 'partially_paid', 'overdue'] },
  { key: 'overdue', statuses: ['overdue'] },
  { key: 'paid', statuses: ['paid'] },
]

export function BillsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const search = useSearch({ from: '/app/bills' })
  const { data: profiles } = useProfiles(true)
  const byId = useMemo(() => new Map((profiles ?? []).map((p) => [p.id, p])), [profiles])
  const statusKey = search.status ?? 'all'
  const statuses = STATUS_FILTERS.find((f) => f.key === statusKey)?.statuses ?? []
  const set = (patch: Partial<typeof search>) => void navigate({ to: '/bills', search: { ...search, ...patch }, replace: true })

  return (
    <div className="flex flex-col gap-5">
      <PageHeader title={t('bills.all')} />
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 sm:mx-0 sm:px-0" role="group" aria-label={t('bills.filterStatus')}>
          {STATUS_FILTERS.map((f) => (
            <Chip key={f.key} selected={statusKey === f.key} onClick={() => set({ status: f.key === 'all' ? undefined : f.key })}>
              {f.key === 'all' ? t('bills.allStatuses') : t(`status.${f.key as 'paid'}`)}
            </Chip>
          ))}
        </div>
        <div className="flex items-center gap-3">
          <label className="sr-only" htmlFor="bills-profile">
            {t('bills.filterProfile')}
          </label>
          <Select id="bills-profile" className="h-11 sm:w-60" value={search.profileId ?? ''} onChange={(e) => set({ profileId: e.target.value || undefined })}>
            <option value="">{t('bills.allProfiles')}</option>
            {(profiles ?? []).map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
        </div>
      </div>
      <Toggle label={t('bills.archivedView')} checked={Boolean(search.archived)} onChange={(v) => set({ archived: v || undefined })} />
      <BillsList filters={{ profileId: search.profileId, status: statuses, archived: search.archived }} profilesById={byId} />
    </div>
  )
}
