import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { FileText, KeyRound, Search, X } from 'lucide-react'
import { Fragment, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Schemas } from '@/api/client'
import { useProfiles, useSearchResults } from '@/api/hooks'
import { PageHeader } from '@/components/layout/app-shell'
import { Card, Chip, EmptyState, ErrorNotice, IconTile, SectionTitle, Skeleton } from '@/components/ui/misc'
import { cn } from '@/lib/cn'
import { formatDate, formatMonth } from '@/lib/dates'
import { tr } from '@/lib/tr'
import { BillRow } from '../bills/bill-row'

type Interpretation = Schemas['SearchInterpretation']

const EXAMPLES = ['electricity september 2025', 'gas last winter', 'unpaid', 'internet this year', 'maintenance 2026']

/** Global search (FR-SRCH-01/02): profiles, bills and document text, with phrases understood as filters. */
export function SearchPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { q = '' } = useSearch({ from: '/app/search' })
  const [text, setText] = useState(q)
  const input = useRef<HTMLInputElement>(null)
  const results = useSearchResults(q)
  const { data: profiles } = useProfiles(true)
  const byId = useMemo(() => new Map((profiles ?? []).map((p) => [p.id, p])), [profiles])

  // Keep the URL in step with the box (debounced), so a search can be shared or refreshed.
  useEffect(() => {
    const next = text.trim()
    if (next === q) return
    const id = setTimeout(() => void navigate({ to: '/search', search: next ? { q: next } : {}, replace: true }), 250)
    return () => clearTimeout(id)
  }, [text, q, navigate])

  const data = results.data
  const total = data ? data.profiles.length + data.bills.length + data.documents.length : 0

  return (
    <div className="flex flex-col gap-5">
      <PageHeader title={t('search.title')} />
      <form
        role="search"
        onSubmit={(e) => {
          e.preventDefault()
          void navigate({ to: '/search', search: text.trim() ? { q: text.trim() } : {}, replace: true })
        }}
        className="relative"
      >
        <Search className="pointer-events-none absolute top-1/2 left-4 size-5 -translate-y-1/2 text-muted" aria-hidden />
        <input
          ref={input}
          type="search"
          autoFocus
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder={t('search.placeholder')}
          aria-label={t('search.label')}
          maxLength={200}
          className="h-14 w-full rounded-full border border-line bg-card pr-12 pl-12 text-base text-ink placeholder:text-muted/80 focus:border-focus focus:outline-2 focus:outline-focus [&::-webkit-search-cancel-button]:hidden"
        />
        {text && (
          <button
            type="button"
            aria-label={t('search.clear')}
            onClick={() => {
              setText('')
              input.current?.focus()
            }}
            className="absolute top-1/2 right-2 grid size-10 -translate-y-1/2 place-items-center rounded-full text-muted hover:bg-card-muted hover:text-ink"
          >
            <X className="size-5" aria-hidden />
          </button>
        )}
      </form>

      {!q ? (
        <Card className="flex flex-col gap-3">
          <p className="text-sm text-body">{t('search.intro')}</p>
          <div className="flex flex-wrap gap-2">
            {EXAMPLES.map((ex) => (
              <Chip key={ex} className="h-9 px-3 font-medium" onClick={() => setText(ex)}>
                {ex}
              </Chip>
            ))}
          </div>
        </Card>
      ) : results.isError ? (
        <ErrorNotice error={results.error} onRetry={() => void results.refetch()} />
      ) : !data ? (
        <div className="flex flex-col gap-3">
          <Skeleton className="h-6 w-64" />
          <Skeleton className="h-40 rounded-card" />
        </div>
      ) : (
        <div className={cn('flex flex-col gap-6 transition-opacity', results.isFetching && 'opacity-70')} aria-busy={results.isFetching}>
          <Understood interpretation={data.interpretation} />
          <p className="sr-only" role="status">
            {t('search.resultCount', { count: total })}
          </p>
          {total === 0 ? (
            <Card>
              <EmptyState icon={<Search className="size-10" />} title={t('search.noneTitle')} body={t('search.noneBody')} />
            </Card>
          ) : (
            <>
              {data.profiles.length > 0 && (
                <section>
                  <SectionTitle>{t('search.profiles')}</SectionTitle>
                  <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                    {data.profiles.map((p) => (
                      <Link
                        key={p.id}
                        to="/profiles/$profileId"
                        params={{ profileId: p.id }}
                        className="flex items-center gap-3 rounded-card bg-card p-4 transition hover:bg-card-muted/60"
                      >
                        <IconTile icon={p.icon} color={p.color} />
                        <span className="flex min-w-0 flex-col">
                          <span className="truncate font-semibold text-ink">{p.name}</span>
                          <span className="truncate text-sm text-muted">
                            {[p.typeName, p.providerName, p.referenceDisplay].filter(Boolean).join(' · ')}
                          </span>
                        </span>
                      </Link>
                    ))}
                  </div>
                </section>
              )}
              {data.bills.length > 0 && (
                <section>
                  <SectionTitle>{t('search.bills', { count: data.bills.length })}</SectionTitle>
                  <ul className="divide-y divide-line overflow-hidden rounded-card bg-card">
                    {data.bills.map((b) => (
                      <li key={b.id}>
                        <BillRow bill={b} profile={byId.get(b.profileId)} />
                      </li>
                    ))}
                  </ul>
                  {data.bills.length >= 50 && <p className="mt-2 text-sm text-muted">{t('search.narrow')}</p>}
                </section>
              )}
              {data.documents.length > 0 && (
                <section>
                  <SectionTitle>{t('search.documents')}</SectionTitle>
                  <ul className="divide-y divide-line overflow-hidden rounded-card bg-card">
                    {data.documents.map((d) => (
                      <li key={d.id}>
                        <DocumentHit doc={d} terms={data.interpretation.terms} />
                      </li>
                    ))}
                  </ul>
                </section>
              )}
            </>
          )}
        </div>
      )}
    </div>
  )
}

/** What the search understood, as chips: type, provider, period, status, words, reference. */
function Understood({ interpretation: it }: { interpretation: Interpretation }) {
  const { t } = useTranslation()
  const chips: string[] = [
    ...it.typeNames,
    ...it.providerNames,
    ...(it.from && it.to ? [it.from === it.to ? formatMonth(it.from) : `${formatMonth(it.from)} – ${formatMonth(it.to)}`] : []),
    ...(it.statuses.length ? [statusLabel(it.statuses)] : []),
  ]
  if (chips.length === 0 && it.terms.length === 0 && !it.referenceMatch) return null
  return (
    <div className="flex flex-wrap items-center gap-2 text-sm text-body" data-testid="search-understood">
      <span className="text-muted">{t('search.showing')}</span>
      {chips.map((c) => (
        <span key={c} className="rounded-full bg-featured px-3 py-1 text-xs font-semibold text-on-featured">
          {c}
        </span>
      ))}
      {it.referenceMatch && (
        <span className="inline-flex items-center gap-1 rounded-full bg-[var(--st-paid-bg)] px-3 py-1 text-xs font-semibold text-[var(--st-paid-fg)]">
          <KeyRound className="size-3.5" aria-hidden /> {t('search.referenceMatch')}
        </span>
      )}
      {it.terms.length > 0 && <span>{t('search.matching', { words: it.terms.map((w) => `“${w}”`).join(' ') })}</span>}
    </div>
  )
}

function statusLabel(statuses: string[]): string {
  const t = tr
  if (statuses.includes('unpaid') && statuses.includes('overdue')) return t('search.notPaid')
  return statuses.map((s) => t(`status.${s as 'paid'}`)).join(', ')
}

function DocumentHit({ doc, terms }: { doc: Schemas['SearchDocument']; terms: string[] }) {
  const { t } = useTranslation()
  const body = (
    <span className="flex min-w-0 flex-1 flex-col gap-0.5">
      <span className="truncate font-medium text-ink">{doc.fileName}</span>
      {doc.snippet && (
        <span className="line-clamp-2 text-sm text-body">
          <Highlight text={doc.snippet} terms={terms} />
        </span>
      )}
      <span className="text-xs text-muted">
        {t(`documents.kind.${doc.kind}`)} · {formatDate(doc.createdAt.slice(0, 10))}
      </span>
    </span>
  )
  const cls = 'flex items-start gap-3 px-4 py-3 sm:px-5'
  return doc.billId ? (
    <Link to="/bills/$billId" params={{ billId: doc.billId }} className={cn(cls, 'transition hover:bg-card-muted/60')}>
      <FileText className="mt-0.5 size-5 shrink-0 text-muted" aria-hidden />
      {body}
    </Link>
  ) : (
    <div className={cls}>
      <FileText className="mt-0.5 size-5 shrink-0 text-muted" aria-hidden />
      {body}
    </div>
  )
}

/** Marks the searched words in a snippet. */
export function Highlight({ text, terms }: { text: string; terms: string[] }) {
  const words = terms.filter(Boolean)
  if (words.length === 0) return <>{text}</>
  const re = new RegExp(`(${words.map((w) => w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')})`, 'gi')
  return (
    <>
      {text.split(re).map((part, i) =>
        i % 2 === 1 ? (
          <mark key={i} className="rounded-sm bg-accent px-0.5 text-on-accent">
            {part}
          </mark>
        ) : (
          <Fragment key={i}>{part}</Fragment>
        ),
      )}
    </>
  )
}
