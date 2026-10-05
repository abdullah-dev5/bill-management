import { expect, test, type Page } from '@playwright/test'

// Tier 3 of the reading chain: the phone reads the photo itself with Tesseract.js.
// Uses the real engine and model (served from /tesseract), so it takes a little longer.

const password = 'e2e test passphrase'

async function billPhoto(page: Page): Promise<Buffer> {
  const dataUrl = await page.evaluate(() => {
    const c = document.createElement('canvas')
    c.width = 1400
    c.height = 1000
    const ctx = c.getContext('2d')!
    ctx.fillStyle = '#fff'
    ctx.fillRect(0, 0, c.width, c.height)
    ctx.fillStyle = '#111'
    ctx.font = 'bold 56px Arial, sans-serif'
    ctx.fillText('HESCO ELECTRICITY BILL', 80, 120)
    ctx.font = '44px Arial, sans-serif'
    const lines = [
      'REFERENCE NO 09 12345 0012345',
      'BILL MONTH SEP 26',
      'DUE DATE 05 OCT 26',
      'PAYABLE WITHIN DUE DATE 3,266',
      'PAYABLE AFTER DUE DATE 3,389',
    ]
    lines.forEach((l, i) => ctx.fillText(l, 80, 260 + i * 110))
    return c.toDataURL('image/jpeg', 0.95)
  })
  return Buffer.from(dataUrl.split(',')[1]!, 'base64')
}

test('read a bill photo on this phone', async ({ page }) => {
  test.setTimeout(150_000)
  await page.goto('/sign-up')
  await page.getByLabel('Your name').fill('Saima Noor')
  await page.getByLabel('Email').fill(`e2e-device-ocr-${test.info().project.name}-${Date.now()}@example.com`)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  await page.getByRole('button', { name: 'I’ll do this later' }).click()
  await expect(page.getByRole('heading', { name: 'Hello, Saima' })).toBeVisible()

  const billId = await page.evaluate(async () => {
    const csrf = document.cookie.split('; ').find((c) => c.startsWith('bm_csrf='))!.slice(8)
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf }
    const p = await (await fetch('/api/v1/profiles', { method: 'POST', headers, body: JSON.stringify({ name: 'Home Electricity', typeId: 'a0000000-0000-4000-8000-000000000001' }) })).json()
    const b = await (await fetch('/api/v1/bills', { method: 'POST', headers, body: JSON.stringify({ profileId: p.id, periodKey: '2026-09', currentChargesMinor: 100 }) })).json()
    return b.id as string
  })

  await page.goto(`/bills/${billId}`)
  const main = page.locator('#main')
  await main.getByRole('button', { name: 'Add document' }).click()
  const sheet = page.getByRole('dialog', { name: 'Add a document' })
  await sheet.getByTestId('file-input').setInputFiles({ name: 'bill.jpg', mimeType: 'image/jpeg', buffer: await billPhoto(page) })
  await sheet.getByRole('button', { name: 'Use photo' }).click()
  await expect(page.getByText('Document added')).toBeVisible()

  // Open the document; its reading panel offers to read it on the device.
  await main.getByRole('button', { name: /bill\.jpg/ }).click()
  const viewer = page.getByRole('dialog', { name: 'bill.jpg' })
  await viewer.getByRole('button', { name: 'Read on this phone' }).click()
  await expect(viewer.getByText('Read with this phone')).toBeVisible({ timeout: 120_000 })
  await expect(viewer.getByText('Rs 3,266').first()).toBeVisible()
  await expect(viewer.getByText('09123450012345').first()).toBeVisible()
  await page.screenshot({ path: `e2e/.screenshots/${test.info().project.name}-device-ocr.png`, fullPage: true })
})
