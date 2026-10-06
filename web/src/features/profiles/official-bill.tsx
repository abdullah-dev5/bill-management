import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { AlertTriangle, ExternalLink, Landmark, RefreshCw, Upload } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api, unwrap, type Profile } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Toggle } from '@/components/ui/field'
import { Card } from '@/components/ui/misc'
import { CaptureSheet } from '@/features/documents/capture-sheet'
import { relativeTime } from '@/features/home/activity'

/**
 * The profile's official source (M6). Assisted: open the provider's own lookup page, solve its
 * CAPTCHA, save the bill and upload it as an official copy. Automatic: a connector checks the
 * official site; the person can switch it on and fetch now.
 */
export function OfficialBillCard({ profile }: { profile: Profile }) {
  const { t } = useTranslation()
  const r = profile.retrieval
  if (!r || r.mode === 'none' || profile.archived) return null
  return (
    <Card className="flex flex-col gap-3" data-testid="official-bill">
      <p className="flex items-center gap-2 font-semibold text-ink">
        <Landmark className="size-5 text-muted" aria-hidden /> {t('official.title')}
      </p>
      {r.mode === 'assisted' ? <Assisted profile={profile} /> : <Automatic profile={profile} />}
    </Card>
  )
}

function Assisted({ profile }: { profile: Profile }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [capturing, setCapturing] = useState(false)
  const url = profile.retrieval?.lookupUrl ?? ''

  const open = async () => {
    // The reference number is ready to paste into the official page.
    if (profile.referenceNumber) {
      try {
        await navigator.clipboard.writeText(profile.referenceNumber.replace(/\s+/g, ''))
        toast.success(t('official.copied'))
      } catch {
        // clipboard blocked: the number is shown on the card
      }
    }
    window.open(url, '_blank', 'noopener,noreferrer')
  }

  return (
    <>
      <p className="text-sm text-body">{t('official.assistedHint')}</p>
      <ol className="flex list-decimal flex-col gap-1 pl-5 text-sm text-body">
        <li>{profile.referenceNumber ? t('official.step1', { ref: profile.referenceNumber }) : t('official.step1NoRef')}</li>
        <li>{t('official.step2')}</li>
        <li>{t('official.step3')}</li>
      </ol>
      <div className="flex flex-wrap gap-2">
        <Button variant="secondary" className="h-11" onClick={() => void open()}>
          <ExternalLink className="size-4" aria-hidden /> {t('official.open')}
        </Button>
        <Button className="h-11" onClick={() => setCapturing(true)}>
          <Upload className="size-4" aria-hidden /> {t('official.upload')}
        </Button>
      </div>
      <p className="text-xs text-muted">{t('official.why')}</p>
      <CaptureSheet
        open={capturing}
        onOpenChange={setCapturing}
        title={t('official.uploadTitle')}
        onUploaded={async ({ document }) => {
          await navigate({ to: '/scan/$documentId', params: { documentId: document.id }, search: { official: true, profileId: profile.id } })
        }}
      />
    </>
  )
}

function Automatic({ profile }: { profile: Profile }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const navigate = useNavigate()
  const r = profile.retrieval!
  const refresh = () =>
    Promise.all(
      // A fetched bill can change any bill already loaded (merged copies), Home and the bell.
      ['profile', 'profiles', 'bills', 'bill', 'home', 'notifications', 'activity'].map((k) => qc.invalidateQueries({ queryKey: [k] })),
    )

  const toggle = useMutation({
    mutationFn: (enabled: boolean) => unwrap(api.PUT('/profiles/{id}/retrieval', { params: { path: { id: profile.id } }, body: { enabled } })),
    onSuccess: () => void refresh(),
    onError: (e) => toast.error(e.message),
  })
  const fetchNow = useMutation({
    mutationFn: () => unwrap(api.POST('/profiles/{id}/retrieve', { params: { path: { id: profile.id } } })),
    onSuccess: async (res) => {
      await refresh()
      const ok = res.outcome === 'created' || res.outcome === 'merged' || res.outcome === 'unchanged' || res.outcome === 'not_found'
      if (!ok) toast.error(res.message)
      else if (res.billId && (res.outcome === 'created' || res.outcome === 'merged'))
        toast.success(res.message, { action: { label: t('official.view'), onClick: () => void navigate({ to: '/bills/$billId', params: { billId: res.billId! } }) } })
      else toast.info(res.message)
    },
    onError: (e) => toast.error(e.message),
  })

  return (
    <>
      <Toggle
        label={t('official.auto')}
        hint={t('official.autoHint')}
        checked={r.enabled}
        disabled={toggle.isPending}
        onChange={(v) => toggle.mutate(v)}
      />
      {r.connectorPaused && (
        <p role="alert" className="flex items-start gap-2 rounded-field bg-[var(--st-soon-bg)] p-3 text-sm text-[var(--st-soon-fg)]">
          <AlertTriangle className="mt-0.5 size-4 shrink-0" aria-hidden /> {t('official.paused')}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-3">
        <Button variant="secondary" className="h-10" loading={fetchNow.isPending} onClick={() => fetchNow.mutate()} disabled={r.connectorPaused ?? false}>
          <RefreshCw className="size-4" aria-hidden /> {t('official.fetchNow')}
        </Button>
        {r.lastAt && (
          <span className="text-sm text-muted" data-testid="last-retrieval">
            {t('official.last', { when: relativeTime(r.lastAt) })}
            {r.lastMessage && ` · ${r.lastMessage}`}
          </span>
        )}
      </div>
    </>
  )
}
