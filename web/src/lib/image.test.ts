import { describe, expect, it } from 'vitest'
import { formatBytes, FULL_CROP, isHeic, isPdf, outputSize, withExtension } from './image'

describe('outputSize', () => {
  it('caps the long edge at 2000 px', () => {
    expect(outputSize(4000, 3000, 0, FULL_CROP)).toMatchObject({ width: 2000, height: 1500 })
  })
  it('never upscales', () => {
    expect(outputSize(800, 600, 0, FULL_CROP)).toMatchObject({ width: 800, height: 600, scale: 1 })
  })
  it('swaps dimensions for a quarter turn', () => {
    expect(outputSize(4000, 3000, 90, FULL_CROP)).toMatchObject({ width: 1500, height: 2000 })
  })
  it('applies the crop before scaling', () => {
    expect(outputSize(4000, 3000, 0, { x: 0.25, y: 0, w: 0.5, h: 1 })).toMatchObject({ width: 1333, height: 2000 })
  })
})

describe('file helpers', () => {
  it('detects HEIC by type or extension', () => {
    expect(isHeic(new File([], 'IMG_1.HEIC'))).toBe(true)
    expect(isHeic(new File([], 'x', { type: 'image/heif' }))).toBe(true)
    expect(isHeic(new File([], 'x.jpg', { type: 'image/jpeg' }))).toBe(false)
  })
  it('detects PDFs', () => {
    expect(isPdf(new File([], 'bill.PDF'))).toBe(true)
    expect(isPdf(new File([], 'x', { type: 'application/pdf' }))).toBe(true)
  })
  it('replaces extensions', () => {
    expect(withExtension('IMG_2041.HEIC', '.jpg')).toBe('IMG_2041.jpg')
    expect(withExtension('scan', '.jpg')).toBe('scan.jpg')
    expect(withExtension('.heic', '.jpg')).toBe('document.jpg')
  })
  it('formats sizes', () => {
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(240 * 1024)).toBe('240 KB')
    expect(formatBytes(3.5 * 1024 * 1024)).toBe('3.5 MB')
  })
})
