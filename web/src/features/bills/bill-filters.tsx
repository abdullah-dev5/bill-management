import { SlidersHorizontal, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { BillType, Provider } from '@/api/client'
import { useProviders, useTypes, type BillFilters, type BillSource } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { DateInput } from '@/components/ui/date-input'
import { Field, Select } from '@/components/ui/field'
import { Chip } from '@/components/ui/misc'
import { MoneyInput } from '@/components/ui/money-input'
import { Sheet } from '@/components/ui/sheet'
import { formatDate } from '@/lib/dates'
import { formatMoney } from '@/lib/money'
import type { BillsSearch } from '@/router'

const SOURCES: BillSource[] = ['camera', 'upload', 'manual']
const split = (v?: string) => (v ? v.split(',').filter(Boolean) : [])

/** The URL's extra filters as API filters (amounts are whole rupees in the URL, minor units in the API). */
export function apiFilters(s: BillsSearch): Partial<BillFilters> & { from?: string; to?: string } {
  return {
    typeIds: split(s.type),
    providerId: s.provider,
    minAmount: s.min != null ? Math.round(s.min * 100) : undefined,
    maxAmount: s.max != null ? Math.round(s.max * 100) : undefined,
    sources: split(s.source) as BillSource[],
    from: s.from,
    to: s.to,
  }
}

type Extra = Pick<BillsSearch, 'type' | 'provider' | 'min' | 'max' | 'source' | 'from' | 'to'>
const EMPTY: Extra = { type: undefined, provider: undefined, min: undefined, max: undefined, source: undefined, from: undefined, to: undefined }

/** "Filters" button + sheet + removable chips for the active filters (FR-SRCH-03/04). */
export function BillFilterBar({ search, onChange }: { search: BillsSearch; onChange: (patch: Extra) => void }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const { data: types } = useTypes()
  const { data: providers } = useProviders()
  const chips = activeChips(search, types ?? [], providers ?? [], t)

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <Button variant="secondary" className="h-11" onClick={() => setOpen(true)} aria-haspopup="dialog">
          <SlidersHorizontal className="size-4" aria-hidden />
          {t('filters.button')}
          {chips.length > 0 && (
            <span className="ml-1 grid size-5 place-items-center rounded-full bg-featured text-xs text-on-featured" aria-label={t('filters.count', { count: chips.length })}>
              {chips.length}
            </span>
          )}
        </Button>
        {chips.length > 0 && (
          <ul className="flex flex-wrap items-center gap-2" aria-label={t('filters.active')}>
            {chips.map((c) => (
              <li key={c.key}>
                <button
                  type="button"
                  onClick={() => onChange(c.remove)}
                  aria-label={t('filters.remove', { name: c.label })}
                  className="inline-flex h-9 items-center gap-1.5 rounded-full bg-featured pr-2 pl-3 text-sm font-semibold text-on-featured hover:brightness-110"
                >
                  {c.label}
                  <X className="size-4" aria-hidden />
                </button>
              </li>
            ))}
            <li>
              <button type="button" onClick={() => onChange(EMPTY)} className="h-9 px-2 text-sm font-semibold text-ink underline-offset-4 hover:underline">
                {t('filters.clear')}
              </button>
            </li>
          </ul>
        )}
      </div>
      {open && (
        <FilterSheet
          search={search}
          types={types ?? []}
          providers={providers ?? []}
          onClose={() => setOpen(false)}
          onApply={(v) => {
            onChange(v)
            setOpen(false)
          }}
        />
      )}
    </div>
  )
}

function FilterSheet({
  search,
  types,
  providers,
  onClose,
  onApply,
}: {
  search: BillsSearch
  types: BillType[]
  providers: Provider[]
  onClose: () => void
  onApply: (v: Extra) => void
}) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<Extra>({
    type: search.type, provider: search.provider, min: search.min, max: search.max, source: search.source, from: search.from, to: search.to,
  })
  const typeIds = split(draft.type)
  const sources = split(draft.source)
  const toggle = (list: string[], v: string) => (list.includes(v) ? list.filter((x) => x !== v) : [...list, v]).join(',') || undefined
  const topLevel = types.filter((ty) => !ty.archived && !ty.parentId)
  const badAmounts = draft.min != null && draft.max != null && draft.min > draft.max

  return (
    <Sheet
      open
      onOpenChange={(v) => !v && onClose()}
      title={t('filters.title')}
      footer={
        <>
          <Button variant="secondary" onClick={() => setDraft(EMPTY)}>
            {t('filters.clear')}
          </Button>
          <Button disabled={badAmounts} onClick={() => onApply(draft)}>
            {t('filters.apply')}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-6">
        <fieldset>
          <legend className="mb-2 text-sm font-medium text-ink">{t('filters.type')}</legend>
          {(['utilities', 'household', 'other'] as const).map((g) => {
            const items = topLevel.filter((ty) => ty.group === g)
            if (!items.length) return null
            return (
              <div key={g} className="mb-3">
                <p className="mb-1.5 text-xs font-semibold tracking-wide text-muted uppercase">{t(`group.${g}`)}</p>
                <div className="flex flex-wrap gap-2">
                  {items.map((ty) => (
                    <Chip key={ty.id} className="h-9 px-3" selected={typeIds.includes(ty.id)} onClick={() => setDraft({ ...draft, type: toggle(typeIds, ty.id) })}>
                      {ty.name}
                    </Chip>
                  ))}
                </div>
              </div>
            )
          })}
        </fieldset>

        <Field label={t('filters.provider')}>
          {(p) => (
            <Select
              {...p}
              value={draft.provider ?? ''}
              onChange={(v) => setDraft({ ...draft, provider: v || undefined })}
              options={[
                { value: '', label: t('filters.anyProvider') },
                ...providers.map((pr) => ({
                  value: pr.id,
                  label: pr.region ? `${pr.shortName} — ${pr.region}` : pr.shortName,
                  hint: pr.name !== pr.shortName ? pr.name : undefined,
                  group: types.find((ty) => ty.id === pr.typeId)?.name,
                })),
              ]}
            />
          )}
        </Field>

        <fieldset className="flex flex-col gap-2">
          <legend className="mb-2 text-sm font-medium text-ink">{t('filters.amount')}</legend>
          <div className="grid grid-cols-2 gap-3">
            <Field label={t('filters.minAmount')}>
              {(p) => (
                <MoneyInput {...p} value={draft.min != null ? draft.min * 100 : null} onChange={(v) => setDraft({ ...draft, min: v != null ? v / 100 : undefined })} />
              )}
            </Field>
            <Field label={t('filters.maxAmount')} error={badAmounts ? t('validation.amountRange') : undefined}>
              {(p) => (
                <MoneyInput {...p} value={draft.max != null ? draft.max * 100 : null} onChange={(v) => setDraft({ ...draft, max: v != null ? v / 100 : undefined })} />
              )}
            </Field>
          </div>
          <p className="text-sm text-muted">{t('filters.amountHint')}</p>
        </fieldset>

        <fieldset>
          <legend className="mb-2 text-sm font-medium text-ink">{t('filters.date')}</legend>
          <div className="grid grid-cols-2 gap-3">
            <Field label={t('filters.fromDate')}>
              {(p) => <DateInput {...p} value={draft.from ?? ''} max={draft.to} onChange={(v) => setDraft({ ...draft, from: v || undefined })} />}
            </Field>
            <Field label={t('filters.toDate')}>
              {(p) => <DateInput {...p} value={draft.to ?? ''} min={draft.from} onChange={(v) => setDraft({ ...draft, to: v || undefined })} />}
            </Field>
          </div>
        </fieldset>

        <fieldset>
          <legend className="mb-2 text-sm font-medium text-ink">{t('filters.source')}</legend>
          <div className="flex flex-wrap gap-2">
            {SOURCES.map((src) => (
              <Chip key={src} className="h-9 px-3" selected={sources.includes(src)} onClick={() => setDraft({ ...draft, source: toggle(sources, src) })}>
                {t(`filters.sources.${src}`)}
              </Chip>
            ))}
          </div>
        </fieldset>
      </div>
    </Sheet>
  )
}

function activeChips(s: BillsSearch, types: BillType[], providers: Provider[], t: ReturnType<typeof useTranslation>['t']) {
  const chips: { key: string; label: string; remove: Extra }[] = []
  const typeIds = split(s.type)
  for (const id of typeIds) {
    const rest = typeIds.filter((x) => x !== id).join(',') || undefined
    chips.push({ key: `type-${id}`, label: types.find((ty) => ty.id === id)?.name ?? '…', remove: { type: rest } })
  }
  if (s.provider) chips.push({ key: 'provider', label: providers.find((p) => p.id === s.provider)?.shortName ?? '…', remove: { provider: undefined } })
  if (s.min != null) chips.push({ key: 'min', label: t('filters.amountFrom', { amount: formatMoney(s.min * 100) }), remove: { min: undefined } })
  if (s.max != null) chips.push({ key: 'max', label: t('filters.amountTo', { amount: formatMoney(s.max * 100) }), remove: { max: undefined } })
  if (s.from) chips.push({ key: 'from', label: t('filters.dateFrom', { date: formatDate(s.from) }), remove: { from: undefined } })
  if (s.to) chips.push({ key: 'to', label: t('filters.dateTo', { date: formatDate(s.to) }), remove: { to: undefined } })
  const sources = split(s.source)
  for (const src of sources) {
    const rest = sources.filter((x) => x !== src).join(',') || undefined
    chips.push({ key: `source-${src}`, label: t(`filters.sources.${src as BillSource}`), remove: { source: rest } })
  }
  return chips
}
