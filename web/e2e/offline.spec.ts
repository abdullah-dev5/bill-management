import { expect, test, type Page } from '@playwright/test'
import { textPdf } from './pdf'

const password = 'e2e test passphrase'

async function shot(page: Page, name: string) {
  await page.screenshot({ path: `e2e/.screenshots/${test.info().project.name}-offline-${name}.png`, fullPage: true })
}

async function signUp(page: Page, name: string) {
  await page.goto('/sign-up')
  await page.getByLabel('Your name').fill(name)
  await page.getByLabel('Email').fill(`e2e-offline-${test.info().project.name}-${Date.now()}@example.com`)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  await page.getByRole('button', { name: 'I’ll do this later' }).click()
  await expect(page.getByRole('heading', { name: `Hello, ${name.split(' ')[0]}` })).toBeVisible()
}

async function createExpense(page: Page): Promise<string> {
  return page.evaluate(async () => {
    const csrf = document.cookie.split('; ').find((c) => c.startsWith('bm_csrf='))!.slice(8)
    const profiles = await (await fetch('/api/v1/profiles')).json()
    const general = profiles.find((p: { isDefault: boolean }) => p.isDefault)
    const res = await fetch('/api/v1/bills', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
      body: JSON.stringify({ profileId: general.id, description: 'Plumber', expenseDate: '2026-10-01', currentChargesMinor: 250000 }),
    })
    return (await res.json()).id as string
  })
}

test('files added offline wait on the device and upload when back online', async ({ page, context }) => {
  await signUp(page, 'Nadia Hussain')
  const billId = await createExpense(page)
  await page.goto(`/bills/${billId}`)
  const main = page.locator('#main')
  await expect(main.getByText('No documents yet.', { exact: false })).toBeVisible()

  // Go offline and add a receipt.
  await context.setOffline(true)
  const banner = page.getByTestId('connection-banner')
  await expect(banner).toContainText('You’re offline.')
  await main.getByRole('button', { name: 'Add document' }).click()
  const sheet = page.getByRole('dialog', { name: 'Add a document' })
  await sheet.getByTestId('file-input').setInputFiles({ name: 'receipt.pdf', mimeType: 'application/pdf', buffer: textPdf(['Paid Rs 2,500']) })
  await expect(page.getByText('It’s saved on this device').first()).toBeVisible()
  await expect(sheet).toBeHidden()
  await expect(banner).toContainText('1 saved file is waiting to upload.')
  await shot(page, '01-waiting')

  // When the connection returns it uploads on its own, and the document is on the bill.
  await context.setOffline(false)
  await expect(page.getByText('Saved file uploaded')).toBeVisible({ timeout: 30_000 })
  await expect(banner).toBeHidden()
  await page.reload()
  await expect(main.getByRole('link', { name: /receipt\.pdf/ }).or(main.getByText('receipt.pdf')).first()).toBeVisible()
})

// The installed app: runs against the production build (`pnpm build && pnpm exec vite preview`),
// because the service worker only exists there. Skipped when the preview isn't running.
test.describe('installed app (production build)', () => {
  test.use({ baseURL: 'http://localhost:4173' })

  test('installable, and opens offline with the last-seen data', async ({ page, context, request }) => {
    const up = await request.get('http://localhost:4173/manifest.webmanifest').then((r) => r.ok()).catch(() => false)
    test.skip(!up, 'vite preview is not running on :4173')

    const manifest = await (await request.get('http://localhost:4173/manifest.webmanifest')).json()
    expect(manifest.display).toBe('standalone')
    expect(manifest.icons.map((i: { sizes: string }) => i.sizes)).toEqual(expect.arrayContaining(['192x192', '512x512']))

    await signUp(page, 'Imran Ali')
    // Wait for the service worker to take control, then load Home once online so it's cached.
    await page.evaluate(() => navigator.serviceWorker.ready)
    await page.reload()
    await expect.poll(() => page.evaluate(() => Boolean(navigator.serviceWorker.controller))).toBe(true)
    await expect(page.getByRole('heading', { name: 'Hello, Imran' })).toBeVisible()
    await page.waitForLoadState('networkidle')

    await context.setOffline(true)
    await page.reload()
    await expect(page.getByRole('heading', { name: 'Hello, Imran' })).toBeVisible()
    await expect(page.getByTestId('connection-banner')).toContainText('You’re offline.')
    await shot(page, '02-offline-home')
    await context.setOffline(false)
  })
})
