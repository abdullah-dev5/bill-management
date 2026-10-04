import { api, ApiError, unwrap, type Doc, type DocumentKind, type DocumentSource } from '@/api/client'
import { sha256Hex, type UploadContentType } from '@/lib/image'

export interface PreparedUpload {
  blob: Blob
  contentType: UploadContentType
  fileName: string
  source: DocumentSource
}

export interface UploadResult {
  document: Doc
  /** The household already had this exact file; nothing was uploaded. */
  duplicate: boolean
}

/** Starts the upload, sends the bytes to the signed link and asks the API to verify them. */
export async function uploadDocument(
  file: PreparedUpload,
  opts: { kind: DocumentKind; billId?: string; onProgress?: (fraction: number) => void },
): Promise<UploadResult> {
  const started = await unwrap(
    api.POST('/documents', {
      body: {
        fileName: file.fileName,
        contentType: file.contentType,
        sizeBytes: file.blob.size,
        sha256: await sha256Hex(file.blob),
        kind: opts.kind,
        source: file.source,
        billId: opts.billId,
      },
    }),
  )
  if (started.duplicate || !started.upload) return { document: started.document, duplicate: true }

  await put(started.upload.url, started.upload.headers, file.blob, opts.onProgress)
  const document = await unwrap(api.POST('/documents/{id}/complete', { params: { path: { id: started.document.id } } }))
  return { document, duplicate: false }
}

// XHR rather than fetch, for upload progress on slow mobile connections.
function put(url: string, headers: Record<string, string>, body: Blob, onProgress?: (f: number) => void): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('PUT', url)
    for (const [k, v] of Object.entries(headers)) xhr.setRequestHeader(k, v)
    xhr.upload.onprogress = (e) => e.lengthComputable && onProgress?.(e.loaded / e.total)
    xhr.onload = () =>
      xhr.status >= 200 && xhr.status < 300
        ? resolve()
        : reject(new ApiError(xhr.status, { code: 'upload_failed', message: 'The upload failed. Please try again.' }))
    xhr.onerror = () => reject(new ApiError(0, { code: 'network', message: "Couldn't reach the server. Check your connection and try again." }))
    xhr.send(body)
  })
}
