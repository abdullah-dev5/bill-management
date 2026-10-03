import { Link } from '@tanstack/react-router'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { ApiError, type BillCreate, type BillDetail, type PaymentMethod, type Profile } from '@/api/client'
import { useTypes } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { Field, Input, Select, Textarea, Toggle } from '@/components/ui/field'
import { MoneyInput } from '@/components/ui/money-input'
import { currentMonthKey, todayISO } from '@/lib/dates'
import { applyServerErrors } from '@/lib/forms'
import { formatMoney } from '@/lib/money'
import { METHODS } from './payment-sheet'

interface Values {
  periodKey: string
  issueDate: string
  dueDate: string
  expenseDate: string
  description: string
  typeId: string
  currentChargesMinor: number | null
  arrearsMinor: number | null
  adjustmentsMinor: number | null
  payableMinor: number | null
  payableAfterDueMinor: number | null
  billNumber: string
  isEstimated: boolean
  cancelled: boolean
  notes: string
  paidNow: boolean
  paidOn: string
  method: PaymentMethod
}

const valid = (n: number | null | undefined): n is number => n != null && !Number.isNaN(n)

function toValues(profile: Profile, bill?: BillDetail): Values {
  const expenseLog = profile.kind === 'expense_log'
  return {
    periodKey: bill?.periodKey ?? (expenseLog ? '' : currentMonthKey()),
    issueDate: bill?.issueDate ?? '',
    dueDate: bill?.dueDate ?? '',
    expenseDate: bill?.expenseDate ?? todayISO(),
    description: bill?.description ?? '',
    typeId: bill?.typeId ?? profile.typeId,
    currentChargesMinor: bill?.currentChargesMinor ?? (profile.kind === 'recurring' ? (profile.expectedAmountMinor ?? null) : null),
    arrearsMinor: bill?.arrearsMinor || null,
    adjustmentsMinor: bill?.adjustmentsMinor || null,
    payableMinor: bill ? bill.payableMinor : null,
    payableAfterDueMinor: bill?.payableAfterDueMinor ?? null,
    billNumber: bill?.billNumber ?? '',
    isEstimated: bill?.isEstimated ?? false,
    cancelled: bill?.cancelled ?? false,
    notes: bill?.notes ?? '',
    // Expense-log entries are usually paid on the spot.
    paidNow: expenseLog,
    paidOn: todayISO(),
    method: 'cash',
  }
}

interface BillFormProps {
  profile: Profile
  bill?: BillDetail
  onSubmit: (body: BillCreate) => Promise<unknown>
  onCancel: () => void
}

export function BillForm({ profile, bill, onSubmit, onCancel }: BillFormProps) {
  const { t } = useTranslation()
  const { data: types } = useTypes()
  const expenseLog = profile.kind === 'expense_log'
  const form = useForm<Values>({ defaultValues: toValues(profile, bill) })
  const { register, control, watch, formState } = form
  const { errors } = formState
  const [more, setMore] = useState(Boolean(bill && (bill.arrearsMinor || bill.adjustmentsMinor || bill.billNumber || bill.notes)))
  const [duplicateId, setDuplicateId] = useState<string>()

  const subCategories = useMemo(() => (types ?? []).filter((ty) => ty.parentId === profile.typeId && !ty.archived), [types, profile.typeId])

  const [current, arrears, adjustments, payable] = watch(['currentChargesMinor', 'arrearsMinor', 'adjustmentsMinor', 'payableMinor'])
  const autoPayable = (valid(current) ? current : 0) + (valid(arrears) ? arrears : 0) + (valid(adjustments) ? adjustments : 0)
  const paidNow = watch('paidNow')

  const submit = form.handleSubmit(async (v) => {
    setDuplicateId(undefined)
    if (!valid(v.currentChargesMinor) || v.currentChargesMinor < 0) {
      form.setError('currentChargesMinor', { message: t('validation.amount') })
      return
    }
    for (const k of ['arrearsMinor', 'adjustmentsMinor', 'payableMinor', 'payableAfterDueMinor'] as const) {
      if (v[k] != null && Number.isNaN(v[k])) {
        form.setError(k, { message: t('validation.amount') })
        return
      }
    }
    const body: BillCreate = {
      profileId: profile.id,
      currentChargesMinor: v.currentChargesMinor,
      arrearsMinor: v.arrearsMinor ?? 0,
      adjustmentsMinor: v.adjustmentsMinor ?? 0,
      payableMinor: valid(v.payableMinor) ? v.payableMinor : null,
      payableAfterDueMinor: valid(v.payableAfterDueMinor) ? v.payableAfterDueMinor : null,
      billNumber: v.billNumber.trim() || null,
      notes: v.notes.trim() || null,
      isEstimated: v.isEstimated,
      cancelled: v.cancelled,
      tags: bill?.tags ?? [],
    }
    if (expenseLog) {
      body.description = v.description.trim()
      body.expenseDate = v.expenseDate
      body.typeId = v.typeId
    } else {
      body.periodKey = v.periodKey
      body.issueDate = v.issueDate || null
      body.dueDate = v.dueDate || null
    }
    if (!bill && v.paidNow) body.paidNow = { paidOn: v.paidOn, method: v.method }
    try {
      await onSubmit(body)
    } catch (err) {
      if (err instanceof ApiError && err.code === 'duplicate_bill') setDuplicateId(err.existingBillId)
      applyServerErrors(err, form.setError)
    }
  })

  const money = (name: 'currentChargesMinor' | 'arrearsMinor' | 'adjustmentsMinor' | 'payableMinor' | 'payableAfterDueMinor', allowNegative = false) =>
    function MoneyField(p: { id: string }) {
      return (
        <Controller
          control={control}
          name={name}
          render={({ field }) => <MoneyInput {...p} allowNegative={allowNegative} value={field.value} onChange={field.onChange} />}
        />
      )
    }

  return (
    <form onSubmit={submit} className="flex flex-col gap-5" noValidate>
      {duplicateId && (
        <div role="alert" className="flex flex-col gap-2 rounded-tile bg-[var(--st-soon-bg)] p-4 text-[var(--st-soon-fg)]">
          <p className="font-semibold">{t('bills.duplicateTitle')}</p>
          <p className="text-sm">{t('bills.duplicateBody')}</p>
          <Link to="/bills/$billId" params={{ billId: duplicateId }} className="text-sm font-semibold underline">
            {t('bills.openExisting')}
          </Link>
        </div>
      )}

      {expenseLog ? (
        <>
          <Field label={t('bills.description')} error={errors.description?.message}>
            {(p) => <Input {...p} placeholder={t('bills.descriptionPlaceholder')} {...register('description', { required: t('validation.required') })} />}
          </Field>
          <div className="grid gap-5 sm:grid-cols-2">
            <Field label={t('bills.amount')} error={errors.currentChargesMinor?.message}>
              {money('currentChargesMinor')}
            </Field>
            <Field label={t('bills.date')} error={errors.expenseDate?.message}>
              {(p) => <Input {...p} type="date" {...register('expenseDate', { required: t('validation.required') })} />}
            </Field>
          </div>
          {subCategories.length > 0 && (
            <Field label={t('bills.subCategory')} optional>
              {(p) => (
                <Select {...p} {...register('typeId')}>
                  <option value={profile.typeId}>{t('common.none')}</option>
                  {subCategories.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
          )}
        </>
      ) : (
        <>
          <div className="grid gap-5 sm:grid-cols-3">
            <Field label={t('bills.month')} error={errors.periodKey?.message}>
              {(p) => <Input {...p} type="month" {...register('periodKey', { required: t('validation.required') })} />}
            </Field>
            <Field label={t('bills.issueDate')} error={errors.issueDate?.message} optional>
              {(p) => <Input {...p} type="date" {...register('issueDate')} />}
            </Field>
            <Field label={t('bills.dueDate')} error={errors.dueDate?.message} optional>
              {(p) => <Input {...p} type="date" {...register('dueDate')} />}
            </Field>
          </div>
          <div className="grid gap-5 sm:grid-cols-2">
            <Field label={t('bills.currentCharges')} hint={t('bills.currentChargesHint')} error={errors.currentChargesMinor?.message}>
              {money('currentChargesMinor')}
            </Field>
            <Field label={t('bills.payableAfterDue')} error={errors.payableAfterDueMinor?.message} optional>
              {money('payableAfterDueMinor')}
            </Field>
          </div>
        </>
      )}

      <button type="button" onClick={() => setMore((m) => !m)} className="flex items-center gap-1 self-start text-sm font-semibold text-ink" aria-expanded={more}>
        {more ? <ChevronUp className="size-4" aria-hidden /> : <ChevronDown className="size-4" aria-hidden />}
        {more ? t('common.fewerDetails') : t('common.moreDetails')}
      </button>

      {more && (
        <div className="flex flex-col gap-5 rounded-card border border-line p-4 sm:p-5">
          {!expenseLog && (
            <>
              <div className="grid gap-5 sm:grid-cols-2">
                <Field label={t('bills.arrears')} error={errors.arrearsMinor?.message} optional>
                  {money('arrearsMinor')}
                </Field>
                <Field label={t('bills.adjustments')} hint={t('bills.adjustmentsHint')} error={errors.adjustmentsMinor?.message} optional>
                  {money('adjustmentsMinor', true)}
                </Field>
              </div>
              <Field
                label={t('bills.payable')}
                hint={valid(payable) ? undefined : `${t('bills.payableHint')} (${formatMoney(autoPayable)})`}
                error={errors.payableMinor?.message}
                optional
              >
                {money('payableMinor')}
              </Field>
              <Field label={t('bills.billNumber')} error={errors.billNumber?.message} optional>
                {(p) => <Input {...p} autoComplete="off" {...register('billNumber')} />}
              </Field>
              <Controller control={control} name="isEstimated" render={({ field }) => <Toggle label={t('bills.estimated')} checked={field.value} onChange={field.onChange} />} />
            </>
          )}
          <Field label={t('bills.notes')} optional>
            {(p) => <Textarea {...p} {...register('notes')} />}
          </Field>
          {bill && (
            <Controller control={control} name="cancelled" render={({ field }) => <Toggle label={t('bills.markCancelled')} checked={field.value} onChange={field.onChange} />} />
          )}
        </div>
      )}

      {!bill && (
        <div className="flex flex-col gap-4 rounded-card bg-card-muted/60 p-4 sm:p-5">
          <Controller control={control} name="paidNow" render={({ field }) => <Toggle label={t('bills.alreadyPaid')} checked={field.value} onChange={field.onChange} />} />
          {paidNow && (
            <div className="grid gap-5 sm:grid-cols-2">
              <Field label={t('bills.paidOn')}>
                {(p) => <Input {...p} type="date" max={todayISO()} {...register('paidOn', { required: true })} />}
              </Field>
              <Field label={t('bills.method')}>
                {(p) => (
                  <Select {...p} {...register('method')}>
                    {METHODS.map((m) => (
                      <option key={m} value={m}>
                        {t(`method.${m}`)}
                      </option>
                    ))}
                  </Select>
                )}
              </Field>
            </div>
          )}
        </div>
      )}

      <div className="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
        <Button variant="secondary" onClick={onCancel}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" loading={formState.isSubmitting}>
          {t('common.save')}
        </Button>
      </div>
    </form>
  )
}
