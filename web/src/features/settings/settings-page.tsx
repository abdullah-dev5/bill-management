import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Archive, Pencil, Plus } from 'lucide-react'
import { useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api, unwrap, type BillType, type Household, type ProfileKind } from '@/api/client'
import { useArchiveType, useHousehold, useMe, useSaveType, useTypes, useUpdateHousehold } from '@/api/hooks'
import { PageHeader } from '@/components/layout/app-shell'
import { Button } from '@/components/ui/button'
import { Field, Input, Select, Toggle } from '@/components/ui/field'
import { Card, Chip, SectionTitle, Skeleton } from '@/components/ui/misc'
import { Sheet } from '@/components/ui/sheet'
import { applyServerErrors } from '@/lib/forms'
import { formatBytes } from '@/lib/image'
import { ICON_NAMES, NamedIcon } from '@/lib/icons'
import { applyTheme, getTheme, type ThemeChoice } from '@/lib/theme'
import { cn } from '@/lib/cn'
import { DataSection } from './data-section'

const TIMEZONES = ['Asia/Karachi', 'Asia/Dubai', 'Asia/Riyadh', 'Europe/London', 'America/New_York', 'UTC']

export function SettingsPage() {
  const { t } = useTranslation()
  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <PageHeader title={t('settings.title')} />
      <HouseholdSection />
      <PrivacySection />
      <StorageSection />
      <DataSection />
      <AppearanceSection />
      <TypesSection />
      <AccountSection />
    </div>
  )
}

function HouseholdSection() {
  const { data: me } = useMe()
  const { data: hh } = useHousehold()
  const isOwner = me?.role === 'owner'
  if (!hh) return <Skeleton className="h-56 rounded-card" />
  return <HouseholdForm hh={hh} isOwner={isOwner} />
}

// Mounted once the household has loaded, so every field (including the dropdowns) starts with its value.
function HouseholdForm({ hh, isOwner }: { hh: Household; isOwner: boolean }) {
  const { t } = useTranslation()
  const save = useUpdateHousehold()
  const form = useForm({ defaultValues: { name: hh.name, currency: hh.currency, timezone: hh.timezone } })
  const timezones = TIMEZONES.includes(hh.timezone) ? TIMEZONES : [hh.timezone, ...TIMEZONES]

  return (
    <Card>
      <SectionTitle>{t('settings.household')}</SectionTitle>
      {!isOwner && <p className="mb-4 text-sm text-muted">{t('settings.ownerOnly')}</p>}
      <form
        className="flex flex-col gap-4"
        noValidate
        onSubmit={form.handleSubmit(async (v) => {
          try {
            await save.mutateAsync(v)
            toast.success(t('settings.householdSaved'))
          } catch (err) {
            applyServerErrors(err, form.setError)
          }
        })}
      >
        <Field label={t('settings.householdName')} error={form.formState.errors.name?.message}>
          {(p) => <Input {...p} disabled={!isOwner} {...form.register('name', { required: t('validation.required') })} />}
        </Field>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t('settings.currency')} error={form.formState.errors.currency?.message}>
            {(p) => (
              <Controller
                control={form.control}
                name="currency"
                render={({ field }) => <Select {...p} {...field} disabled={!isOwner} options={[{ value: 'PKR', label: t('settings.pkr') }]} />}
              />
            )}
          </Field>
          <Field label={t('settings.timezone')} error={form.formState.errors.timezone?.message}>
            {(p) => (
              <Controller
                control={form.control}
                name="timezone"
                render={({ field }) => <Select {...p} {...field} disabled={!isOwner} options={timezones.map((tz) => ({ value: tz, label: tz.replace(/_/g, ' ') }))} />}
              />
            )}
          </Field>
        </div>
        {isOwner && (
          <Button type="submit" className="self-end" loading={form.formState.isSubmitting}>
            {t('common.save')}
          </Button>
        )}
      </form>
    </Card>
  )
}

function PrivacySection() {
  const { t } = useTranslation()
  const { data: me } = useMe()
  const { data: hh } = useHousehold()
  const save = useUpdateHousehold()
  if (!hh) return <Skeleton className="h-40 rounded-card" />
  const isOwner = me?.role === 'owner'

  const setConsent = async (on: boolean) => {
    try {
      await save.mutateAsync({ name: hh.name, currency: hh.currency, timezone: hh.timezone, aiExtractionConsent: on })
      toast.success(on ? t('settings.aiConsentOn') : t('settings.aiConsentOff'))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errorTitle'))
    }
  }

  return (
    <Card>
      <SectionTitle>{t('settings.privacy')}</SectionTitle>
      {!isOwner && <p className="mb-3 text-sm text-muted">{t('settings.ownerOnly')}</p>}
      <Toggle
        label={t('settings.aiConsent')}
        hint={t('settings.aiConsentHint')}
        checked={hh.aiExtractionConsent}
        disabled={!isOwner || save.isPending}
        onChange={(on) => void setConsent(on)}
      />
      <p className="mt-3 rounded-field bg-card-muted p-4 text-sm text-body">{t('settings.aiConsentNotice')}</p>
    </Card>
  )
}

function StorageSection() {
  const { t } = useTranslation()
  const { data: hh } = useHousehold()
  if (!hh) return <Skeleton className="h-28 rounded-card" />
  const { usedBytes, quotaBytes, documentCount } = hh.storage
  const fraction = Math.min(1, usedBytes / quotaBytes)
  return (
    <Card>
      <SectionTitle>{t('settings.storage')}</SectionTitle>
      <div
        className="h-2.5 overflow-hidden rounded-full bg-card-muted"
        role="meter"
        aria-label={t('settings.storage')}
        aria-valuemin={0}
        aria-valuemax={quotaBytes}
        aria-valuenow={usedBytes}
      >
        <div className={cn('h-full rounded-full', fraction > 0.9 ? 'bg-[var(--danger)]' : 'bg-featured')} style={{ width: `${Math.max(fraction * 100, usedBytes ? 1 : 0)}%` }} />
      </div>
      <p className="mt-2 text-sm text-muted">
        {t('settings.storageUsed', { used: formatBytes(usedBytes), quota: formatBytes(quotaBytes), count: documentCount })}
      </p>
      {fraction > 0.9 && <p className="mt-1 text-sm font-medium text-[var(--danger)]">{t('settings.storageFull')}</p>}
    </Card>
  )
}

function AppearanceSection() {
  const { t } = useTranslation()
  const [theme, setTheme] = useState<ThemeChoice>(getTheme)
  return (
    <Card>
      <SectionTitle>{t('settings.appearance')}</SectionTitle>
      <div className="flex flex-wrap gap-2" role="group">
        {(['system', 'light', 'dark'] as const).map((c) => (
          <Chip
            key={c}
            selected={theme === c}
            onClick={() => {
              setTheme(c)
              applyTheme(c)
            }}
          >
            {t(`settings.theme.${c}`)}
          </Chip>
        ))}
      </div>
    </Card>
  )
}

function TypesSection() {
  const { t } = useTranslation()
  const { data: types } = useTypes()
  const archive = useArchiveType()
  const [editing, setEditing] = useState<BillType | 'new' | null>(null)
  const custom = (types ?? []).filter((ty) => !ty.isSystem && !ty.archived)
  const system = (types ?? []).filter((ty) => ty.isSystem && !ty.parentId)

  return (
    <Card>
      <SectionTitle
        action={
          <Button size="sm" variant="secondary" icon={<Plus className="size-4" />} onClick={() => setEditing('new')}>
            {t('settings.newType')}
          </Button>
        }
      >
        {t('settings.types')}
      </SectionTitle>
      <p className="mb-4 text-sm text-muted">{t('settings.typesHint')}</p>
      {custom.length > 0 && (
        <ul className="mb-4 divide-y divide-line">
          {custom.map((ty) => (
            <li key={ty.id} className="flex items-center gap-3 py-2">
              <NamedIcon name={ty.icon} className="size-5 text-ink" />
              <span className="flex-1 font-medium text-ink">{ty.name}</span>
              <span className="text-sm text-muted">{t(`kind.${ty.defaultKind}`)}</span>
              <Button size="icon" variant="ghost" aria-label={t('common.edit')} onClick={() => setEditing(ty)}>
                <Pencil className="size-4" />
              </Button>
              <Button
                size="icon"
                variant="ghost"
                aria-label={t('common.archive')}
                onClick={async () => {
                  try {
                    await archive.mutateAsync(ty.id)
                    toast.success(t('settings.typeArchived'))
                  } catch (err) {
                    toast.error(err instanceof Error ? err.message : t('common.errorTitle'))
                  }
                }}
              >
                <Archive className="size-4" />
              </Button>
            </li>
          ))}
        </ul>
      )}
      <div className="flex flex-wrap gap-2">
        {system.map((ty) => (
          <span key={ty.id} className="inline-flex items-center gap-1.5 rounded-full bg-card-muted px-3 py-1.5 text-sm text-body">
            <NamedIcon name={ty.icon} className="size-4" />
            {ty.name}
          </span>
        ))}
      </div>
      {editing && <TypeSheet type={editing === 'new' ? undefined : editing} types={types ?? []} onClose={() => setEditing(null)} />}
    </Card>
  )
}

function TypeSheet({ type, types, onClose }: { type?: BillType; types: BillType[]; onClose: () => void }) {
  const { t } = useTranslation()
  const save = useSaveType()
  const form = useForm({
    defaultValues: {
      name: type?.name ?? '',
      icon: type?.icon ?? 'receipt',
      defaultKind: (type?.defaultKind ?? 'expense_log') as ProfileKind,
      group: type?.group ?? 'other',
      parentId: type?.parentId ?? '',
    },
  })
  const icon = form.watch('icon')
  const parents = types.filter((ty) => !ty.parentId && !ty.archived && ty.id !== type?.id)

  const submit = form.handleSubmit(async (v) => {
    try {
      await save.mutateAsync({ id: type?.id, body: { ...v, name: v.name.trim(), parentId: v.parentId || null } })
      toast.success(t('settings.typeSaved'))
      onClose()
    } catch (err) {
      applyServerErrors(err, form.setError)
    }
  })

  return (
    <Sheet
      open
      onOpenChange={(o) => !o && onClose()}
      title={type ? type.name : t('settings.newType')}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button loading={form.formState.isSubmitting} onClick={() => void submit()}>
            {t('common.save')}
          </Button>
        </>
      }
    >
      <form onSubmit={submit} className="flex flex-col gap-4" noValidate>
        <Field label={t('settings.typeName')} error={form.formState.errors.name?.message}>
          {(p) => <Input {...p} {...form.register('name', { required: t('validation.required') })} />}
        </Field>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t('settings.defaultKind')}>
            {(p) => (
              <Controller
                control={form.control}
                name="defaultKind"
                render={({ field }) => (
                  <Select
                    {...p}
                    {...field}
                    options={(['metered', 'recurring', 'expense_log'] as const).map((k) => ({ value: k, label: t(`kind.${k}`), hint: t(`kind.${k}Hint`) }))}
                  />
                )}
              />
            )}
          </Field>
          <Field label={t('settings.typeGroup')}>
            {(p) => (
              <Controller
                control={form.control}
                name="group"
                render={({ field }) => <Select {...p} {...field} options={(['utilities', 'household', 'other'] as const).map((g) => ({ value: g, label: t(`group.${g}`) }))} />}
              />
            )}
          </Field>
        </div>
        <Field label={t('settings.parentType')} error={form.formState.errors.parentId?.message} optional>
          {(p) => (
            <Controller
              control={form.control}
              name="parentId"
              render={({ field }) => (
                <Select {...p} {...field} options={[{ value: '', label: t('settings.noParent') }, ...parents.map((ty) => ({ value: ty.id, label: ty.name }))]} />
              )}
            />
          )}
        </Field>
        <fieldset>
          <legend className="mb-2 text-sm font-medium text-ink">{t('profiles.icon')}</legend>
          <div className="flex flex-wrap gap-2">
            {ICON_NAMES.map((name) => (
              <button
                key={name}
                type="button"
                aria-label={name}
                aria-pressed={icon === name}
                onClick={() => form.setValue('icon', name)}
                className={cn('grid size-10 place-items-center rounded-tile', icon === name ? 'bg-featured text-accent' : 'bg-card-muted text-ink')}
              >
                <NamedIcon name={name} className="size-5" />
              </button>
            ))}
          </div>
        </fieldset>
      </form>
    </Sheet>
  )
}

function AccountSection() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { data: me } = useMe()
  const [confirm, setConfirm] = useState(false)
  const [busy, setBusy] = useState(false)

  const leave = async (call: () => Promise<unknown>) => {
    setBusy(true)
    try {
      await call()
      qc.clear()
      await navigate({ to: '/sign-in' })
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errorTitle'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <SectionTitle>{t('settings.account')}</SectionTitle>
      {me && <p className="mb-4 text-sm text-muted">{t('settings.signedInAs', { email: me.user.email })}</p>}
      <div className="flex flex-col gap-2 sm:flex-row">
        <Button variant="secondary" loading={busy} onClick={() => void leave(() => unwrap(api.POST('/auth/logout')))}>
          {t('settings.signOut')}
        </Button>
        <Button variant="outline" disabled={busy} onClick={() => void leave(() => unwrap(api.POST('/auth/logout-all')))}>
          {t('settings.signOutAll')}
        </Button>
        <Button variant="ghost" className="text-[var(--danger)] sm:ml-auto" onClick={() => setConfirm(true)}>
          {t('settings.deleteAccount')}
        </Button>
      </div>
      <Sheet
        open={confirm}
        onOpenChange={setConfirm}
        title={t('settings.deleteAccountTitle')}
        description={t('settings.deleteAccountBody')}
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirm(false)}>
              {t('common.cancel')}
            </Button>
            <Button variant="danger" loading={busy} onClick={() => void leave(() => unwrap(api.DELETE('/account')))}>
              {t('settings.deleteAccountConfirm')}
            </Button>
          </>
        }
      >
        {null}
      </Sheet>
    </Card>
  )
}
