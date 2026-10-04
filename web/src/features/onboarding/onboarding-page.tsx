import { useNavigate } from '@tanstack/react-router'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ApiError, type BillType } from '@/api/client'
import { useCreateProfile, useProviders, useTypes } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { Field, Input, Select } from '@/components/ui/field'
import { providerOptions } from '@/features/profiles/options'
import { Card, Chip, Skeleton } from '@/components/ui/misc'
import { NamedIcon } from '@/lib/icons'
import { suggestName } from '../profiles/profile-form'

// Types offered on first run (system type IDs from the seed migration).
const STARTER_TYPES = [
  'a0000000-0000-4000-8000-000000000001', // Electricity
  'a0000000-0000-4000-8000-000000000002', // Gas
  'a0000000-0000-4000-8000-000000000003', // Water
  'a0000000-0000-4000-8000-000000000004', // Internet
  'a0000000-0000-4000-8000-000000000005', // Mobile / Telephone
  'a0000000-0000-4000-8000-000000000011', // Rent
  'a0000000-0000-4000-8000-000000000012', // Maintenance
  'a0000000-0000-4000-8000-000000000023', // Subscriptions
]

interface Draft {
  name: string
  providerId: string
  referenceNumber: string
  error?: string
}

/** F7 — "What bills do you pay?": each chosen type becomes a profile. */
export function OnboardingPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data: types } = useTypes()
  const create = useCreateProfile()
  const [drafts, setDrafts] = useState<Record<string, Draft>>({})
  const [saving, setSaving] = useState(false)

  const starters = useMemo(
    () => STARTER_TYPES.map((id) => types?.find((ty) => ty.id === id)).filter((ty): ty is BillType => Boolean(ty)),
    [types],
  )
  const selected = starters.filter((ty) => drafts[ty.id])

  const toggle = (ty: BillType) =>
    setDrafts((d) => {
      const next = { ...d }
      if (next[ty.id]) delete next[ty.id]
      else next[ty.id] = { name: suggestName(ty), providerId: '', referenceNumber: '' }
      return next
    })

  const update = (id: string, patch: Partial<Draft>) => setDrafts((d) => ({ ...d, [id]: { ...d[id]!, ...patch } }))

  const submit = async () => {
    setSaving(true)
    let failed = false
    for (const ty of selected) {
      const d = drafts[ty.id]!
      try {
        await create.mutateAsync({
          name: d.name.trim() || suggestName(ty),
          typeId: ty.id,
          providerId: d.providerId || null,
          referenceNumber: d.referenceNumber.trim() || null,
        })
        setDrafts((cur) => {
          const next = { ...cur }
          delete next[ty.id]
          return next
        })
      } catch (err) {
        failed = true
        const msg = err instanceof ApiError ? (Object.values(err.fields)[0] ?? err.message) : String(err)
        update(ty.id, { error: msg })
      }
    }
    setSaving(false)
    if (!failed) {
      toast.success(t('onboarding.done'))
      await navigate({ to: '/' })
    }
  }

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6 py-4">
      <header>
        <h1 className="text-2xl font-semibold sm:text-[28px]">{t('onboarding.title')}</h1>
        <p className="mt-2 text-muted">{t('onboarding.subtitle')}</p>
      </header>

      {!types ? (
        <Skeleton className="h-24 rounded-card" />
      ) : (
        <div className="flex flex-wrap gap-2">
          {starters.map((ty) => (
            <Chip key={ty.id} selected={Boolean(drafts[ty.id])} onClick={() => toggle(ty)}>
              {!drafts[ty.id] && <NamedIcon name={ty.icon} className="size-4" />}
              {ty.name}
            </Chip>
          ))}
        </div>
      )}

      {selected.map((ty) => (
        <DraftCard key={ty.id} type={ty} draft={drafts[ty.id]!} onChange={(patch) => update(ty.id, patch)} />
      ))}

      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <Button variant="ghost" onClick={() => void navigate({ to: '/' })}>
          {t('onboarding.skip')}
        </Button>
        <Button disabled={selected.length === 0} loading={saving} onClick={() => void submit()}>
          {t('onboarding.create', { count: selected.length })}
        </Button>
      </div>
    </div>
  )
}

function DraftCard({ type, draft, onChange }: { type: BillType; draft: Draft; onChange: (patch: Partial<Draft>) => void }) {
  const { t } = useTranslation()
  const { data: providers } = useProviders(type.id)
  const hasProviders = (providers ?? []).length > 0 && type.defaultKind !== 'expense_log'
  return (
    <Card className="flex flex-col gap-4">
      <div className="flex items-center gap-2 font-semibold text-ink">
        <NamedIcon name={type.icon} className="size-5" />
        {type.name}
      </div>
      {draft.error && <p className="text-sm text-[var(--danger)]">{draft.error}</p>}
      <Field label={t('onboarding.namePlaceholder')}>
        {(p) => <Input {...p} value={draft.name} onChange={(e) => onChange({ name: e.target.value, error: undefined })} />}
      </Field>
      {hasProviders && (
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t('onboarding.provider')} optional>
            {(p) => (
              <Select {...p} value={draft.providerId} onChange={(v) => onChange({ providerId: v, error: undefined })} options={providerOptions(providers, t)} />
            )}
          </Field>
          <Field label={t('onboarding.reference')} optional>
            {(p) => <Input {...p} value={draft.referenceNumber} onChange={(e) => onChange({ referenceNumber: e.target.value, error: undefined })} />}
          </Field>
        </div>
      )}
    </Card>
  )
}
