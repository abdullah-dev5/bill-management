import { Paperclip } from 'lucide-react'
import { useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { BillDetail, Doc, PaymentMethod } from '@/api/client'
import { useAddPayment } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { Field, Input, Select } from '@/components/ui/field'
import { MoneyInput } from '@/components/ui/money-input'
import { Sheet } from '@/components/ui/sheet'
import { todayISO } from '@/lib/dates'
import { applyServerErrors } from '@/lib/forms'
import { CaptureSheet } from '@/features/documents/capture-sheet'

export const METHODS: PaymentMethod[] = ['cash', 'bank_transfer', 'card', 'jazzcash', 'easypaisa', 'cheque', 'other']

/** The amount that applies if paid on `paidOn` (late amount after the due date, FR-PAY-05). */
type DueFields = Pick<BillDetail, 'payableMinor' | 'payableAfterDueMinor' | 'dueDate' | 'paidMinor' | 'lateStepUntil' | 'payableFinalMinor'>

/** The full amount payable on a date: within the due date, after it, or after the late-fee step. */
export function payableOn(bill: Omit<DueFields, 'paidMinor'>, paidOn: string): number {
  if (bill.dueDate == null || paidOn <= bill.dueDate || bill.payableAfterDueMinor == null) return bill.payableMinor
  if (bill.lateStepUntil && bill.payableFinalMinor != null && paidOn > bill.lateStepUntil) return bill.payableFinalMinor
  return bill.payableAfterDueMinor
}

export function amountDueOn(bill: DueFields, paidOn: string): number {
  return Math.max(payableOn(bill, paidOn) - bill.paidMinor, 0)
}

interface Values {
  paidOn: string
  amountMinor: number | null
  method: PaymentMethod
  reference: string
}

export function PaymentSheet({ bill, open, onOpenChange }: { bill: BillDetail; open: boolean; onOpenChange: (v: boolean) => void }) {
  const { t } = useTranslation()
  const add = useAddPayment(bill.id)
  const lastMethod = bill.payments.at(-1)?.method ?? 'cash'
  const form = useForm<Values>({
    values: { paidOn: todayISO(), amountMinor: amountDueOn(bill, todayISO()), method: lastMethod, reference: '' },
  })
  const [receipt, setReceipt] = useState<Doc>()
  const [capturing, setCapturing] = useState(false)
  const paidOn = form.watch('paidOn')
  const isLate = bill.dueDate != null && paidOn > bill.dueDate && bill.payableAfterDueMinor != null

  const submit = form.handleSubmit(async (v) => {
    if (v.amountMinor == null || Number.isNaN(v.amountMinor) || v.amountMinor <= 0) {
      form.setError('amountMinor', { message: t('validation.amount') })
      return
    }
    try {
      await add.mutateAsync({
        paidOn: v.paidOn, amountMinor: v.amountMinor, method: v.method, reference: v.reference.trim() || null,
        receiptDocumentId: receipt?.id,
      })
      toast.success(t('bills.paymentAdded'))
      setReceipt(undefined)
      onOpenChange(false)
    } catch (err) {
      applyServerErrors(err, form.setError)
    }
  })

  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={t('bills.addPayment')}
      footer={
        <>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          <Button onClick={() => void submit()} loading={form.formState.isSubmitting}>
            {t('common.save')}
          </Button>
        </>
      }
    >
      <form onSubmit={submit} className="flex flex-col gap-4" noValidate>
        <Field label={t('bills.paidOn')} error={form.formState.errors.paidOn?.message}>
          {(p) => (
            <Input
              {...p}
              type="date"
              max={todayISO()}
              {...form.register('paidOn', {
                required: true,
                onChange: (e) => form.setValue('amountMinor', amountDueOn(bill, e.target.value)),
              })}
            />
          )}
        </Field>
        {isLate && <p className="rounded-tile bg-[var(--st-soon-bg)] p-3 text-sm text-[var(--st-soon-fg)]">{t('bills.lateFeeNotice')}</p>}
        <Field label={t('bills.amount')} error={form.formState.errors.amountMinor?.message}>
          {(p) => (
            <Controller
              control={form.control}
              name="amountMinor"
              render={({ field }) => <MoneyInput {...p} currency={bill.currency} value={field.value} onChange={field.onChange} />}
            />
          )}
        </Field>
        <Field label={t('bills.method')}>
          {(p) => (
            <Select {...p} {...form.register('method')}>
              {METHODS.map((m) => (
                <option key={m} value={m}>
                  {t(`method.${m}`)}
                </option>
              ))}
            </Select>
          )}
        </Field>
        <Field label={t('bills.reference')} optional>
          {(p) => <Input {...p} autoComplete="off" {...form.register('reference')} />}
        </Field>
        <div className="flex flex-wrap items-center gap-3 rounded-field border border-dashed border-line p-3">
          {receipt ? (
            <>
              <Paperclip className="size-4 text-muted" aria-hidden />
              <span className="min-w-0 flex-1 truncate text-sm text-ink" data-testid="receipt-name">
                {receipt.fileName}
              </span>
              <Button size="sm" variant="ghost" onClick={() => setReceipt(undefined)}>
                {t('common.remove')}
              </Button>
            </>
          ) : (
            <Button size="sm" variant="ghost" icon={<Paperclip className="size-4" />} onClick={() => setCapturing(true)}>
              {t('bills.attachReceipt')}
            </Button>
          )}
        </div>
      </form>
      <CaptureSheet
        open={capturing}
        onOpenChange={setCapturing}
        billId={bill.id}
        defaultKind="receipt"
        title={t('bills.attachReceipt')}
        onUploaded={({ document }) => setReceipt(document)}
      />
    </Sheet>
  )
}
