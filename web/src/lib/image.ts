/** Client-side document preparation (docs/05 §2): HEIC → JPEG, rotate, crop, compress. */

export const MAX_UPLOAD_BYTES = 10 * 1024 * 1024
export const TARGET_IMAGE_BYTES = 1024 * 1024
const MAX_EDGE = 2000

export type UploadContentType = 'image/jpeg' | 'image/png' | 'image/webp' | 'application/pdf'

/** A crop rectangle in fractions (0–1) of the rotated image. */
export interface CropRect {
  x: number
  y: number
  w: number
  h: number
}

export const FULL_CROP: CropRect = { x: 0, y: 0, w: 1, h: 1 }

export type Rotation = 0 | 90 | 180 | 270

export function isHeic(file: File): boolean {
  return /image\/hei[cf]/i.test(file.type) || /\.hei[cf]$/i.test(file.name)
}

export function isPdf(file: File): boolean {
  return file.type === 'application/pdf' || /\.pdf$/i.test(file.name)
}

export function isImage(file: File): boolean {
  return file.type.startsWith('image/') || isHeic(file)
}

/** Decodes any supported image to a bitmap, applying EXIF orientation. */
export async function decodeImage(file: File): Promise<ImageBitmap> {
  let blob: Blob = file
  if (isHeic(file)) {
    const { default: heic2any } = await import('heic2any')
    const out = await heic2any({ blob: file, toType: 'image/jpeg', quality: 0.92 })
    blob = Array.isArray(out) ? out[0]! : out
  }
  return createImageBitmap(blob, { imageOrientation: 'from-image' })
}

/** Output size for an image: crop applied, long edge capped at MAX_EDGE. */
export function outputSize(width: number, height: number, rotation: Rotation, crop: CropRect, maxEdge = MAX_EDGE) {
  const turned = rotation === 90 || rotation === 270
  const w = (turned ? height : width) * crop.w
  const h = (turned ? width : height) * crop.h
  const scale = Math.min(1, maxEdge / Math.max(w, h))
  return { width: Math.max(1, Math.round(w * scale)), height: Math.max(1, Math.round(h * scale)), scale }
}

/** Renders the rotated, cropped, resized image and encodes it as a JPEG of at most ~1 MB. */
export async function renderJpeg(bitmap: ImageBitmap, rotation: Rotation, crop: CropRect): Promise<Blob> {
  let maxEdge = MAX_EDGE
  for (;;) {
    const canvas = drawTransformed(bitmap, rotation, crop, maxEdge)
    for (const quality of [0.85, 0.75, 0.65, 0.55]) {
      const blob = await toBlob(canvas, quality)
      if (blob.size <= TARGET_IMAGE_BYTES) return blob
    }
    if (maxEdge <= 1000) return toBlob(canvas, 0.5)
    maxEdge = Math.round(maxEdge * 0.8)
  }
}

/** Draws the rotated, cropped image onto a new canvas whose long edge is at most maxEdge. */
export function drawTransformed(bitmap: ImageBitmap, rotation: Rotation, crop: CropRect, maxEdge: number): HTMLCanvasElement {
  const { width, height, scale } = outputSize(bitmap.width, bitmap.height, rotation, crop, maxEdge)
  const turned = rotation === 90 || rotation === 270
  const fullW = (turned ? bitmap.height : bitmap.width) * scale
  const fullH = (turned ? bitmap.width : bitmap.height) * scale

  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const ctx = canvas.getContext('2d')
  if (!ctx) throw new Error('Canvas is not available')
  ctx.fillStyle = '#fff'
  ctx.fillRect(0, 0, width, height)
  ctx.imageSmoothingQuality = 'high'
  // Move the crop origin to (0,0), then rotate the full image about its centre.
  ctx.translate(-crop.x * fullW, -crop.y * fullH)
  ctx.translate(fullW / 2, fullH / 2)
  ctx.rotate((rotation * Math.PI) / 180)
  const drawW = bitmap.width * scale
  const drawH = bitmap.height * scale
  ctx.drawImage(bitmap, -drawW / 2, -drawH / 2, drawW, drawH)
  return canvas
}

function toBlob(canvas: HTMLCanvasElement, quality: number): Promise<Blob> {
  return new Promise((resolve, reject) =>
    canvas.toBlob((b) => (b ? resolve(b) : reject(new Error('Could not encode the image'))), 'image/jpeg', quality),
  )
}

export async function sha256Hex(blob: Blob): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', await blob.arrayBuffer())
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('')
}

/** Replaces the extension, e.g. "IMG_2041.HEIC" → "IMG_2041.jpg". */
export function withExtension(name: string, ext: string): string {
  const base = name.replace(/\.[^./\\]+$/, '') || 'document'
  return base + ext
}

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}
