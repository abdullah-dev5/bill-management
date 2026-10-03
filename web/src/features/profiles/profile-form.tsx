import { useEffect, useMemo, useRef } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import type { BillType, Profile, ProfileInput, ProfileKind } from '@/api/client'
import { useProviders, useTypes } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { Field, Input, Select, Toggle } from '@/components/ui/field'
import { MoneyInput } from '@/components/ui/money-input'
import { cn } from '@/lib/cn'
import { applyServerErrors } from '@/lib/forms'
import { ICON_NAMES, NamedIcon } from '@/lib/icons'

interface Values {
  typeId: string
  name: string
  kind: ProfileKind
  icon: string
  color: number
  providerId: string
  referenceNumber: string
  meterNumber: string
  multiplyingFactor: string
  consumptionUnit: string
  billingCycle: NonNullable<ProfileInput['billingCycle']>
  typicalDueDay: string
  expectedAmountMinor: number | null
  autoCreateBill: boolean
  pinned: boolean
}

const KINDS: ProfileKind[] = ['metered', 'recurring', 'expense_log']
const CYCLES = ['monthly', 'bimonthly', 'quarterly', 'yearly'] as const
const PALETTE = [1, 2, 3, 4, 5, 6, 7, 8]

/** A good default name for a new profile of this type, e.g. "Home Electricity". */
export function suggestName(type: BillType | undefined): string {
  if (!type) return ''
  return type.group === 'utilities' ? `Home ${type.name}` : type.name
}

function toValues(p?: Profile, defaultTypeId?: string): Values {
  return {
    typeId: p?.typeId ?? defaultTypeId ?? '',
    name: p?.name ?? '',
    kind: p?.kind ?? 'metered',
    icon: p?.icon ?? '',
    color: p?.color ?? 0,
    providerId: p?.providerId ?? '',
    referenceNumber: p?.referenceNumber ?? '',
    meterNumber: p?.meterNumber ?? '',
    multiplyingFactor: p?.multiplyingFactor != null ? String(p.multiplyingFactor) : '1',
    consumptionUnit: p?.consumptionUnit ?? '',
    billingCycle: p?.billingCycle ?? 'monthly',
    typicalDueDay: p?.typicalDueDay != null ? String(p.typicalDueDay) : '',
    expectedAmountMinor: p?.expectedAmountMinor ?? null,
    autoCreateBill: p?.autoCreateBill ?? false,
    pinned: p?.pinned ?? true,
  }
}

interface ProfileFormProps {
  profile?: Profile
  defaultTypeId?: string
  submitLabel: string
  onSubmit: (input: ProfileInput) => Promise<unknown>
  onCancel?: () => void
}

export function ProfileForm({ profile, defaultTypeId, submitLabel, onSubmit, onCancel }: ProfileFormProps) {
  const { t } = useTranslation()
  const { data: types } = useTypes()
  const form = useForm<Values>({ defaultValues: toValues(profile, defaultTypeId) })
  const { register, control, watch, setValue, formState } = form
  const { errors } = formState

  const typeId = watch('typeId')
  const kind = watch('kind')
  const icon = watch('icon')
  const { data: providers } = useProviders(typeId || undefined)

  const topLevel = useMemo(() => (types ?? []).filter((ty) => !ty.parentId && !ty.archived), [types])
  const selectedType = topLevel.find((ty) => ty.id === typeId)
  const kindLocked = (profile?.summary.billCount ?? 0) > 0 || profile?.isDefault
  const touched = useRef({ name: Boolean(profile), kind: Boolean(profile), icon: Boolean(profile) })

  // Choosing a type fills in its kind, icon and a suggested name until the user changes them.
  useEffect(() => {
    if (!selectedType) return
    if (!touched.current.kind) setValue('kind', selectedType.defaultKind)
    if (!touched.current.icon) setValue('icon', selectedType.icon)
    if (!touched.current.name) setValue('name', suggestName(selectedType))
  }, [selectedType, setValue])

  const grouped = (['utilities', 'household', 'other'] as const).map((g) => ({ g, items: topLevel.filter((ty) => ty.group === g) }))
  const cycleBased = kind === 'metered' || kind === 'recurring'

  const submit = form.handleSubmit(async (v) => {
    const mf = Number(v.multiplyingFactor)
    const input: ProfileInput = {
      name: v.name.trim(),
      typeId: v.typeId,
      kind: v.kind,
      icon: v.icon || undefined,
      color: v.color || undefined,
      providerId: v.providerId || null,
      referenceNumber: v.referenceNumber.trim() || null,
      pinned: v.pinned,
    }
    if (cycleBased) {
      input.billingCycle = v.billingCycle
      input.typicalDueDay = v.typicalDueDay ? Number(v.typicalDueDay) : null
    }
    if (v.kind === 'metered') {
      input.meterNumber = v.meterNumber.trim() || null
      input.multiplyingFactor = Number.isFinite(mf) && mf > 0 ? mf : 1
      input.consumptionUnit = v.consumptionUnit.trim() || null
    }
    if (v.kind === 'recurring') {
      input.expectedAmountMinor = v.expectedAmountMinor != null && !Number.isNaN(v.expectedAmountMinor) ? v.expectedAmountMinor : null
      input.autoCreateBill = v.autoCreateBill
    }
    try {
      await onSubmit(input)
    } catch (err) {
      applyServerErrors(err, form.setError)
    }
  })

  return (
    <form onSubmit={submit} className="flex flex-col gap-5" noValidate>
      <Field label={t('profiles.type')} error={errors.typeId?.message}>
        {(p) => (
          <Select {...p} {...register('typeId', { required: t('validation.required') })} disabled={profile?.isDefault}>
            <option value="">{t('profiles.chooseType')}</option>
            {grouped.map(({ g, items }) =>
              items.length ? (
                <optgroup key={g} label={t(`group.${g}`)}>
                  {items.map((ty) => (
                    <option key={ty.id} value={ty.id}>
                      {ty.name}
                    </option>
                  ))}
                </optgroup>
              ) : null,
            )}
          </Select>
        )}
      </Field>

      <Field label={t('profiles.name')} error={errors.name?.message}>
        {(p) => (
          <Input
            {...p}
            placeholder={t('profiles.namePlaceholder')}
            {...register('name', { required: t('validation.required'), onChange: () => (touched.current.name = true) })}
          />
        )}
      </Field>

      <fieldset className="flex flex-col gap-2">
        <legend className="mb-1.5 text-sm font-medium text-ink">{t('profiles.kind')}</legend>
        {KINDS.map((k) => (
          <label
            key={k}
            className={cn(
              'flex cursor-pointer items-start gap-3 rounded-tile border p-3 transition',
              kind === k ? 'border-featured bg-card-muted' : 'border-line',
              kindLocked && 'cursor-not-allowed opacity-60',
            )}
          >
            <input
              type="radio"
              value={k}
              disabled={kindLocked}
              className="mt-1 accent-[var(--featured)]"
              {...register('kind', { onChange: () => (touched.current.kind = true) })}
            />
            <span>
              <span className="block font-medium text-ink">{t(`kind.${k}`)}</span>
              <span className="block text-sm text-muted">{t(`kind.${k}Hint`)}</span>
            </span>
          </label>
        ))}
        {errors.kind && <p className="text-sm text-[var(--danger)]">{errors.kind.message}</p>}
      </fieldset>

      {kind !== 'expense_log' && (
        <div className="grid gap-5 sm:grid-cols-2">
          <Field label={t('profiles.provider')} error={errors.providerId?.message} optional>
            {(p) => (
              <Select {...p} {...register('providerId')}>
                <option value="">{t('profiles.noProvider')}</option>
                {(providers ?? []).map((pr) => (
                  <option key={pr.id} value={pr.id}>
                    {pr.shortName}
                    {pr.region ? ` — ${pr.region}` : ''}
                  </option>
                ))}
              </Select>
            )}
          </Field>
          <Field label={t('profiles.reference')} hint={t('profiles.referenceHint')} error={errors.referenceNumber?.message} optional>
            {(p) => <Input {...p} inputMode="text" autoComplete="off" {...register('referenceNumber')} />}
          </Field>
        </div>
      )}

      {kind === 'metered' && (
        <div className="grid gap-5 sm:grid-cols-3">
          <Field label={t('profiles.meterNumber')} error={errors.meterNumber?.message} optional>
            {(p) => <Input {...p} {...register('meterNumber')} />}
          </Field>
          <Field label={t('profiles.multiplyingFactor')} hint={t('profiles.multiplyingFactorHint')} error={errors.multiplyingFactor?.message}>
            {(p) => <Input {...p} inputMode="decimal" {...register('multiplyingFactor')} />}
          </Field>
          <Field label={t('profiles.unit')} error={errors.consumptionUnit?.message} optional>
            {(p) => <Input {...p} placeholder="kWh" {...register('consumptionUnit')} />}
          </Field>
        </div>
      )}

      {kind === 'recurring' && (
        <>
          <Field label={t('profiles.expectedAmount')} error={errors.expectedAmountMinor?.message} optional>
            {(p) => (
              <Controller
                control={control}
                name="expectedAmountMinor"
                render={({ field }) => <MoneyInput {...p} value={field.value} onChange={field.onChange} />}
              />
            )}
          </Field>
          <Controller
            control={control}
            name="autoCreateBill"
            render={({ field }) => <Toggle label={t('profiles.autoCreate')} checked={field.value} onChange={field.onChange} />}
          />
        </>
      )}

      {cycleBased && (
        <div className="grid gap-5 sm:grid-cols-2">
          <Field label={t('profiles.cycle')}>
            {(p) => (
              <Select {...p} {...register('billingCycle')}>
                {CYCLES.map((c) => (
                  <option key={c} value={c}>
                    {t(`profiles.cycles.${c}`)}
                  </option>
                ))}
              </Select>
            )}
          </Field>
          <Field label={t('profiles.dueDay')} hint={t('profiles.dueDayHint')} error={errors.typicalDueDay?.message} optional>
            {(p) => <Input {...p} type="number" min={1} max={31} inputMode="numeric" {...register('typicalDueDay')} />}
          </Field>
        </div>
      )}

      <fieldset>
        <legend className="mb-2 text-sm font-medium text-ink">{t('profiles.icon')}</legend>
        <div className="flex flex-wrap gap-2">
          {ICON_NAMES.map((name) => (
            <button
              key={name}
              type="button"
              aria-label={name}
              aria-pressed={icon === name}
              onClick={() => {
                touched.current.icon = true
                setValue('icon', name)
              }}
              className={cn(
                'grid size-11 place-items-center rounded-tile transition',
                icon === name ? 'bg-featured text-accent' : 'bg-card-muted text-ink hover:brightness-95',
              )}
            >
              <NamedIcon name={name} className="size-5" />
            </button>
          ))}
        </div>
      </fieldset>

      <fieldset>
        <legend className="mb-2 text-sm font-medium text-ink">{t('profiles.color')}</legend>
        <Controller
          control={control}
          name="color"
          render={({ field }) => (
            <div className="flex flex-wrap gap-3">
              {PALETTE.map((c) => (
                <button
                  key={c}
                  type="button"
                  aria-label={`${t('profiles.color')} ${c}`}
                  aria-pressed={field.value === c}
                  onClick={() => field.onChange(c)}
                  className={cn('size-10 rounded-full ring-offset-2 ring-offset-card transition', field.value === c && 'ring-2 ring-focus')}
                  style={{ backgroundColor: `var(--p${c})` }}
                />
              ))}
            </div>
          )}
        />
      </fieldset>

      <Controller control={control} name="pinned" render={({ field }) => <Toggle label={t('profiles.pinned')} checked={field.value} onChange={field.onChange} />} />

      <div className="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
        {onCancel && (
          <Button variant="secondary" onClick={onCancel}>
            {t('common.cancel')}
          </Button>
        )}
        <Button type="submit" loading={formState.isSubmitting}>
          {submitLabel}
        </Button>
      </div>
    </form>
  )
}
