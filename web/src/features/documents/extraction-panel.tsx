import { AlertTriangle, CheckCircle2, CircleDashed, ScanText } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { ExtractedField, Extraction } from '@/api/client'
import { useExtraction, useStartExtraction } from '@/api/hooks'
import type { Doc } from '@/api/client'
import { DeviceReadButton } from './device-read'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'
import { formatDate, formatMonth, formatMonthShort } from '@/lib/dates'
import { formatMoney } from '@/lib/money'

/** Field display order; anything else the extractor returns is shown after these. */
export const FIELD_ORDER = [
  'providerName', 'referenceNumber', 'billingMonth', 'issueDate', 'dueDate', 'payableMinor', 'payableAfterDueMinor',
  'currentChargesMinor', 'arrearsMinor', 'adjustmentsMinor', 'previousReading', 'currentReading', 'units',
  'multiplyingFactor', 'readingDate', 'billNumber', 'periodStart', 'periodEnd',
] as const

export const HIGH_CONFIDENCE = 0.85

/** Formats a normalised extracted value for display. */
export function formatFieldValue(name: string, value: string): string {
  if (name.endsWith('Minor')) return formatMoney(Number(value))
  if (name === 'billingMonth') return formatMonth(value)
  if (/^\d{4}-\d{2}-\d{2}$/.test(value)) return formatDate(value)
  return value
}

export function orderedFields(fields: Record<string, ExtractedField>): [string, ExtractedField][] {
  const known = FIELD_ORDER.filter((k) => fields[k]).map((k) => [k, fields[k]!] as [string, ExtractedField])
  const rest = Object.entries(fields).filter(([k]) => !(FIELD_ORDER as readonly string[]).includes(k))
  return [...known, ...rest]
}

/** "Read bill" action and the extracted result for one document (docs/05). */
export function ExtractionPanel({ documentId, doc }: { documentId: string; doc?: Doc }) {
  const { t } = useTranslation()
  const extraction = useExtraction(documentId)
  const start = useStartExtraction()
  const e = extraction.data
  const busy = start.isPending || e?.status === 'queued' || e?.status === 'running'

  const run = async () => {
    try {
      await start.mutateAsync(documentId)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errorTitle'))
    }
  }

  return (
    <section className="flex flex-col gap-3 rounded-field border border-line p-4" aria-labelledby={`extract-${documentId}`}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 id={`extract-${documentId}`} className="flex items-center gap-2 font-semibold text-ink">
          <ScanText className="size-5" aria-hidden /> {t('extraction.title')}
        </h3>
        <Button size="sm" variant={e ? 'ghost' : 'primary'} loading={busy} onClick={() => void run()}>
          {busy ? t('extraction.reading') : e ? t('extraction.readAgain') : t('extraction.read')}
        </Button>
      </div>

      {!e && !busy && <p className="text-sm text-muted">{t('extraction.hint')}</p>}
      {busy && <p className="text-sm text-muted" role="status">{t('extraction.inProgress')}</p>}
      {e?.status === 'failed' && (
        <p role="alert" className="flex items-start gap-2 text-sm text-[var(--danger)]">
          <AlertTriangle className="mt-0.5 size-4 shrink-0" aria-hidden /> {e.error}
        </p>
      )}
      {doc && !busy && e?.status !== 'succeeded' && (
        <div className="flex flex-col items-start gap-1">
          <DeviceReadButton doc={doc} className="h-10" />
          <p className="text-xs text-muted">{t('deviceRead.hint')}</p>
        </div>
      )}
      {e?.status === 'succeeded' && <ExtractionResult extraction={e} />}
    </section>
  )
}

function ExtractionResult({ extraction: e }: { extraction: Extraction }) {
  const { t } = useTranslation()
  const used = e.tiers.filter((tier) => tier.outcome === 'used').map((tier) => t(`extraction.tier.${tier.tier}`, { defaultValue: tier.tier }))
  const fields = orderedFields(e.fields)

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-muted">{t('extraction.readWith', { tiers: used.join(' + ') })}</p>

      {e.warnings.length > 0 && (
        <ul className="flex flex-col gap-1 rounded-field bg-[var(--st-soon-bg)] text-[var(--st-soon-fg)] p-3 text-sm">
          {e.warnings.map((w, i) => (
            <li key={i} className="flex gap-2">
              <AlertTriangle className="mt-0.5 size-4 shrink-0" aria-hidden />
              <span>
                {w.field && <strong>{t(`extraction.fields.${w.field}`, { defaultValue: w.field })}: </strong>}
                {w.message}
              </span>
            </li>
          ))}
        </ul>
      )}

      <dl className="tabular divide-y divide-line text-sm">
        {fields.map(([name, f]) => (
          <div key={name} className="flex items-center justify-between gap-4 py-2">
            <dt className="text-muted">{t(`extraction.fields.${name}`, { defaultValue: name })}</dt>
            <dd className="flex items-center gap-2 text-right text-ink">
              {formatFieldValue(name, f.value)}
              <ConfidenceMark confidence={f.confidence} />
            </dd>
          </div>
        ))}
      </dl>

      {e.lateFees.length > 1 && (
        <div>
          <h4 className="mb-1 text-sm font-semibold text-ink">{t('extraction.lateFees')}</h4>
          <ul className="text-sm">
            {e.lateFees.map((f, i) => (
              <li key={i} className="tabular flex justify-between py-1">
                <span className="text-muted">{f.until ? t('extraction.until', { date: formatDate(f.until) }) : t('extraction.afterThat')}</span>
                <span className="text-ink">{formatMoney(f.amountMinor)}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {e.history.length > 0 && (
        <div>
          <h4 className="mb-1 text-sm font-semibold text-ink">{t('extraction.history', { count: e.history.length })}</h4>
          <div className="relative overflow-x-auto">
            <table className="tabular w-full text-sm">
              <thead className="text-left text-muted">
                <tr>
                  <th className="py-1 font-medium">{t('extraction.month')}</th>
                  <th className="py-1 text-right font-medium">{t('extraction.units')}</th>
                  <th className="py-1 text-right font-medium">{t('extraction.billed')}</th>
                  <th className="py-1 text-right font-medium">{t('extraction.paid')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {e.history.map((h) => (
                  <tr key={h.month}>
                    <td className="py-1">{formatMonthShort(h.month)}</td>
                    <td className="py-1 text-right">{h.units ?? '—'}</td>
                    <td className="py-1 text-right">{h.billedMinor != null ? formatMoney(h.billedMinor) : '—'}</td>
                    <td className="py-1 text-right">{h.paidMinor != null ? formatMoney(h.paidMinor) : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}

function ConfidenceMark({ confidence }: { confidence: number }) {
  const { t } = useTranslation()
  const high = confidence >= HIGH_CONFIDENCE
  const Icon = high ? CheckCircle2 : CircleDashed
  return (
    <span title={t('extraction.confidence', { pct: Math.round(confidence * 100) })} className={cn('inline-flex', high ? 'text-[var(--st-paid-fg)]' : 'text-[var(--st-soon-fg)]')}>
      <Icon className="size-4" aria-hidden />
      <span className="sr-only">{high ? t('extraction.sure') : t('extraction.check')}</span>
    </span>
  )
}
