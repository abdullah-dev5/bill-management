import { expect, test, type Page } from '@playwright/test'

const password = 'e2e test passphrase'

async function shot(page: Page, name: string) {
  await page.screenshot({ path: `e2e/.screenshots/${test.info().project.name}-docs-${name}.png`, fullPage: true })
}

/** A photo-like JPEG drawn in the browser, so it decodes like a real camera image. */
async function makePhoto(page: Page): Promise<Buffer> {
  const dataUrl = await page.evaluate(() => {
    const c = document.createElement('canvas')
    c.width = 1600
    c.height = 1200
    const ctx = c.getContext('2d')!
    ctx.fillStyle = '#d9d4c7'
    ctx.fillRect(0, 0, c.width, c.height)
    ctx.fillStyle = '#fff'
    ctx.fillRect(200, 100, 1200, 1000)
    ctx.fillStyle = '#222'
    ctx.font = 'bold 64px sans-serif'
    ctx.fillText('LESCO — Electricity Bill', 260, 240)
    ctx.font = '40px sans-serif'
    ctx.fillText('Payable within due date: Rs 8,420', 260, 400)
    return c.toDataURL('image/jpeg', 0.9)
  })
  return Buffer.from(dataUrl.split(',')[1]!, 'base64')
}

const pdf = Buffer.from('%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\ntrailer << /Root 1 0 R >>\n%%EOF\n')

test('attach, view, deduplicate and delete bill documents', async ({ page }) => {
  await page.goto('/sign-up')
  await page.getByLabel('Your name').fill('Bilal Ahmed')
  await page.getByLabel('Email').fill(`e2e-docs-${test.info().project.name}-${Date.now()}@example.com`)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  await page.getByRole('button', { name: 'I’ll do this later' }).click()
  await expect(page.getByRole('heading', { name: 'Hello, Bilal' })).toBeVisible()

  // A bill to attach documents to, created through the API with the page's session.
  const billId = await page.evaluate(async () => {
    const csrf = document.cookie.split('; ').find((c) => c.startsWith('bm_csrf='))!.slice(8)
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf }
    const profiles = await (await fetch('/api/v1/profiles')).json()
    const general = profiles.find((p: { isDefault: boolean }) => p.isDefault)
    const res = await fetch('/api/v1/bills', {
      method: 'POST',
      headers,
      body: JSON.stringify({ profileId: general.id, description: 'Plumber', expenseDate: '2026-10-01', currentChargesMinor: 250000 }),
    })
    return (await res.json()).id as string
  })

  await page.goto(`/bills/${billId}`)
  const main = page.locator('#main')
  await expect(main.getByText('No documents yet.', { exact: false })).toBeVisible()

  // Photo: pick, crop, upload.
  await main.getByRole('button', { name: 'Add document' }).click()
  const sheet = page.getByRole('dialog', { name: 'Add a document' })
  await expect(sheet).toBeVisible()
  await sheet.getByTestId('file-input').setInputFiles({ name: 'bill-photo.jpg', mimeType: 'image/jpeg', buffer: await makePhoto(page) })
  await expect(sheet.getByRole('button', { name: 'Use photo' })).toBeVisible({ timeout: 15_000 })
  const preview = sheet.getByRole('img', { name: 'Photo preview' })
  await expect(preview).toBeVisible()
  const box = (await preview.boundingBox())!
  await page.mouse.move(box.x + 2, box.y + 2)
  await page.mouse.down()
  await page.mouse.move(box.x + box.width * 0.12, box.y + box.height * 0.08, { steps: 5 })
  await page.mouse.up()
  await shot(page, '01-crop')
  await sheet.getByRole('button', { name: 'Use photo' }).click()
  await expect(page.getByText('Document added')).toBeVisible()
  await expect(main.getByRole('button', { name: /^bill-photo\.jpg/ })).toBeVisible()
  await expect(main.getByText('attached a document')).toBeVisible()

  // PDF, then the same PDF again: the second is recognised as a duplicate.
  for (const expected of ['Document added', 'This file is already saved.']) {
    await main.getByRole('button', { name: 'Add document' }).click()
    await page.getByRole('dialog', { name: 'Add a document' }).getByRole('button', { name: 'Receipt' }).click()
    await page.getByRole('dialog', { name: 'Add a document' }).getByTestId('file-input').setInputFiles({ name: 'receipt.pdf', mimeType: 'application/pdf', buffer: pdf })
    await expect(page.getByText(expected)).toBeVisible()
  }
  await expect(main.getByRole('button', { name: /^receipt\.pdf/ })).toHaveCount(1)
  await shot(page, '02-documents')

  // Open the photo, then delete it.
  await main.getByRole('button', { name: /bill-photo\.jpg/ }).click()
  const viewer = page.getByRole('dialog', { name: 'bill-photo.jpg' })
  await expect(viewer.getByRole('img', { name: 'bill-photo.jpg' })).toBeVisible()
  await expect(viewer.getByRole('link', { name: 'Download' })).toBeVisible()
  await shot(page, '03-viewer')
  await viewer.getByRole('button', { name: 'Delete' }).click()
  await page.getByRole('dialog', { name: 'Delete this document?' }).getByRole('button', { name: 'Delete' }).click()
  await expect(page.getByText('Document deleted')).toBeVisible()
  await expect(main.getByRole('button', { name: /^bill-photo\.jpg/ })).toHaveCount(0)
  await expect(main.getByText('removed a document')).toBeVisible()

  // Settings shows storage use and the AI consent switch (off by default).
  await page.goto('/settings')
  await expect(main.getByText(/of 100\.0 MB used · 1 document$/)).toBeVisible()
  const consent = main.getByRole('switch', { name: /Use AI-assisted extraction/ })
  await expect(consent).toHaveAttribute('aria-checked', 'false')
  await consent.click()
  await expect(page.getByText('AI-assisted extraction turned on')).toBeVisible()
  await expect(consent).toHaveAttribute('aria-checked', 'true')
  await shot(page, '04-settings')
})
