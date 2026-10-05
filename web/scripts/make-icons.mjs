// Renders the app icons (PNG) from public/favicon.svg with Playwright's Chromium.
// Run: node scripts/make-icons.mjs
import { readFileSync, writeFileSync } from 'node:fs'
import { chromium } from '@playwright/test'

const svg = readFileSync(new URL('../public/favicon.svg', import.meta.url), 'utf8')
const browser = await chromium.launch()
const page = await browser.newPage()

async function render(file, size, { maskable = false } = {}) {
  // Maskable icons keep the mark inside the 80% safe zone on a full-bleed background.
  const inner = maskable ? Math.round(size * 0.7) : size
  const bg = maskable ? '#2D2D2D' : 'transparent'
  await page.setViewportSize({ width: size, height: size })
  await page.setContent(`<html><body style="margin:0;background:${bg};display:grid;place-items:center;width:${size}px;height:${size}px">
    <div style="width:${inner}px;height:${inner}px">${svg.replace('<svg ', `<svg width="${inner}" height="${inner}" `)}</div></body></html>`)
  writeFileSync(new URL(`../public/${file}`, import.meta.url), await page.screenshot({ omitBackground: !maskable }))
}

await render('icon-192.png', 192)
await render('icon-512.png', 512)
await render('icon-maskable-512.png', 512, { maskable: true })
await render('apple-touch-icon.png', 180, { maskable: true })
await browser.close()
console.log('icons written')
