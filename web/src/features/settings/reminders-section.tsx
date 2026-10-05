import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { Schemas } from '@/api/client'
import { useNotificationSettings, useSaveNotificationSettings } from '@/api/hooks'
import { Field, Select, Toggle } from '@/components/ui/field'
import { Card, SectionTitle, Skeleton } from '@/components/ui/misc'

type Settings = Schemas['NotificationSettings']

const LEAD_DAYS = [0, 1, 2, 3, 5, 7, 10, 14]

/** Settings → Reminders (FR-REM-02). Each change saves straight away. */
export function RemindersSection() {
  const { t } = useTranslation()
  const { data } = useNotificationSettings()
  const save = useSaveNotificationSettings()

  if (!data) return <Skeleton className="h-64 rounded-card" />
  const update = (patch: Partial<Settings>) =>
    save.mutate({ ...data, ...patch }, { onSuccess: () => toast.success(t('reminders.saved')), onError: (e) => toast.error(e.message) })

  return (
    <Card id="reminders" className="flex scroll-mt-6 flex-col gap-4">
      <div>
        <SectionTitle>{t('reminders.title')}</SectionTitle>
        <p className="-mt-2 text-sm text-muted">{t('reminders.hint')}</p>
      </div>
      <Toggle label={t('reminders.dueSoon')} hint={t('reminders.dueSoonHint')} checked={data.dueSoon} onChange={(v) => update({ dueSoon: v })} />
      {data.dueSoon && (
        <Field label={t('reminders.leadDays')} className="sm:max-w-xs">
          {(p) => (
            <Select
              {...p}
              value={String(data.leadDays)}
              onChange={(v) => update({ leadDays: Number(v) })}
              options={LEAD_DAYS.map((d) => ({ value: String(d), label: t('reminders.leadDaysOption', { count: d }) }))}
            />
          )}
        </Field>
      )}
      <Toggle label={t('reminders.overdue')} hint={t('reminders.overdueHint')} checked={data.overdue} onChange={(v) => update({ overdue: v })} />
      <Toggle label={t('reminders.missing')} hint={t('reminders.missingHint')} checked={data.missing} onChange={(v) => update({ missing: v })} />
    </Card>
  )
}
