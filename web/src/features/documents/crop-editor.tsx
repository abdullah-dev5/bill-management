import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { drawTransformed, FULL_CROP, type CropRect, type Rotation } from '@/lib/image'

const MIN_SIZE = 0.1
type Handle = 'move' | 'nw' | 'ne' | 'sw' | 'se'

const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v))

/** Applies a pointer drag of (dx, dy) fractions to the crop rectangle. */
export function dragCrop(start: CropRect, handle: Handle, dx: number, dy: number): CropRect {
  if (handle === 'move') {
    return { ...start, x: clamp(start.x + dx, 0, 1 - start.w), y: clamp(start.y + dy, 0, 1 - start.h) }
  }
  let { x, y } = start
  let right = start.x + start.w
  let bottom = start.y + start.h
  if (handle === 'nw' || handle === 'sw') x = clamp(x + dx, 0, right - MIN_SIZE)
  if (handle === 'ne' || handle === 'se') right = clamp(right + dx, x + MIN_SIZE, 1)
  if (handle === 'nw' || handle === 'ne') y = clamp(y + dy, 0, bottom - MIN_SIZE)
  if (handle === 'sw' || handle === 'se') bottom = clamp(bottom + dy, y + MIN_SIZE, 1)
  return { x, y, w: right - x, h: bottom - y }
}

/** Shows the rotated photo with a draggable crop box. */
export function CropEditor({ bitmap, rotation, crop, onCropChange }: {
  bitmap: ImageBitmap
  rotation: Rotation
  crop: CropRect
  onCropChange: (c: CropRect) => void
}) {
  const { t } = useTranslation()
  const [src, setSrc] = useState<string>()
  const frame = useRef<HTMLDivElement>(null)
  const drag = useRef<{ handle: Handle; startX: number; startY: number; start: CropRect }>(null)

  useEffect(() => {
    let url: string | undefined
    let cancelled = false
    drawTransformed(bitmap, rotation, FULL_CROP, 1200).toBlob((b) => {
      if (!b || cancelled) return
      url = URL.createObjectURL(b)
      setSrc(url)
    }, 'image/jpeg', 0.8)
    return () => {
      cancelled = true
      if (url) URL.revokeObjectURL(url)
    }
  }, [bitmap, rotation])

  const onDown = (e: ReactPointerEvent<HTMLElement>) => {
    e.preventDefault()
    e.stopPropagation()
    e.currentTarget.setPointerCapture(e.pointerId)
    const handle = (e.currentTarget.dataset.handle ?? 'move') as Handle
    drag.current = { handle, startX: e.clientX, startY: e.clientY, start: crop }
  }
  const onMove = (e: ReactPointerEvent) => {
    const d = drag.current
    const box = frame.current?.getBoundingClientRect()
    if (!d || !box) return
    onCropChange(dragCrop(d.start, d.handle, (e.clientX - d.startX) / box.width, (e.clientY - d.startY) / box.height))
  }
  const onUp = () => {
    drag.current = null
  }

  const pct = (v: number) => `${v * 100}%`
  return (
    <div className="flex justify-center overflow-hidden rounded-field bg-card-muted p-4">
      <div ref={frame} className="relative inline-block touch-none select-none" onPointerMove={onMove} onPointerUp={onUp} onPointerCancel={onUp}>
        {src ? (
          <img src={src} alt={t('documents.preview')} className="block max-h-[52dvh] max-w-full" draggable={false} />
        ) : (
          <div className="h-64 w-48 animate-pulse rounded bg-line" />
        )}
        {src && (
          <div
            className="absolute cursor-move border-2 border-white shadow-[0_0_0_9999px_rgba(0,0,0,0.45)]"
            style={{ left: pct(crop.x), top: pct(crop.y), width: pct(crop.w), height: pct(crop.h) }}
            data-handle="move"
            onPointerDown={onDown}
            aria-label={t('documents.cropArea')}
          >
            {(['nw', 'ne', 'sw', 'se'] as const).map((h) => (
              <span
                key={h}
                data-handle={h}
                onPointerDown={onDown}
                className="absolute size-7 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white bg-featured/80"
                style={{ left: h.endsWith('w') ? '0%' : '100%', top: h.startsWith('n') ? '0%' : '100%' }}
                aria-hidden
              />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
