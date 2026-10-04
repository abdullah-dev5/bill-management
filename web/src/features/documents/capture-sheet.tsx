import { Camera, FileUp, RotateCw } from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { DocumentKind } from '@/api/client'
import { useInvalidateDocuments } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { Chip, ErrorNotice } from '@/components/ui/misc'
import { Sheet } from '@/components/ui/sheet'
import {
  decodeImage, FULL_CROP, isImage, isPdf, MAX_UPLOAD_BYTES, renderJpeg, withExtension,
  type CropRect, type Rotation,
} from '@/lib/image'
import { CropEditor } from './crop-editor'
import { uploadDocument, type PreparedUpload, type UploadResult } from './upload'

type Step =
  | { name: 'pick' }
  | { name: 'decoding' }
  | { name: 'edit'; bitmap: ImageBitmap; fileName: string; source: 'camera' | 'upload' }
  | { name: 'uploading'; progress: number }

const KINDS: DocumentKind[] = ['bill', 'receipt', 'other']

/** Take a photo or pick a file, crop/rotate it, and upload it (optionally to a bill). */
export function CaptureSheet({ open, onOpenChange, billId, defaultKind = 'bill', title, onUploaded }: {
  open: boolean
  onOpenChange: (open: boolean) => void
  billId?: string
  defaultKind?: DocumentKind
  title?: string
  onUploaded?: (result: UploadResult) => void | Promise<void>
}) {
  const { t } = useTranslation()
  const invalidate = useInvalidateDocuments()
  const [step, setStep] = useState<Step>({ name: 'pick' })
  const [kind, setKind] = useState<DocumentKind>(defaultKind)
  const [rotation, setRotation] = useState<Rotation>(0)
  const [crop, setCrop] = useState<CropRect>(FULL_CROP)
  const [error, setError] = useState<Error>()
  const cameraInput = useRef<HTMLInputElement>(null)
  const fileInput = useRef<HTMLInputElement>(null)

  const reset = () => {
    if (step.name === 'edit') step.bitmap.close()
    setStep({ name: 'pick' })
    setRotation(0)
    setCrop(FULL_CROP)
    setError(undefined)
  }
  const close = (o: boolean) => {
    if (step.name === 'uploading') return
    if (!o) {
      reset()
      setKind(defaultKind)
    }
    onOpenChange(o)
  }

  const send = async (file: PreparedUpload) => {
    setStep({ name: 'uploading', progress: 0 })
    try {
      const result = await uploadDocument(file, {
        kind,
        billId,
        onProgress: (progress) => setStep({ name: 'uploading', progress }),
      })
      await invalidate()
      if (result.duplicate) {
        toast.info(result.document.billId && result.document.billId !== billId ? t('documents.duplicateElsewhere') : t('documents.duplicate'))
      } else {
        toast.success(t('documents.uploaded'))
      }
      await onUploaded?.(result)
      reset()
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)))
      setStep({ name: 'pick' })
    }
  }

  const onPicked = async (file: File | undefined, source: 'camera' | 'upload') => {
    if (!file) return
    setError(undefined)
    if (isPdf(file)) {
      if (file.size > MAX_UPLOAD_BYTES) return setError(new Error(t('documents.tooLarge')))
      return send({ blob: file, contentType: 'application/pdf', fileName: file.name, source })
    }
    if (!isImage(file)) return setError(new Error(t('documents.unsupported')))
    setStep({ name: 'decoding' })
    try {
      const bitmap = await decodeImage(file)
      setStep({ name: 'edit', bitmap, fileName: file.name, source })
    } catch {
      setStep({ name: 'pick' })
      setError(new Error(t('documents.unreadable')))
    }
  }

  const confirmPhoto = async () => {
    if (step.name !== 'edit') return
    const { bitmap, fileName, source } = step
    try {
      const blob = await renderJpeg(bitmap, rotation, crop)
      bitmap.close()
      await send({ blob, contentType: 'image/jpeg', fileName: withExtension(fileName, '.jpg'), source })
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)))
    }
  }

  return (
    <Sheet
      open={open}
      onOpenChange={close}
      title={title ?? t('documents.addTitle')}
      description={step.name === 'edit' ? t('documents.cropHint') : t('documents.addHint')}
      footer={
        step.name === 'edit' ? (
          <>
            <Button variant="secondary" onClick={reset}>
              {t('documents.retake')}
            </Button>
            <Button onClick={() => void confirmPhoto()}>{t('documents.usePhoto')}</Button>
          </>
        ) : undefined
      }
    >
      <div className="flex flex-col gap-4">
        <div className="flex flex-wrap gap-2" role="group" aria-label={t('documents.kindLabel')}>
          {KINDS.map((k) => (
            <Chip key={k} selected={kind === k} onClick={() => setKind(k)} disabled={step.name === 'uploading'}>
              {t(`documents.kind.${k}`)}
            </Chip>
          ))}
        </div>

        {error && <ErrorNotice error={error} />}

        {step.name === 'pick' && (
          <div className="grid gap-3 sm:grid-cols-2">
            <Button variant="primary" icon={<Camera className="size-5" />} onClick={() => cameraInput.current?.click()}>
              {t('documents.takePhoto')}
            </Button>
            <Button variant="secondary" icon={<FileUp className="size-5" />} onClick={() => fileInput.current?.click()}>
              {t('documents.chooseFile')}
            </Button>
            <p className="text-sm text-muted sm:col-span-2">{t('documents.limits')}</p>
          </div>
        )}

        {step.name === 'decoding' && <p className="py-8 text-center text-sm text-muted">{t('documents.preparing')}</p>}

        {step.name === 'edit' && (
          <>
            <CropEditor bitmap={step.bitmap} rotation={rotation} crop={crop} onCropChange={setCrop} />
            <div className="flex justify-center gap-2">
              <Button
                variant="ghost"
                size="sm"
                icon={<RotateCw className="size-4" />}
                onClick={() => {
                  setRotation((r) => ((r + 90) % 360) as Rotation)
                  setCrop(FULL_CROP)
                }}
              >
                {t('documents.rotate')}
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setCrop(FULL_CROP)} disabled={crop === FULL_CROP}>
                {t('documents.resetCrop')}
              </Button>
            </div>
          </>
        )}

        {step.name === 'uploading' && (
          <div className="flex flex-col gap-2 py-6" role="status">
            <p className="text-sm font-medium text-ink">{t('documents.uploading')}</p>
            <div className="h-2 overflow-hidden rounded-full bg-card-muted">
              <div className="h-full rounded-full bg-featured transition-[width]" style={{ width: `${Math.round(step.progress * 100)}%` }} />
            </div>
          </div>
        )}
      </div>

      <input
        ref={cameraInput}
        type="file"
        accept="image/*"
        capture="environment"
        className="hidden"
        data-testid="camera-input"
        onChange={(e) => {
          void onPicked(e.target.files?.[0], 'camera')
          e.target.value = ''
        }}
      />
      <input
        ref={fileInput}
        type="file"
        accept="image/*,.heic,.heif,application/pdf"
        className="hidden"
        data-testid="file-input"
        onChange={(e) => {
          void onPicked(e.target.files?.[0], 'upload')
          e.target.value = ''
        }}
      />
    </Sheet>
  )
}
