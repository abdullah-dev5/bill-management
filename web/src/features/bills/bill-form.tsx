import { Link } from '@tanstack/react-router'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { ApiError, type BillCreate, type BillDetail, type PaymentMethod, type Profile } from '@/api/client'
import { useTypes } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { DateInput } from '@/components/ui/date-input'
import { Field, Input, Select, Textarea, Toggle } from '@/components/ui/field'
import { MoneyInput } from '@/components/ui/money-input'
import { currentMonthKey, todayISO } from '@/lib/dates'
import { applyServerErrors } from '@/lib/forms'
import { formatMoney } from '@/lib/money'
import { methodOptions } from './payment-sheet'

export interface BillFormValues {
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
  lateStepUntil: string
  payableFinalMinor: number | null
  billNumber: string
  isEstimated: boolean
  previousReading: string
  currentReading: string
  units: string
  multiplyingFactor: string
  meterReplaced: boolean
  cancelled: boolean
  notes: string
  paidNow: boolean
  paidOn: string
  method: PaymentMethod
}

const valid = (n: number | null | undefined): n is number => n != null && !Number.isNaN(n)

const numText = (n: number | null | undefined) => (n == null ? '' : String(n))

/** Parses a reading or unit count; empty is null, garbage is NaN. */
export const parseNumber = (s: string): number | null => {
  const t = s.replace(/,/g, '').trim()
  if (t === '') return null
  return /^\d+(\.\d+)?$/.test(t) ? Number(t) : NaN
}

/** Units from the readings: (present − previous) × multiplying factor. */
export function unitsFromReadings(prev: number | null, cur: number | null, mf: number | null): number | null {
  if (prev == null || cur == null || Number.isNaN(prev) || Number.isNaN(cur) || cur < prev) return null
  return Math.round((cur - prev) * (mf && mf > 0 ? mf : 1) * 100) / 100
}

function toValues(profile: Profile, bill?: BillDetail): BillFormValues {
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
    lateStepUntil: bill?.lateStepUntil ?? '',
    payableFinalMinor: bill?.payableFinalMinor ?? null,
    billNumber: bill?.billNumber ?? '',
    isEstimated: bill?.isEstimated ?? false,
    // A new bill starts from the profile's last present reading.
    previousReading: numText(bill ? bill.previousReading : profile.summary?.lastReading),
    currentReading: numText(bill?.currentReading),
    units: numText(bill?.units),
    multiplyingFactor: numText(bill?.multiplyingFactor ?? profile.multiplyingFactor ?? 1),
    meterReplaced: bill?.meterReplaced ?? false,
    cancelled: bill?.cancelled ?? false,
    notes: bill?.notes ?? '',
    // Expense-log entries are usually paid on the spot.
    paidNow: expenseLog,
    paidOn: todayISO(),
    method: 'cash',
  }
}

/** Review mode: values read from a document, with how sure the extractor was of each. */
export interface BillReview {
  values: Partial<BillFormValues>
  /** Form field name → confidence 0–1; fields not listed weren't read. */
  confidence: Partial<Record<keyof BillFormValues, number>>
  submitLabel: string
}

interface BillFormProps {
  profile: Profile
  bill?: BillDetail
  review?: BillReview
  onSubmit: (body: BillCreate) => Promise<unknown>
  onCancel: () => void
}

const CHECK_BELOW = 0.85

export function BillForm({ profile, bill, review, onSubmit, onCancel }: BillFormProps) {
  const { t } = useTranslation()
  const { data: types } = useTypes()
  const expenseLog = profile.kind === 'expense_log'
  const form = useForm<BillFormValues>({
    defaultValues: { ...toValues(profile, bill), ...review?.values, ...(review ? { paidNow: false } : {}) },
  })
  const { register, control, watch, formState } = form
  const { errors } = formState
  const initial = { ...toValues(profile, bill), ...review?.values }
  const [more, setMore] = useState(
    Boolean((bill || review) && (initial.arrearsMinor || initial.adjustmentsMinor || initial.billNumber || initial.notes)),
  )

  /** In review mode: a hint and highlight for values read with low confidence. */
  const check = (name: keyof BillFormValues): { hint?: string; className?: string } => {
    const c = review?.confidence[name]
    if (c == null || c >= CHECK_BELOW) return {}
    return { hint: t('scan.checkThis'), className: 'rounded-field ring-2 ring-[var(--st-soon-fg)]/40 ring-offset-4 ring-offset-card' }
  }
  const afterDue = watch('payableAfterDueMinor')
  const metered = profile.kind === 'metered'
  const [prevReading, curReading, mfText, replaced] = watch(['previousReading', 'currentReading', 'multiplyingFactor', 'meterReplaced'])
  const computedUnits = replaced ? null : unitsFromReadings(parseNumber(prevReading), parseNumber(curReading), parseNumber(mfText))
  const unitLabel = profile.consumptionUnit || t('bills.unitsDefault')
  const unitsFieldLabel = profile.consumptionUnit ? t('bills.unitsLabel', { unit: profile.consumptionUnit }) : t('bills.unitsPlain')
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
    for (const k of ['arrearsMinor', 'adjustmentsMinor', 'payableMinor', 'payableAfterDueMinor', 'payableFinalMinor'] as const) {
      if (v[k] != null && Number.isNaN(v[k])) {
        form.setError(k, { message: t('validation.amount') })
        return
      }
    }
    const readings: Partial<BillCreate> = {}
    if (metered) {
      const nums = {
        previousReading: parseNumber(v.previousReading),
        currentReading: parseNumber(v.currentReading),
        units: parseNumber(v.units),
        multiplyingFactor: parseNumber(v.multiplyingFactor),
      }
      for (const [k, n] of Object.entries(nums)) {
        if (n != null && Number.isNaN(n)) {
          form.setError(k as keyof BillFormValues, { message: t('validation.number') })
          return
        }
      }
      Object.assign(readings, nums, { meterReplaced: v.meterReplaced })
    }
    const body: BillCreate = {
      ...readings,
      profileId: profile.id,
      currentChargesMinor: v.currentChargesMinor,
      arrearsMinor: v.arrearsMinor ?? 0,
      adjustmentsMinor: v.adjustmentsMinor ?? 0,
      payableMinor: valid(v.payableMinor) ? v.payableMinor : null,
      payableAfterDueMinor: valid(v.payableAfterDueMinor) ? v.payableAfterDueMinor : null,
      lateStepUntil: valid(v.payableAfterDueMinor) && v.lateStepUntil ? v.lateStepUntil : null,
      payableFinalMinor: valid(v.payableAfterDueMinor) && v.lateStepUntil && valid(v.payableFinalMinor) ? v.payableFinalMinor : null,
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
    if (!bill && !review && v.paidNow) body.paidNow = { paidOn: v.paidOn, method: v.method }
    try {
      await onSubmit(body)
    } catch (err) {
      if (err instanceof ApiError && err.code === 'duplicate_bill') setDuplicateId(err.existingBillId)
      applyServerErrors(err, form.setError)
    }
  })

  const money = (
    name: 'currentChargesMinor' | 'arrearsMinor' | 'adjustmentsMinor' | 'payableMinor' | 'payableAfterDueMinor' | 'payableFinalMinor',
    allowNegative = false,
  ) =>
    function MoneyField(p: { id: string }) {
      return (
        <Controller
          control={control}
          name={name}
          render={({ field }) => <MoneyInput {...p} allowNegative={allowNegative} value={field.value} onChange={field.onChange} />}
        />
      )
    }

  const date = (name: 'issueDate' | 'dueDate' | 'expenseDate' | 'lateStepUntil' | 'paidOn' | 'periodKey', opts: { required?: boolean; max?: string } = {}) =>
    function DateField(p: { id: string }) {
      return (
        <Controller
          control={control}
          name={name}
          rules={opts.required ? { required: t('validation.required') } : undefined}
          render={({ field }) => <DateInput {...p} {...field} mode={name === 'periodKey' ? 'month' : 'date'} required={opts.required} max={opts.max} />}
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
              {date('expenseDate', { required: true })}
            </Field>
          </div>
          {subCategories.length > 0 && (
            <Field label={t('bills.subCategory')} optional>
              {(p) => (
                <Controller
                  control={control}
                  name="typeId"
                  render={({ field }) => (
                    <Select
                      {...p}
                      {...field}
                      options={[{ value: profile.typeId, label: t('bills.noSubCategory') }, ...subCategories.map((sc) => ({ value: sc.id, label: sc.name }))]}
                    />
                  )}
                />
              )}
            </Field>
          )}
        </>
      ) : (
        <>
          <div className="grid gap-5 sm:grid-cols-3">
            <Field label={t('bills.month')} error={errors.periodKey?.message} {...check('periodKey')}>
              {date('periodKey', { required: true })}
            </Field>
            <Field label={t('bills.issueDate')} error={errors.issueDate?.message} optional {...check('issueDate')}>
              {date('issueDate')}
            </Field>
            <Field label={t('bills.dueDate')} error={errors.dueDate?.message} optional {...check('dueDate')}>
              {date('dueDate')}
            </Field>
          </div>
          <div className="grid gap-5 sm:grid-cols-2">
            <Field
              label={t('bills.currentCharges')}
              hint={t('bills.currentChargesHint')}
              error={errors.currentChargesMinor?.message}
              {...check('currentChargesMinor')}
            >
              {money('currentChargesMinor')}
            </Field>
            <Field label={t('bills.payableAfterDue')} error={errors.payableAfterDueMinor?.message} optional {...check('payableAfterDueMinor')}>
              {money('payableAfterDueMinor')}
            </Field>
          </div>
          {metered && (
            <div className="flex flex-col gap-3 rounded-field bg-card-muted/50 p-4">
              <div className="grid gap-4 sm:grid-cols-3">
                <Field label={t('bills.previousReading')} error={errors.previousReading?.message} optional {...check('previousReading')}>
                  {(p) => <Input {...p} inputMode="decimal" className="tabular" {...register('previousReading')} />}
                </Field>
                <Field label={t('bills.currentReading')} error={errors.currentReading?.message} optional {...check('currentReading')}>
                  {(p) => <Input {...p} inputMode="decimal" className="tabular" {...register('currentReading')} />}
                </Field>
                <Field
                  label={unitsFieldLabel}
                  hint={computedUnits != null ? t('bills.unitsComputed', { units: computedUnits, unit: unitLabel }) : undefined}
                  error={errors.units?.message}
                  optional
                  {...check('units')}
                >
                  {(p) => <Input {...p} inputMode="decimal" className="tabular" placeholder={computedUnits != null ? String(computedUnits) : ''} {...register('units')} />}
                </Field>
              </div>
              <Controller
                control={control}
                name="meterReplaced"
                render={({ field }) => (
                  <Toggle label={t('bills.meterReplaced')} hint={t('bills.meterReplacedHint')} checked={field.value} onChange={field.onChange} />
                )}
              />
            </div>
          )}
          {valid(afterDue) && (
            <div className="grid gap-5 sm:grid-cols-2">
              <Field label={t('bills.lateStepUntil')} hint={t('bills.lateStepHint')} error={errors.lateStepUntil?.message} optional>
                {date('lateStepUntil')}
              </Field>
              <Field label={t('bills.payableFinal')} error={errors.payableFinalMinor?.message} optional>
                {money('payableFinalMinor')}
              </Field>
            </div>
          )}
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
                <Field label={t('bills.arrears')} error={errors.arrearsMinor?.message} optional {...check('arrearsMinor')}>
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
              <Field label={t('bills.billNumber')} error={errors.billNumber?.message} optional {...check('billNumber')}>
                {(p) => <Input {...p} autoComplete="off" {...register('billNumber')} />}
              </Field>
              {metered && (
                <Field label={t('bills.multiplyingFactor')} hint={t('bills.multiplyingFactorHint')} error={errors.multiplyingFactor?.message}>
                  {(p) => <Input {...p} inputMode="decimal" className="tabular sm:max-w-40" {...register('multiplyingFactor')} />}
                </Field>
              )}
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

      {!bill && !review && (
        <div className="flex flex-col gap-4 rounded-card bg-card-muted/60 p-4 sm:p-5">
          <Controller control={control} name="paidNow" render={({ field }) => <Toggle label={t('bills.alreadyPaid')} checked={field.value} onChange={field.onChange} />} />
          {paidNow && (
            <div className="grid gap-5 sm:grid-cols-2">
              <Field label={t('bills.paidOn')}>
                {date('paidOn', { required: true, max: todayISO() })}
              </Field>
              <Field label={t('bills.method')}>
                {(p) => (
                  <Controller control={control} name="method" render={({ field }) => <Select {...p} {...field} options={methodOptions(t)} />} />
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
          {review?.submitLabel ?? t('common.save')}
        </Button>
      </div>
    </form>
  )
}
