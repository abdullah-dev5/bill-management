import { useNavigate } from '@tanstack/react-router'
import { CloudOff, UploadCloud } from 'lucide-react'
import { useCallback, useEffect, useRef, useSyncExternalStore } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ApiError } from '@/api/client'
import { useInvalidateDocuments } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { listQueued, markAttempt, onQueueChange, queueSnapshot, removeQueued } from '@/lib/offline-queue'
import { uploadDocument } from './upload'

/** A failed request that a connection would fix (as opposed to the server refusing it). */
export function isNetworkError(err: unknown): boolean {
  return err instanceof TypeError || (err instanceof ApiError && (err.status === 0 || err.code === 'network'))
}

function subscribeOnline(cb: () => void) {
  window.addEventListener('online', cb)
  window.addEventListener('offline', cb)
  return () => {
    window.removeEventListener('online', cb)
    window.removeEventListener('offline', cb)
  }
}

export const useOnline = () => useSyncExternalStore(subscribeOnline, () => navigator.onLine, () => true)

const MAX_ATTEMPTS = 3

/**
 * Uploads queued photos when the device is online (on start, when the connection returns, and
 * every 20 s while anything waits). Mounted once, in the app shell.
 */
export function useUploadQueue() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const invalidate = useInvalidateDocuments()
  const online = useOnline()
  const items = useSyncExternalStore(onQueueChange, queueSnapshot, queueSnapshot)
  const running = useRef(false)
  // Latest helpers without re-creating process() (useInvalidateDocuments returns a new function each render).
  const deps = useRef({ invalidate, navigate, t })
  useEffect(() => {
    deps.current = { invalidate, navigate, t }
  })

  const process = useCallback(async () => {
    if (running.current || !navigator.onLine) return
    running.current = true
    try {
      for (const item of await listQueued()) {
        if (item.attempts >= MAX_ATTEMPTS) continue
        const { invalidate, navigate, t } = deps.current
        try {
          const result = await uploadDocument(item, { kind: item.kind, billId: item.billId })
          await removeQueued(item.id)
          await invalidate()
          const docId = result.document.id
          if (!item.billId && item.kind === 'bill' && !result.document.billId) {
            toast.success(t('queue.uploadedCheck'), { action: { label: t('queue.check'), onClick: () => void navigate({ to: '/scan/$documentId', params: { documentId: docId } }) } })
          } else {
            toast.success(t('queue.uploaded'))
          }
        } catch (err) {
          if (isNetworkError(err)) break // still offline; try again later
          await markAttempt(item, err instanceof Error ? err.message : String(err))
        }
      }
    } finally {
      running.current = false
    }
  }, [])

  // On start and whenever the connection comes back.
  useEffect(() => {
    if (online) void process()
  }, [online, process])

  const waiting = items.filter((i) => i.attempts < MAX_ATTEMPTS).length
  useEffect(() => {
    if (!online || waiting === 0) return
    const id = setInterval(() => void process(), 20_000)
    return () => clearInterval(id)
  }, [online, waiting, process])

  return { online, items, waiting, failed: items.length - waiting, retry: process }
}

/** Offline notice and the queued-photos line, at the top of every page. */
export function ConnectionBanner() {
  const { t } = useTranslation()
  const { online, waiting, failed, items, retry } = useUploadQueue()
  if (online && waiting === 0 && failed === 0) return null
  return (
    <div role="status" className="mb-4 flex flex-wrap items-center gap-3 rounded-card bg-card px-4 py-3 text-sm text-body ring-1 ring-line" data-testid="connection-banner">
      {online ? <UploadCloud className="size-5 shrink-0 text-muted" aria-hidden /> : <CloudOff className="size-5 shrink-0 text-muted" aria-hidden />}
      <span className="min-w-0 flex-1">
        {!online && <span className="font-semibold text-ink">{t('queue.offline')} </span>}
        {waiting > 0
          ? t(online ? 'queue.uploading' : 'queue.waiting', { count: waiting })
          : !online
            ? t('queue.offlineHint')
            : null}
        {failed > 0 && <span className="block text-[var(--danger)]">{t('queue.failed', { count: failed, reason: items.find((i) => i.lastError)?.lastError ?? '' })}</span>}
      </span>
      {online && waiting > 0 && (
        <Button variant="secondary" className="h-9 px-3" onClick={() => void retry()}>
          {t('queue.retry')}
        </Button>
      )}
    </div>
  )
}
