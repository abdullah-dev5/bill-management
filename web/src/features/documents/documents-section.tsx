import { Download, ExternalLink, FileText, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { Doc } from '@/api/client'
import { useDeleteDocument, useDocument } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { Card, SectionTitle, Skeleton } from '@/components/ui/misc'
import { Sheet } from '@/components/ui/sheet'
import { formatBytes } from '@/lib/image'
import { CaptureSheet } from './capture-sheet'
import { ExtractionPanel } from './extraction-panel'

const isPdfDoc = (d: Doc) => d.contentType === 'application/pdf'

/** The documents attached to a bill: thumbnails, viewer, add/replace/delete (FR-DOC-01/02). */
export function DocumentsSection({ billId, documents, readOnly, onLinksExpired }: {
  billId: string
  documents: Doc[]
  readOnly?: boolean
  onLinksExpired: () => void
}) {
  const { t } = useTranslation()
  const [adding, setAdding] = useState(false)
  const [openId, setOpenId] = useState<string>()

  return (
    <Card>
      <SectionTitle
        action={
          !readOnly ? (
            <Button size="sm" variant="secondary" icon={<Plus className="size-4" />} onClick={() => setAdding(true)}>
              {t('documents.add')}
            </Button>
          ) : undefined
        }
      >
        {t('documents.title')}
      </SectionTitle>

      {documents.length === 0 ? (
        <p className="text-sm text-muted">{t('documents.none')}</p>
      ) : (
        <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3">
          {documents.map((d) => (
            <li key={d.id}>
              <button
                type="button"
                onClick={() => setOpenId(d.id)}
                className="group flex w-full flex-col overflow-hidden rounded-field border border-line bg-card text-left transition hover:border-focus"
              >
                <span className="flex aspect-[4/3] items-center justify-center overflow-hidden bg-card-muted">
                  {isPdfDoc(d) || !d.viewUrl ? (
                    <FileText className="size-10 text-muted" aria-hidden />
                  ) : (
                    <img src={d.viewUrl} alt="" loading="lazy" className="size-full object-cover" onError={onLinksExpired} />
                  )}
                </span>
                <span className="flex flex-col px-3 py-2">
                  <span className="truncate text-sm font-medium text-ink">{d.fileName}</span>
                  <span className="text-xs text-muted">
                    {t(`documents.kind.${d.kind}`)} · {formatBytes(d.sizeBytes)}
                  </span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}

      <CaptureSheet open={adding} onOpenChange={setAdding} billId={billId} />
      <DocumentViewer id={openId} billId={billId} readOnly={readOnly} onClose={() => setOpenId(undefined)} />
    </Card>
  )
}

function DocumentViewer({ id, billId, readOnly, onClose }: { id?: string; billId: string; readOnly?: boolean; onClose: () => void }) {
  const { t } = useTranslation()
  const doc = useDocument(id)
  const remove = useDeleteDocument()
  const [confirming, setConfirming] = useState(false)
  const [replacing, setReplacing] = useState(false)
  const d = doc.data

  const del = async () => {
    if (!id) return
    try {
      await remove.mutateAsync(id)
      toast.success(t('documents.deleted'))
      setConfirming(false)
      onClose()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('common.errorTitle'))
    }
  }

  return (
    <>
      <Sheet
        open={id != null && !replacing}
        onOpenChange={(o) => !o && onClose()}
        title={d?.fileName ?? t('documents.title')}
        description={d ? `${t(`documents.kind.${d.kind}`)} · ${formatBytes(d.sizeBytes)}` : undefined}
        footer={
          d &&
          !readOnly && (
            <>
              <Button variant="danger" icon={<Trash2 className="size-4" />} onClick={() => setConfirming(true)}>
                {t('common.delete')}
              </Button>
              <Button variant="secondary" icon={<RefreshCw className="size-4" />} onClick={() => setReplacing(true)}>
                {t('documents.replace')}
              </Button>
            </>
          )
        }
      >
        {!d ? (
          <Skeleton className="h-72 w-full rounded-field" />
        ) : (
          <div className="flex flex-col gap-4">
            {isPdfDoc(d) ? (
              <div className="flex flex-col items-center gap-3 rounded-field bg-card-muted px-4 py-10 text-center">
                <FileText className="size-12 text-muted" aria-hidden />
                <p className="text-sm text-muted">{t('documents.pdfHint')}</p>
              </div>
            ) : (
              d.viewUrl && <img src={d.viewUrl} alt={d.fileName} className="max-h-[60dvh] w-full rounded-field bg-card-muted object-contain" />
            )}
            {d.kind === 'bill' && <ExtractionPanel documentId={d.id} />}
            <div className="flex flex-wrap gap-2">
              {d.viewUrl && isPdfDoc(d) && (
                <a href={d.viewUrl} target="_blank" rel="noreferrer" className="inline-flex h-10 items-center gap-2 rounded-full bg-featured px-4 text-sm font-semibold text-on-featured">
                  <ExternalLink className="size-4" aria-hidden /> {t('documents.openPdf')}
                </a>
              )}
              {d.downloadUrl && (
                <a href={d.downloadUrl} className="inline-flex h-10 items-center gap-2 rounded-full bg-card-muted px-4 text-sm font-semibold text-ink">
                  <Download className="size-4" aria-hidden /> {t('documents.download')}
                </a>
              )}
            </div>
          </div>
        )}
      </Sheet>

      <Sheet
        open={confirming}
        onOpenChange={setConfirming}
        title={t('documents.deleteConfirmTitle')}
        description={t('documents.deleteConfirmBody')}
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirming(false)}>
              {t('common.cancel')}
            </Button>
            <Button variant="danger" loading={remove.isPending} onClick={() => void del()}>
              {t('common.delete')}
            </Button>
          </>
        }
      >
        {null}
      </Sheet>

      {d && (
        <CaptureSheet
          open={replacing}
          onOpenChange={setReplacing}
          billId={billId}
          defaultKind={d.kind}
          title={t('documents.replaceTitle')}
          onUploaded={async (result) => {
            // Originals are immutable: replacing uploads a new file, then removes the old one.
            if (result.document.id !== d.id) await remove.mutateAsync(d.id)
            onClose()
          }}
        />
      )}
    </>
  )
}
