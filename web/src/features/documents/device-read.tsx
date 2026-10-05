import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Smartphone } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api, unwrap, type Doc } from '@/api/client'
import { keys } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { readOnDevice } from '@/lib/device-ocr'

/**
 * "Read on this phone" (tier 3, docs/05): Tesseract.js recognises the photo on the device and
 * the text is parsed by the same rules as OCR. Offered for photos only.
 */
export function DeviceReadButton({ doc, variant = 'secondary', className }: { doc: Doc; variant?: 'primary' | 'secondary' | 'ghost'; className?: string }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [progress, setProgress] = useState<number>()

  const read = useMutation({
    mutationFn: async () => {
      if (!doc.viewUrl) throw new Error(t('deviceRead.noFile'))
      setProgress(0)
      const res = await fetch(doc.viewUrl)
      if (!res.ok) throw new Error(t('deviceRead.noFile'))
      const { text, confidence } = await readOnDevice(await res.blob(), setProgress)
      return unwrap(api.POST('/documents/{id}/extraction/device', { params: { path: { id: doc.id } }, body: { text: text || ' ', confidence } }))
    },
    onSuccess: (data) => {
      qc.setQueryData(keys.extraction(doc.id), data)
      if (data.status === 'failed') toast.error(data.error ?? t('deviceRead.nothing'))
      else toast.success(t('deviceRead.done'))
    },
    onError: (e) => toast.error(e instanceof Error && e.message !== 'Failed to fetch' ? e.message : t('deviceRead.failed')),
    onSettled: () => setProgress(undefined),
  })

  if (!doc.contentType.startsWith('image/')) return null
  return (
    <Button variant={variant} className={className} loading={read.isPending} onClick={() => read.mutate()} aria-describedby={`device-read-${doc.id}`}>
      {!read.isPending && <Smartphone className="size-4" aria-hidden />}
      {read.isPending ? t('deviceRead.reading', { pct: Math.round((progress ?? 0) * 100) }) : t('deviceRead.button')}
      <span id={`device-read-${doc.id}`} className="sr-only">
        {t('deviceRead.hint')}
      </span>
    </Button>
  )
}
