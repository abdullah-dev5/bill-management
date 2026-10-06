import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import { CheckCircle2, ExternalLink, FileText, Link2, Loader2 } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ApiError, type BillCreate, type Extraction, type Profile, type Provider } from '@/api/client'
import {
  useCreateProfile,
  useDocument,
  useExtraction,
  useProfiles,
  useProviders,
  useSaveScannedBill,
  useStartExtraction,
  useTypes,
} from '@/api/hooks'
import { PageHeader } from '@/components/layout/app-shell'
import { Button } from '@/components/ui/button'
import { Field, Input, Select, Toggle } from '@/components/ui/field'
import { providerOptions } from '@/features/profiles/options'
import { Card, ErrorNotice, Skeleton } from '@/components/ui/misc'
import { Sheet } from '@/components/ui/sheet'
import { BackLink } from '@/features/bills/bill-pages'
import { BillForm, type BillFormValues, type BillReview } from '@/features/bills/bill-form'
import { DeviceReadButton } from '@/features/documents/device-read'
import { formatMonthShort } from '@/lib/dates'
import { formatMoney } from '@/lib/money'

const NEW = 'new'

export const normalizeRef = (s: string | null | undefined) => (s ?? '').toUpperCase().replace(/[^A-Z0-9]/g, '')

/** Maps an extraction to bill-form values and per-field confidence. */
export function reviewFromExtraction(e: Extraction | null | undefined): Pick<BillReview, 'values' | 'confidence'> {
  const values: Partial<BillFormValues> = {}
  const confidence: BillReview['confidence'] = {}
  if (!e || e.status !== 'succeeded') return { values, confidence }
  const f = e.fields
  const take = (field: string, key: keyof BillFormValues, convert: (v: string) => string | number = (v) => v) => {
    const got = f[field]
    if (!got) return
    ;(values as Record<string, unknown>)[key] = convert(got.value)
    confidence[key] = got.confidence
  }
  const num = (v: string) => Number(v)
  take('billingMonth', 'periodKey')
  take('issueDate', 'issueDate')
  take('dueDate', 'dueDate')
  take('currentChargesMinor', 'currentChargesMinor', num)
  take('arrearsMinor', 'arrearsMinor', num)
  take('adjustmentsMinor', 'adjustmentsMinor', num)
  take('payableAfterDueMinor', 'payableAfterDueMinor', num)
  take('billNumber', 'billNumber')
  take('previousReading', 'previousReading')
  take('currentReading', 'currentReading')
  take('units', 'units')
  take('multiplyingFactor', 'multiplyingFactor')
  if (values.issueDate) values.expenseDate = values.issueDate

  // Keep the printed payable only when it differs from charges + arrears + adjustments.
  const payable = f.payableMinor ? Number(f.payableMinor.value) : null
  const sum = (values.currentChargesMinor ?? 0) + (values.arrearsMinor ?? 0) + (values.adjustmentsMinor ?? 0)
  if (payable != null && payable !== sum) {
    values.payableMinor = payable
    confidence.payableMinor = f.payableMinor!.confidence
  }
  if (e.lateFees.length >= 2 && e.lateFees[0]!.until) {
    values.lateStepUntil = e.lateFees[0]!.until
    values.payableFinalMinor = e.lateFees[e.lateFees.length - 1]!.amountMinor
  }
  return { values, confidence }
}

/** Check what was read from a scanned/uploaded bill, link it to a profile and save it (FR-EXTR-08/09). */
export function ScanPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { documentId } = useParams({ from: '/app/scan/$documentId' })
  const { official, profileId: fromProfile } = useSearch({ from: '/app/scan/$documentId' })
  const doc = useDocument(documentId)
  const extraction = useExtraction(documentId)
  const start = useStartExtraction()
  const profiles = useProfiles()
  const providers = useProviders()
  const types = useTypes()
  const createProfile = useCreateProfile()
  const save = useSaveScannedBill(documentId)
  const [manual, setManual] = useState(false)
  const [choice, setChoice] = useState<string>()
  const [saveHistory, setSaveHistory] = useState(true)
  const [duplicate, setDuplicate] = useState<{
    code: string
    existingBillId: string
    body: BillCreate
    message: string
  }>()
  const started = useRef(false)

  const e = extraction.data
  const reading = !manual && (start.isPending || e?.status === 'queued' || e?.status === 'running' || e === null)

  // Start reading automatically the first time the page opens.
  useEffect(() => {
    if (started.current || extraction.isLoading || e !== null || !doc.data || doc.data.status !== 'ready') return
    started.current = true
    start.mutate(documentId, { onError: () => setManual(true) })
  }, [e, extraction.isLoading, doc.data, documentId, start])

  const active = useMemo(() => (profiles.data ?? []).filter((p) => !p.archived), [profiles.data])
  const extractedRef = normalizeRef(e?.fields.referenceNumber?.value)
  const matched = extractedRef ? active.find((p) => normalizeRef(p.referenceNumber) === extractedRef) : undefined
  const provider = (providers.data ?? []).find((p) => p.id === e?.providerId)
  // Uploaded from a profile's Official bill card? Prefer that profile, unless the bill's own
  // reference matches a different one.
  const fromProfileActive = fromProfile ? active.find((p) => p.id === fromProfile) : undefined
  const selected =
    choice ??
    (matched ? matched.id : fromProfileActive ? fromProfileActive.id : e?.status === 'succeeded' && (extractedRef || provider) ? NEW : active.find((p) => p.isDefault)?.id)

  const [draft, setDraft] = useState<{
    name: string
    typeId: string
    providerId: string
    reference: string
  }>()
  const typeForProvider = (p?: Provider) => p?.typeId ?? (types.data ?? []).find((ty) => ty.isSystem && ty.name === 'Electricity')?.id ?? ''
  const newProfile = draft ?? {
    name: provider ? `${provider.shortName} ${(types.data ?? []).find((ty) => ty.id === provider.typeId)?.name ?? ''}`.trim() : '',
    typeId: typeForProvider(provider),
    providerId: provider?.id ?? '',
    reference: e?.fields.referenceNumber?.value ?? '',
  }
  const draftType = (types.data ?? []).find((ty) => ty.id === newProfile.typeId)

  const profileForForm: Profile | undefined =
    selected === NEW
      ? ({
          id: '',
          name: newProfile.name,
          kind: draftType?.defaultKind ?? 'metered',
          typeId: newProfile.typeId,
          consumptionUnit: (providers.data ?? []).find((pv) => pv.id === newProfile.providerId)?.consumptionUnit ?? null,
        } as Profile)
      : active.find((p) => p.id === selected)

  // Once the user chooses to type the bill in, a late extraction result no longer replaces their input.
  const review = manual ? { values: {}, confidence: {} } : reviewFromExtraction(e)
  const history = !manual && e?.status === 'succeeded' ? e.history : []

  const submit = async (body: BillCreate, duplicateChoice?: 'attach' | 'keep_separate') => {
    let profileId = body.profileId
    if (selected === NEW) {
      const created = await createProfile.mutateAsync({
        name: newProfile.name.trim(),
        typeId: newProfile.typeId,
        providerId: newProfile.providerId || null,
        referenceNumber: newProfile.reference.trim() || null,
      })
      profileId = created.id
      setChoice(created.id)
    }
    const { paidNow: _ignored, ...bill } = body
    try {
      const saved = await save.mutateAsync({
        bill: { ...bill, profileId },
        extractionId: !manual && e?.status === 'succeeded' ? e.id : undefined,
        history:
          saveHistory && history.length > 0
            ? history.map((h) => ({
                month: h.month,
                units: h.units,
                billedMinor: h.billedMinor,
                paidMinor: h.paidMinor,
              }))
            : undefined,
        duplicate: duplicateChoice,
        officialCopy: official || undefined,
      })
      setDuplicate(undefined)
      const differs = saved.conflicts.filter((c) => c.status === 'open').length
      if (duplicateChoice === 'attach' && differs > 0) toast.warning(t('scan.attachedDiffers', { count: differs }))
      else toast.success(duplicateChoice === 'attach' ? t('scan.attached') : t('scan.saved'))
      await navigate({ to: '/bills/$billId', params: { billId: saved.id } })
    } catch (err) {
      if (err instanceof ApiError && (err.code === 'duplicate_bill' || err.code === 'probable_duplicate') && err.existingBillId) {
        setDuplicate({
          code: err.code,
          existingBillId: err.existingBillId,
          body: { ...body, profileId },
          message: err.message,
        })
        return
      }
      if (err instanceof ApiError && err.code === 'already_attached' && err.existingBillId) {
        toast.info(t('scan.alreadySaved'))
        await navigate({
          to: '/bills/$billId',
          params: { billId: err.existingBillId },
        })
        return
      }
      throw err
    }
  }

  if (doc.isError) return <ErrorNotice error={doc.error} />
  const d = doc.data

  return (
    <div className="mx-auto max-w-6xl">
      <PageHeader back={<BackLink to="/" label={t('nav.home')} />} title={t('scan.title')} />
      <div className="grid gap-5 lg:grid-cols-2">
        <div className="flex flex-col gap-4 lg:sticky lg:top-6 lg:self-start">
          <Card className="p-3">
            {!d ? (
              <Skeleton className="h-80 w-full rounded-field" />
            ) : d.contentType === 'application/pdf' ? (
              <div className="flex flex-col items-center gap-3 rounded-field bg-card-muted px-4 py-12 text-center">
                <FileText className="size-12 text-muted" aria-hidden />
                <p className="font-medium text-ink">{d.fileName}</p>
                {d.viewUrl && (
                  <a
                    href={d.viewUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-2 text-sm font-semibold text-ink underline"
                  >
                    <ExternalLink className="size-4" aria-hidden /> {t('documents.openPdf')}
                  </a>
                )}
              </div>
            ) : (
              d.viewUrl && <img src={d.viewUrl} alt={d.fileName} className="max-h-[70dvh] w-full rounded-field bg-card-muted object-contain" />
            )}
          </Card>
          {e?.status === 'succeeded' && e.warnings.length > 0 && (
            <Card className="bg-[var(--st-soon-bg)] text-sm text-[var(--st-soon-fg)]">
              <p className="mb-1 font-semibold">{t('scan.warnings')}</p>
              <ul className="list-disc pl-5">
                {e.warnings.map((w, i) => (
                  <li key={i}>{w.message}</li>
                ))}
              </ul>
            </Card>
          )}
        </div>

        <div className="flex flex-col gap-4">
          {reading ? (
            <Card className="flex flex-col items-center gap-3 py-12 text-center" role="status">
              <Loader2 className="size-8 animate-spin text-muted" aria-hidden />
              <p className="font-medium text-ink">{t('scan.reading')}</p>
              <p className="max-w-sm text-sm text-muted">{t('scan.readingHint')}</p>
              <Button variant="ghost" size="sm" onClick={() => setManual(true)}>
                {t('scan.enterMyself')}
              </Button>
            </Card>
          ) : (
            <>
              {e?.status === 'failed' && (
                <div role="alert" className="flex flex-col items-start gap-3 rounded-card bg-[var(--st-overdue-bg)] p-4 text-sm text-[var(--st-overdue-fg)]">
                  <p>{e.error}</p>
                  {doc.data && doc.data.contentType.startsWith('image/') && (
                    <div className="flex flex-col items-start gap-1">
                      <DeviceReadButton doc={doc.data} variant="primary" className="h-10" />
                      <p className="text-xs text-body">{t('deviceRead.hint')}</p>
                    </div>
                  )}
                </div>
              )}
              {official && (
                <p className="rounded-field bg-card-muted p-3 text-sm text-body" data-testid="official-copy">
                  {t('official.copyNote')}
                </p>
              )}
              {e?.status === 'succeeded' && (
                <p className="flex items-center gap-2 text-sm text-muted">
                  <CheckCircle2 className="size-4 text-[var(--st-paid-fg)]" aria-hidden /> {t('scan.readOk')}
                </p>
              )}

              <Card className="flex flex-col gap-4">
                <Field label={t('scan.profile')} hint={matched ? t('scan.matched') : undefined}>
                  {(p) => (
                    <Select
                      {...p}
                      value={selected ?? ''}
                      onChange={setChoice}
                      options={[
                        ...active.map((pr) => ({ value: pr.id, label: pr.name, hint: matched?.id === pr.id ? t('scan.matchedOption') : undefined })),
                        { value: NEW, label: t('scan.newProfile'), hint: t('scan.newProfileHint') },
                      ]}
                    />
                  )}
                </Field>
                {matched && selected === matched.id && (
                  <p className="flex items-center gap-2 text-sm font-medium text-[var(--st-paid-fg)]">
                    <Link2 className="size-4" aria-hidden /> {t('scan.matchedTo', { name: matched.name })}
                  </p>
                )}
                {selected === NEW && (
                  <div className="grid gap-4 rounded-field border border-line p-4 sm:grid-cols-2">
                    <Field label={t('profiles.name')} className="sm:col-span-2">
                      {(p) => <Input {...p} value={newProfile.name} onChange={(ev) => setDraft({ ...newProfile, name: ev.target.value })} />}
                    </Field>
                    <Field label={t('profiles.type')}>
                      {(p) => (
                        <Select
                          {...p}
                          value={newProfile.typeId}
                          onChange={(v) => setDraft({ ...newProfile, typeId: v, providerId: '' })}
                          options={(types.data ?? [])
                            .filter((ty) => !ty.archived && !ty.parentId)
                            .map((ty) => ({ value: ty.id, label: ty.name, group: t(`group.${ty.group}`) }))}
                        />
                      )}
                    </Field>
                    <Field label={t('profiles.provider')} optional>
                      {(p) => (
                        <Select
                          {...p}
                          value={newProfile.providerId}
                          onChange={(v) => setDraft({ ...newProfile, providerId: v })}
                          options={providerOptions(
                            (providers.data ?? []).filter((pv) => pv.typeId === newProfile.typeId),
                            t,
                          )}
                        />
                      )}
                    </Field>
                    <Field label={t('profiles.reference')} optional className="sm:col-span-2">
                      {(p) => (
                        <Input
                          {...p}
                          value={newProfile.reference}
                          onChange={(ev) =>
                            setDraft({
                              ...newProfile,
                              reference: ev.target.value,
                            })
                          }
                        />
                      )}
                    </Field>
                  </div>
                )}
              </Card>

              {history.length > 0 && (
                <Card className="flex flex-col gap-2">
                  <Toggle
                    label={t('scan.saveHistory', { count: history.length })}
                    hint={t('scan.saveHistoryHint', {
                      from: formatMonthShort(history[0]!.month),
                      to: formatMonthShort(history[history.length - 1]!.month),
                    })}
                    checked={saveHistory}
                    onChange={setSaveHistory}
                  />
                </Card>
              )}

              {profileForForm && (
                <Card>
                  <BillForm
                    key={`${selected}-${manual ? 'manual' : (e?.id ?? 'none')}-${profileForForm.kind}`}
                    profile={profileForForm}
                    review={{ ...review, submitLabel: t('scan.save') }}
                    onCancel={() => window.history.back()}
                    onSubmit={(body) => submit(body)}
                  />
                </Card>
              )}
            </>
          )}
        </div>
      </div>

      <Sheet
        open={duplicate != null}
        onOpenChange={(o) => !o && setDuplicate(undefined)}
        title={duplicate?.code === 'duplicate_bill' ? t('scan.duplicateTitle') : t('scan.probableTitle')}
        description={duplicate?.message}
        footer={
          duplicate && (
            <>
              {duplicate.code === 'probable_duplicate' ? (
                <Button variant="secondary" loading={save.isPending} onClick={() => void submit(duplicate.body, 'keep_separate')}>
                  {t('scan.keepSeparate')}
                </Button>
              ) : (
                <Button
                  variant="secondary"
                  onClick={() =>
                    void navigate({
                      to: '/bills/$billId',
                      params: { billId: duplicate.existingBillId },
                    })
                  }
                >
                  {t('scan.viewExisting')}
                </Button>
              )}
              <Button loading={save.isPending} onClick={() => void submit(duplicate.body, 'attach')}>
                {t('scan.attachToExisting')}
              </Button>
            </>
          )
        }
      >
        <p className="text-sm text-muted">
          {t('scan.duplicateHint', {
            amount: formatMoney(duplicate?.body.currentChargesMinor ?? 0),
          })}
        </p>
      </Sheet>
    </div>
  )
}
