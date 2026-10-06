import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { AlertTriangle, Check, GitMerge, Split } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api, unwrap, type BillDetail, type Schemas } from '@/api/client'
import { keys } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { DateInput } from '@/components/ui/date-input'
import { Field, Input } from '@/components/ui/field'
import { Card, SectionTitle } from '@/components/ui/misc'
import { MoneyInput } from '@/components/ui/money-input'
import { Sheet } from '@/components/ui/sheet'
import { cn } from '@/lib/cn'
import { formatDate, formatMonth } from '@/lib/dates'
import { formatMoney } from '@/lib/money'

type Conflict = Schemas['BillConflict']
type Origin = Schemas['BillOrigin']

// The compared fields, in the order a bill reads.
export const RECONCILE_FIELDS = [
  'periodKey', 'billNumber', 'issueDate', 'dueDate', 'currentChargesMinor', 'arrearsMinor',
  'payableMinor', 'payableAfterDueMinor', 'previousReading', 'currentReading', 'units',
] as const
type FieldKey = (typeof RECONCILE_FIELDS)[number]

const isMoney = (f: string) => f.endsWith('Minor')
const isDate = (f: string) => f === 'issueDate' || f === 'dueDate'

/** A stored value as people read it: "Rs 3,266", "05 Oct 2026", "September 2026". */
export function showValue(field: string, value: string | undefined, currency = 'PKR'): string {
  if (value == null || value === '') return '—'
  if (isMoney(field)) return formatMoney(Number(value), currency)
  if (isDate(field)) return formatDate(value)
  if (field === 'periodKey') return formatMonth(value)
  return value
}

function useInvalidateBill(id: string) {
  const qc = useQueryClient()
  return (data: BillDetail) => {
    qc.setQueryData(keys.bill(id), data)
    void qc.invalidateQueries({ queryKey: ['bills'] })
    void qc.invalidateQueries({ queryKey: ['home'] })
  }
}

/** "Needs review": fields where sources disagree, each settled by a person (FR-RECO-04/05). */
export function ConflictsCard({ bill }: { bill: BillDetail }) {
  const { t } = useTranslation()
  const [active, setActive] = useState<Conflict>()
  const open = bill.conflicts.filter((c) => c.status === 'open')
  if (open.length === 0) return null
  return (
    <Card className="flex flex-col gap-3 bg-[var(--st-soon-bg)] text-[var(--st-soon-fg)]" data-testid="conflicts">
      <p className="flex items-center gap-2 font-semibold">
        <AlertTriangle className="size-5" aria-hidden /> {t('reconcile.title', { count: open.length })}
      </p>
      <p className="text-sm">{t('reconcile.hint')}</p>
      <ul className="flex flex-col gap-2">
        {open.map((c) => (
          <li key={c.id} className="flex flex-wrap items-center gap-3 rounded-field bg-card p-3 text-body">
            <span className="min-w-0 flex-1 text-sm">
              <span className="font-semibold text-ink">{t(`reconcile.field.${c.field as FieldKey}`)}</span>
              <span className="block text-muted">
                {t('reconcile.versus', {
                  bill: showValue(c.field, c.billValue, bill.currency),
                  source: showValue(c.field, c.sourceValue, bill.currency),
                  kind: t(`reconcile.kind.${c.sourceKind}`),
                })}
              </span>
            </span>
            <Button size="sm" variant="primary" onClick={() => setActive(c)}>
              {t('reconcile.decide')}
            </Button>
          </li>
        ))}
      </ul>
      {active && <ResolveSheet bill={bill} conflict={active} onClose={() => setActive(undefined)} />}
    </Card>
  )
}

function ResolveSheet({ bill, conflict: c, onClose }: { bill: BillDetail; conflict: Conflict; onClose: () => void }) {
  const { t } = useTranslation()
  const done = useInvalidateBill(bill.id)
  const [manual, setManual] = useState<string>('')
  const resolve = useMutation({
    mutationFn: (body: Schemas['ConflictResolution']) =>
      unwrap(api.POST('/bills/{id}/conflicts/{conflictId}/resolve', { params: { path: { id: bill.id, conflictId: c.id } }, body })),
    onSuccess: (data) => {
      done(data)
      toast.success(t('reconcile.resolved'))
      onClose()
    },
    onError: (e) => toast.error(e.message),
  })
  const label = t(`reconcile.field.${c.field as FieldKey}`)
  const billShown = showValue(c.field, c.billValue, bill.currency)
  const sourceShown = showValue(c.field, c.sourceValue, bill.currency)
  return (
    <Sheet open onOpenChange={(o) => !o && onClose()} title={t('reconcile.sheetTitle', { field: label })} description={t('reconcile.sheetHint')}>
      <div className="flex flex-col gap-3">
        <Choice
          title={t('reconcile.keep', { value: billShown })}
          hint={t('reconcile.keepHint')}
          busy={resolve.isPending}
          onClick={() => resolve.mutate({ action: 'keep' })}
        />
        <Choice
          title={t('reconcile.useSource', { value: sourceShown, kind: t(`reconcile.kind.${c.sourceKind}`) })}
          hint={t('reconcile.useSourceHint')}
          busy={resolve.isPending}
          onClick={() => resolve.mutate({ action: 'source' })}
        />
        {(isMoney(c.field) || isDate(c.field)) && (
          <Choice
            title={t('reconcile.revised', { value: sourceShown })}
            hint={t('reconcile.revisedHint')}
            busy={resolve.isPending}
            onClick={() => resolve.mutate({ action: 'revised' })}
          />
        )}
        <form
          className="flex flex-col gap-2 rounded-field border border-line p-4"
          onSubmit={(e) => {
            e.preventDefault()
            if (manual) resolve.mutate({ action: 'manual', value: manual })
          }}
        >
          <p className="font-semibold text-ink">{t('reconcile.manual')}</p>
          <Field label={label}>
            {(p) =>
              isMoney(c.field) ? (
                <MoneyInput {...p} value={manual ? Number(manual) : null} onChange={(v) => setManual(v == null ? '' : String(v))} />
              ) : isDate(c.field) ? (
                <DateInput {...p} value={manual} onChange={setManual} />
              ) : c.field === 'periodKey' ? (
                <DateInput {...p} mode="month" value={manual} onChange={setManual} />
              ) : (
                <Input {...p} value={manual} onChange={(e) => setManual(e.target.value)} inputMode={c.field === 'billNumber' ? 'text' : 'decimal'} />
              )
            }
          </Field>
          <Button type="submit" variant="secondary" className="self-start" disabled={!manual} loading={resolve.isPending}>
            {t('reconcile.save')}
          </Button>
        </form>
      </div>
    </Sheet>
  )
}

function Choice({ title, hint, busy, onClick }: { title: string; hint: string; busy: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      disabled={busy}
      onClick={onClick}
      className="flex items-start gap-3 rounded-field border border-line p-4 text-left transition hover:bg-card-muted disabled:opacity-50"
    >
      <Check className="mt-0.5 size-5 shrink-0 text-muted" aria-hidden />
      <span className="flex flex-col">
        <span className="font-semibold text-ink">{title}</span>
        <span className="text-sm text-muted">{hint}</span>
      </span>
    </button>
  )
}

/** Every source and its values side by side (FR-RECO-06), with "separate" for recent merges (FR-RECO-07). */
export function SourcesCard({ bill }: { bill: BillDetail }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const done = useInvalidateBill(bill.id)
  const [confirming, setConfirming] = useState<Origin>()
  const separate = useMutation({
    mutationFn: (s: Origin) => unwrap(api.POST('/bills/{id}/sources/{sourceId}/separate', { params: { path: { id: bill.id, sourceId: s.id } } })),
    onSuccess: (data) => {
      done(data.bill)
      setConfirming(undefined)
      toast.success(t('reconcile.separated'), {
        action: data.documentId
          ? { label: t('reconcile.saveSeparately'), onClick: () => void navigate({ to: '/scan/$documentId', params: { documentId: data.documentId! } }) }
          : undefined,
      })
    },
    onError: (e) => toast.error(e.message),
  })

  if (bill.sources.length < 2) return null
  const current: Record<string, string | undefined> = {
    periodKey: bill.periodKey ?? undefined, billNumber: bill.billNumber ?? undefined, issueDate: bill.issueDate ?? undefined,
    dueDate: bill.dueDate ?? undefined, currentChargesMinor: String(bill.currentChargesMinor),
    arrearsMinor: bill.arrearsMinor ? String(bill.arrearsMinor) : undefined, payableMinor: String(bill.payableMinor),
    payableAfterDueMinor: bill.payableAfterDueMinor != null ? String(bill.payableAfterDueMinor) : undefined,
    previousReading: bill.previousReading?.toString(), currentReading: bill.currentReading?.toString(), units: bill.units?.toString(),
  }
  const rows = RECONCILE_FIELDS.filter((f) => current[f] != null || bill.sources.some((s) => s.values[f]))

  return (
    <Card data-testid="sources" className="min-w-0 overflow-hidden">
      <SectionTitle>
        <span className="flex items-center gap-2">
          <GitMerge className="size-5" aria-hidden /> {t('reconcile.sources', { count: bill.sources.length })}
        </span>
      </SectionTitle>
      <div className="relative w-full max-w-full overflow-x-auto">
        <table className="tabular w-full min-w-[30rem] text-sm">
          <thead className="text-left text-xs text-muted">
            <tr>
              <th className="py-2 pr-3 font-medium">{t('reconcile.fieldHeader')}</th>
              <th className="py-2 pr-3 font-semibold text-ink">{t('reconcile.onBill')}</th>
              {bill.sources.map((s) => (
                <th key={s.id} className="py-2 pr-3 font-medium">
                  {t(`reconcile.kind.${s.kind}`)}
                  <span className="block font-normal">{formatDate(s.createdAt.slice(0, 10))}</span>
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {rows.map((f) => (
              <tr key={f}>
                <td className="py-2 pr-3 text-muted">{t(`reconcile.field.${f}`)}</td>
                <td className="py-2 pr-3 font-medium text-ink">{showValue(f, current[f], bill.currency)}</td>
                {bill.sources.map((s) => {
                  const v = s.values[f]
                  const differs = v != null && current[f] != null && showValue(f, v) !== showValue(f, current[f])
                  return (
                    <td key={s.id} className={cn('py-2 pr-3', differs && 'font-semibold text-[var(--st-soon-fg)]', v == null && 'text-muted')}>
                      {showValue(f, v, bill.currency)}
                      {s.filledFields.includes(f) && <span className="sr-only"> ({t('reconcile.filled')})</span>}
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {bill.sources.some((s) => s.canSeparate) && (
        <div className="mt-3 flex flex-wrap gap-2 border-t border-line pt-3">
          {bill.sources
            .filter((s) => s.canSeparate)
            .map((s) => (
              <Button key={s.id} size="sm" variant="ghost" icon={<Split className="size-4" />} onClick={() => setConfirming(s)}>
                {t('reconcile.separate', { kind: t(`reconcile.kind.${s.kind}`), date: formatDate(s.createdAt.slice(0, 10)) })}
              </Button>
            ))}
        </div>
      )}
      {confirming && (
        <Sheet
          open
          onOpenChange={(o) => !o && setConfirming(undefined)}
          title={t('reconcile.separateTitle')}
          description={t('reconcile.separateHint')}
          footer={
            <>
              <Button variant="secondary" onClick={() => setConfirming(undefined)}>
                {t('common.cancel')}
              </Button>
              <Button variant="danger" loading={separate.isPending} onClick={() => separate.mutate(confirming)}>
                {t('reconcile.separateConfirm')}
              </Button>
            </>
          }
        >
          <p className="text-sm text-body">{t('reconcile.separateBody', { kind: t(`reconcile.kind.${confirming.kind}`) })}</p>
        </Sheet>
      )}
    </Card>
  )
}
