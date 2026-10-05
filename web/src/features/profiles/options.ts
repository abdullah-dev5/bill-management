import type { TFunction } from 'i18next'
import type { Profile, Provider } from '@/api/client'
import type { SelectOption } from '@/components/ui/field'

/** Providers as "LESCO — Lahore" with the full name underneath; the household's own ones listed last. */
export function providerOptions(providers: Provider[] | undefined, t: TFunction): SelectOption[] {
  const list = [...(providers ?? [])].sort((a, b) => Number(a.isCustom) - Number(b.isCustom))
  return [
    { value: '', label: t('profiles.noProvider'), hint: t('profiles.noProviderHint') },
    ...list.map((p) => ({
      value: p.id,
      label: p.region ? `${p.shortName} — ${p.region}` : p.shortName,
      hint: p.name !== p.shortName ? p.name : undefined,
      group: p.isCustom ? t('profiles.customProviders') : t('profiles.catalogueProviders'),
    })),
  ]
}

/** Profiles grouped by their bill type's group (Utilities / Household / Other), in the given order. */
export function profileOptions(profiles: Profile[] | undefined, t: TFunction, groupOf?: (p: Profile) => string | undefined): SelectOption[] {
  return (profiles ?? []).map((p) => {
    const g = groupOf?.(p)
    return { value: p.id, label: p.name, group: g ? t(`group.${g as 'utilities'}`) : undefined }
  })
}
