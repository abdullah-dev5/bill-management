/**
 * Offline capture queue (FR-EXTR-13): photos taken without a connection wait in IndexedDB on
 * this device and upload when it comes back. Nothing leaves the phone until then.
 */
import type { DocumentKind, DocumentSource } from '@/api/client'
import type { UploadContentType } from '@/lib/image'

export interface QueuedUpload {
  id: string
  blob: Blob
  contentType: UploadContentType
  fileName: string
  source: DocumentSource
  kind: DocumentKind
  billId?: string
  createdAt: number
  attempts: number
  lastError?: string
}

const DB = 'bill-manager'
const STORE = 'upload-queue'
const listeners = new Set<() => void>()

function open(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB, 1)
    req.onupgradeneeded = () => {
      if (!req.result.objectStoreNames.contains(STORE)) req.result.createObjectStore(STORE, { keyPath: 'id' })
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

async function run<T>(mode: IDBTransactionMode, fn: (store: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  const db = await open()
  try {
    return await new Promise<T>((resolve, reject) => {
      const tx = db.transaction(STORE, mode)
      const req = fn(tx.objectStore(STORE))
      tx.oncomplete = () => resolve(req.result)
      tx.onerror = () => reject(tx.error)
      tx.onabort = () => reject(tx.error)
    })
  } finally {
    db.close()
  }
}

// A snapshot of the queue for React (useSyncExternalStore): refreshed after every change.
let snapshot: QueuedUpload[] = []
let loaded = false

async function changed() {
  snapshot = await listQueued()
  for (const l of listeners) l()
}

/** Calls fn whenever the queue changes; returns the unsubscribe function. */
export function onQueueChange(fn: () => void): () => void {
  listeners.add(fn)
  if (!loaded) {
    loaded = true
    void changed()
  }
  return () => listeners.delete(fn)
}

export const queueSnapshot = () => snapshot

export async function enqueue(item: Omit<QueuedUpload, 'id' | 'createdAt' | 'attempts'>): Promise<QueuedUpload> {
  const full: QueuedUpload = { ...item, id: crypto.randomUUID(), createdAt: Date.now(), attempts: 0 }
  await run('readwrite', (s) => s.put(full))
  await changed()
  return full
}

export async function listQueued(): Promise<QueuedUpload[]> {
  try {
    const items = await run('readonly', (s) => s.getAll() as IDBRequest<QueuedUpload[]>)
    return items.sort((a, b) => a.createdAt - b.createdAt)
  } catch {
    return [] // private browsing or storage blocked: no queue
  }
}

export async function removeQueued(id: string) {
  await run('readwrite', (s) => s.delete(id))
  await changed()
}

export async function markAttempt(item: QueuedUpload, error: string) {
  await run('readwrite', (s) => s.put({ ...item, attempts: item.attempts + 1, lastError: error }))
  await changed()
}

/** Removes everything (sign-out: another person may use this device next). */
export async function clearQueue() {
  try {
    await run('readwrite', (s) => s.clear())
  } catch {
    // nothing stored
  }
  await changed()
}
