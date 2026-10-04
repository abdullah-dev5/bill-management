import { Link } from '@tanstack/react-router'
import { FileText, Paperclip, Receipt } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { BillStatus, Profile } from '@/api/client'
import { useProfileDocuments, useProfilePayments } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { Card, Chip, EmptyState, ErrorNotice, Skeleton } from '@/components/ui/misc'
import { BillsList } from '@/features/bills/bill-row'
import { DocumentViewer } from '@/features/documents/documents-section'
import { formatDate, formatMonth } from '@/lib/dates'
import { formatBytes } from '@/lib/image'
import { formatMoney } from '@/lib/money'

export const PROFILE_STATUS_FILTERS: { key: string; statuses: BillStatus[] }[] = [
  { key: 'all', statuses: [] },
  { key: 'unpaid', statuses: ['unpaid', 'partially_paid', 'overdue'] },
  { key: 'overdue', statuses: ['overdue'] },
  { key: 'paid', statuses: ['paid'] },
]

const billLabel = (periodKey: string | null | undefined, description: string | null | undefined, expenseDate: string) =>
  periodKey ? formatMonth(periodKey) : (description ?? formatDate(expenseDate))

/** The profile's bills with a status filter kept in the URL. */
export function ProfileBills({ profile, status, onStatus, empty }: {
  profile: Profile
  status: string
  onStatus: (s: string | undefined) => void
  empty: React.ReactNode
}) {
  const { t } = useTranslation()
  const statuses = PROFILE_STATUS_FILTERS.find((f) => f.key === status)?.statuses ?? []
  return (
    <div className="flex flex-col gap-4">
      <div className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 sm:mx-0 sm:px-0" role="group" aria-label={t('bills.filterStatus')}>
        {PROFILE_STATUS_FILTERS.map((f) => (
          <Chip key={f.key} selected={status === f.key} onClick={() => onStatus(f.key === 'all' ? undefined : f.key)}>
            {f.key === 'all' ? t('bills.allStatuses') : t(`status.${f.key as 'paid'}`)}
          </Chip>
        ))}
      </div>
      <BillsList filters={{ profileId: profile.id, status: statuses }} empty={status === 'all' ? empty : undefined} />
    </div>
  )
}

/** Every payment made towards the profile's bills, with receipts. */
export function ProfilePayments({ profile }: { profile: Profile }) {
  const { t } = useTranslation()
  const payments = useProfilePayments(profile.id)
  const [receiptId, setReceiptId] = useState<{ doc: string; bill: string }>()

  if (payments.isError) return <ErrorNotice error={payments.error} onRetry={() => void payments.refetch()} />
  if (!payments.data) return <Skeleton className="h-48 rounded-card" />
  if (payments.data.length === 0) {
    return (
      <Card>
        <EmptyState icon={<Receipt className="size-10" />} title={t('profiles.noPaymentsTitle')} body={t('profiles.noPaymentsBody')} />
      </Card>
    )
  }
  const total = payments.data.reduce((sum, p) => sum + p.amountMinor, 0)
  return (
    <Card className="flex flex-col gap-2">
      <p className="text-sm text-muted">{t('profiles.paymentsTotal', { count: payments.data.length, amount: formatMoney(total) })}</p>
      <ul className="divide-y divide-line">
        {payments.data.map((p) => (
          <li key={p.id} className="flex items-center gap-3 py-3">
            <Link to="/bills/$billId" params={{ billId: p.billId }} className="flex min-w-0 flex-1 flex-col">
              <span className="truncate font-medium text-ink">{billLabel(p.billPeriodKey, p.billDescription, p.billExpenseDate)}</span>
              <span className="text-sm text-muted">
                {formatDate(p.paidOn)} · {t(`method.${p.method}`)}
                {p.reference && ` · ${p.reference}`}
              </span>
            </Link>
            <span className="tabular font-medium text-ink">{formatMoney(p.amountMinor)}</span>
            {p.receiptDocumentId && (
              <Button size="icon" variant="ghost" aria-label={t('bills.viewReceipt')} onClick={() => setReceiptId({ doc: p.receiptDocumentId!, bill: p.billId })}>
                <Paperclip className="size-4" />
              </Button>
            )}
          </li>
        ))}
      </ul>
      <DocumentViewer id={receiptId?.doc} billId={receiptId?.bill ?? ''} readOnly onClose={() => setReceiptId(undefined)} />
    </Card>
  )
}

/** Every document attached to the profile's bills. */
export function ProfileDocuments({ profile }: { profile: Profile }) {
  const { t } = useTranslation()
  const docs = useProfileDocuments(profile.id)
  const [open, setOpen] = useState<{ doc: string; bill: string }>()

  if (docs.isError) return <ErrorNotice error={docs.error} onRetry={() => void docs.refetch()} />
  if (!docs.data) return <Skeleton className="h-48 rounded-card" />
  if (docs.data.length === 0) {
    return (
      <Card>
        <EmptyState icon={<FileText className="size-10" />} title={t('profiles.noDocumentsTitle')} body={t('profiles.noDocumentsBody')} />
      </Card>
    )
  }
  return (
    <>
      <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
        {docs.data.map((d) => (
          <li key={d.id}>
            <button
              type="button"
              onClick={() => setOpen({ doc: d.id, bill: d.billId ?? '' })}
              className="flex w-full flex-col overflow-hidden rounded-field border border-line bg-card text-left transition hover:border-focus"
            >
              <span className="flex aspect-[4/3] items-center justify-center overflow-hidden bg-card-muted">
                {d.contentType === 'application/pdf' || !d.viewUrl ? (
                  <FileText className="size-10 text-muted" aria-hidden />
                ) : (
                  <img src={d.viewUrl} alt="" loading="lazy" className="size-full object-cover" />
                )}
              </span>
              <span className="flex flex-col px-3 py-2">
                <span className="truncate text-sm font-medium text-ink">{d.fileName}</span>
                <span className="text-xs text-muted">
                  {t(`documents.kind.${d.kind}`)} · {formatBytes(d.sizeBytes)} · {formatDate(d.createdAt.slice(0, 10))}
                </span>
              </span>
            </button>
          </li>
        ))}
      </ul>
      <DocumentViewer id={open?.doc} billId={open?.bill ?? ''} onClose={() => setOpen(undefined)} />
    </>
  )
}
