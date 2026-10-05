import { Download, Send, Smartphone } from 'lucide-react'
import { useEffect, useState, useSyncExternalStore } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api, unwrap, type Schemas } from '@/api/client'
import { useNotificationSettings, useSaveNotificationSettings } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { Field, Select, Toggle } from '@/components/ui/field'
import { Card, SectionTitle, Skeleton } from '@/components/ui/misc'
import { currentSubscription, installState, onInstallChange, promptInstall, pushSupported, subscribeDevice } from '@/lib/pwa'

type Settings = Schemas['NotificationSettings']

const LEAD_DAYS = [0, 1, 2, 3, 5, 7, 10, 14]
const HOURS = Array.from({ length: 24 }, (_, h) => h)
const hourLabel = (h: number) => `${String(h).padStart(2, '0')}:00`

/** Settings → Reminders (FR-REM-02/03). Each change saves straight away. */
export function RemindersSection() {
  const { t } = useTranslation()
  const { data } = useNotificationSettings()
  const save = useSaveNotificationSettings()

  if (!data) return <Skeleton className="h-64 rounded-card" />
  const update = (patch: Partial<Settings>) =>
    save.mutateAsync({ ...data, ...patch }).then(
      () => toast.success(t('reminders.saved')),
      (e: Error) => toast.error(e.message),
    )

  return (
    <Card id="reminders" className="flex scroll-mt-6 flex-col gap-4">
      <div>
        <SectionTitle>{t('reminders.title')}</SectionTitle>
        <p className="-mt-2 text-sm text-muted">{t('reminders.hint')}</p>
      </div>
      <Toggle label={t('reminders.dueSoon')} hint={t('reminders.dueSoonHint')} checked={data.dueSoon} onChange={(v) => void update({ dueSoon: v })} />
      {data.dueSoon && (
        <Field label={t('reminders.leadDays')} className="sm:max-w-xs">
          {(p) => (
            <Select
              {...p}
              value={String(data.leadDays)}
              onChange={(v) => void update({ leadDays: Number(v) })}
              options={LEAD_DAYS.map((d) => ({ value: String(d), label: t('reminders.leadDaysOption', { count: d }) }))}
            />
          )}
        </Field>
      )}
      <Toggle label={t('reminders.overdue')} hint={t('reminders.overdueHint')} checked={data.overdue} onChange={(v) => void update({ overdue: v })} />
      <Toggle label={t('reminders.missing')} hint={t('reminders.missingHint')} checked={data.missing} onChange={(v) => void update({ missing: v })} />
      <PushControls settings={data} update={update} />
      <InstallRow />
    </Card>
  )
}

function PushControls({ settings, update }: { settings: Settings; update: (p: Partial<Settings>) => Promise<unknown> }) {
  const { t } = useTranslation()
  const [busy, setBusy] = useState(false)
  const [subscribed, setSubscribed] = useState<boolean>()
  const supported = pushSupported()

  useEffect(() => {
    let live = true
    void currentSubscription().then((s) => live && setSubscribed(Boolean(s)))
    return () => {
      live = false
    }
  }, [])

  const enable = async () => {
    setBusy(true)
    try {
      const { publicKey } = await unwrap(api.GET('/push/key'))
      const sub = await subscribeDevice(publicKey)
      await unwrap(api.POST('/push/subscriptions', { body: { endpoint: sub.endpoint ?? '', keys: { p256dh: sub.keys?.p256dh ?? '', auth: sub.keys?.auth ?? '' } } }))
      setSubscribed(true)
      await update({ push: true })
    } catch (e) {
      const code = e instanceof Error ? e.message : ''
      toast.error(code === 'denied' ? t('reminders.pushDenied') : code === 'no-service-worker' ? t('reminders.pushNoWorker') : t('reminders.pushFailed'))
    } finally {
      setBusy(false)
    }
  }

  const disable = async () => {
    setBusy(true)
    try {
      const sub = await currentSubscription()
      if (sub) {
        await unwrap(api.POST('/push/unsubscribe', { body: { endpoint: sub.endpoint } }))
        await sub.unsubscribe()
      }
      setSubscribed(false)
      await update({ push: false })
    } finally {
      setBusy(false)
    }
  }

  const test = async () => {
    const res = await unwrap(api.POST('/push/test'))
    if (res.delivered > 0) toast.success(t('reminders.pushTestSent', { count: res.delivered }))
    else toast.error(t('reminders.pushTestNone'))
  }

  const on = settings.push && subscribed !== false
  return (
    <div className="flex flex-col gap-3 border-t border-line pt-4">
      <Toggle
        label={t('reminders.push')}
        hint={supported ? t('reminders.pushHint') : t('reminders.pushUnsupported')}
        checked={on}
        disabled={!supported || busy}
        onChange={(v) => void (v ? enable() : disable())}
      />
      {on && (
        <>
          <div className="grid grid-cols-2 gap-3 sm:max-w-md">
            <Field label={t('reminders.quietFrom')}>
              {(p) => (
                <Select
                  {...p}
                  value={settings.quietStart == null ? '' : String(settings.quietStart)}
                  onChange={(v) =>
                    void update(v === '' ? { quietStart: null, quietEnd: null } : { quietStart: Number(v), quietEnd: settings.quietEnd ?? 7 })
                  }
                  options={[{ value: '', label: t('reminders.quietOff') }, ...HOURS.map((h) => ({ value: String(h), label: hourLabel(h) }))]}
                />
              )}
            </Field>
            <Field label={t('reminders.quietTo')}>
              {(p) => (
                <Select
                  {...p}
                  disabled={settings.quietStart == null}
                  value={settings.quietEnd == null ? '' : String(settings.quietEnd)}
                  onChange={(v) => void update({ quietEnd: Number(v) })}
                  options={HOURS.map((h) => ({ value: String(h), label: hourLabel(h) }))}
                />
              )}
            </Field>
          </div>
          <p className="-mt-1 text-sm text-muted">{t('reminders.quietHint')}</p>
          <Button variant="secondary" className="h-10 self-start" onClick={() => void test()}>
            <Send className="size-4" aria-hidden /> {t('reminders.pushTest')}
          </Button>
        </>
      )}
    </div>
  )
}

function InstallRow() {
  const { t } = useTranslation()
  const state = useSyncExternalStore(onInstallChange, installState, () => 'unavailable' as const)
  if (state === 'unavailable') return null
  return (
    <div className="flex flex-wrap items-center gap-3 border-t border-line pt-4">
      <Smartphone className="size-5 shrink-0 text-muted" aria-hidden />
      <p className="min-w-0 flex-1 text-sm text-body">
        {state === 'installed' ? t('install.installed') : state === 'ios' ? t('install.ios') : t('install.hint')}
      </p>
      {state === 'available' && (
        <Button className="h-10" onClick={() => void promptInstall()}>
          <Download className="size-4" aria-hidden /> {t('install.button')}
        </Button>
      )}
    </div>
  )
}
