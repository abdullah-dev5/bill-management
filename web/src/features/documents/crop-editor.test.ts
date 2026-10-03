import { describe, expect, it } from 'vitest'
import { FULL_CROP } from '@/lib/image'
import { dragCrop } from './crop-editor'

describe('dragCrop', () => {
  it('moves the box but keeps it inside the image', () => {
    const box = { x: 0.2, y: 0.2, w: 0.5, h: 0.5 }
    const moved = dragCrop(box, 'move', 0.1, -0.1)
    expect(moved.x).toBeCloseTo(0.3)
    expect(moved.y).toBeCloseTo(0.1)
    expect(dragCrop(box, 'move', 1, 1)).toEqual({ x: 0.5, y: 0.5, w: 0.5, h: 0.5 })
  })
  it('resizes from a corner', () => {
    const out = dragCrop(FULL_CROP, 'nw', 0.25, 0.1)
    expect(out.x).toBeCloseTo(0.25)
    expect(out.y).toBeCloseTo(0.1)
    expect(out.w).toBeCloseTo(0.75)
    expect(out.h).toBeCloseTo(0.9)
  })
  it('never collapses below the minimum size', () => {
    const out = dragCrop(FULL_CROP, 'se', -2, -2)
    expect(out.w).toBeCloseTo(0.1)
    expect(out.h).toBeCloseTo(0.1)
  })
})
