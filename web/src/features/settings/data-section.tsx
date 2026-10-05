import { Archive, Download, FileSpreadsheet } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMe } from '@/api/hooks'
import { DateInput } from '@/components/ui/date-input'
import { Field } from '@/components/ui/field'
import { Card, Chip, SectionTitle } from '@/components/ui/misc'
import { todayISO } from '@/lib/dates'

type Period = 'thisYear' | 'lastYear' | '12m' | 'all' | 'custom'

/** The date range for a period choice; empty means everything. */
export function periodRange(period: Period, today = todayISO(), custom: { from?: string; to?: string } = {}): { from?: string; to?: string } {
  const year = Number(today.slice(0, 4))
  switch (period) {
    case 'thisYear':
      return { from: `${year}-01-01`, to: `${year}-12-31` }
    case 'lastYear':
      return { from: `${year - 1}-01-01`, to: `${year - 1}-12-31` }
    case '12m': {
      const d = new Date(year, Number(today.slice(5, 7)) - 1, 1)
      d.setMonth(d.getMonth() - 11)
      return { from: `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-01`, to: today }
    }
    case 'custom':
      return custom
    default:
      return {}
  }
}

function exportUrl(file: string, range: { from?: string; to?: string }) {
  const q = new URLSearchParams()
  if (range.from) q.set('from', range.from)
  if (range.to) q.set('to', range.to)
  const qs = q.toString()
  return `/api/v1/export/${file}${qs ? `?${qs}` : ''}`
}

const linkCls =
  'inline-flex h-12 items-center justify-center gap-2 rounded-full px-5 text-sm font-semibold transition aria-disabled:pointer-events-none aria-disabled:opacity-50'

/** "Your data": CSV exports for a period and the full backup (FR-EXP-01/02). */
export function DataSection() {
  const { t } = useTranslation()
  const { data: me } = useMe()
  const isOwner = me?.role === 'owner'
  const [period, setPeriod] = useState<Period>('thisYear')
  const [custom, setCustom] = useState<{ from?: string; to?: string }>({})
  const range = periodRange(period, todayISO(), custom)
  const badRange = period === 'custom' && Boolean(range.from && range.to && range.from > range.to)

  return (
    <Card className="flex flex-col gap-5">
      <SectionTitle>{t('data.title')}</SectionTitle>

      <div className="flex flex-col gap-3">
        <p className="text-sm text-body">{t('data.csvHint')}</p>
        <div className="-mx-1 flex flex-wrap gap-2 px-1" role="group" aria-label={t('data.period')}>
          {(['thisYear', 'lastYear', '12m', 'all', 'custom'] as const).map((p) => (
            <Chip key={p} className="h-9 px-3" selected={period === p} onClick={() => setPeriod(p)}>
              {t(`data.periods.${p}`)}
            </Chip>
          ))}
        </div>
        {period === 'custom' && (
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label={t('filters.fromDate')}>
              {(p) => <DateInput {...p} value={custom.from ?? ''} max={custom.to} onChange={(v) => setCustom({ ...custom, from: v || undefined })} />}
            </Field>
            <Field label={t('filters.toDate')}>
              {(p) => <DateInput {...p} value={custom.to ?? ''} min={custom.from} onChange={(v) => setCustom({ ...custom, to: v || undefined })} />}
            </Field>
          </div>
        )}
        <div className="flex flex-col gap-2 sm:flex-row">
          <a href={exportUrl('bills.csv', range)} download aria-disabled={badRange} className={`${linkCls} bg-featured text-on-featured hover:brightness-110`}>
            <FileSpreadsheet className="size-4" aria-hidden /> {t('data.bills')}
          </a>
          <a href={exportUrl('payments.csv', range)} download aria-disabled={badRange} className={`${linkCls} bg-card-muted text-ink hover:brightness-95`}>
            <FileSpreadsheet className="size-4" aria-hidden /> {t('data.payments')}
          </a>
        </div>
      </div>

      <div className="flex flex-col gap-3 border-t border-line pt-5">
        <div className="flex items-start gap-3">
          <Archive className="mt-0.5 size-5 shrink-0 text-muted" aria-hidden />
          <div className="flex flex-col gap-1">
            <p className="font-semibold text-ink">{t('data.backup')}</p>
            <p className="text-sm text-body">{t('data.backupHint')}</p>
            {!isOwner && <p className="text-sm text-muted">{t('data.backupOwnerOnly')}</p>}
          </div>
        </div>
        {isOwner && (
          <a href="/api/v1/export/backup.zip" download className={`${linkCls} self-start bg-card-muted text-ink hover:brightness-95`}>
            <Download className="size-4" aria-hidden /> {t('data.downloadBackup')}
          </a>
        )}
      </div>
    </Card>
  )
}
