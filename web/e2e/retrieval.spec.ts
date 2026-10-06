import { expect, test, type Page } from '@playwright/test'
import { textPdf } from './pdf'

// M6 phase 2: bills from the official source. Automatic uses the development "sample" connector
// (no network); assisted opens the provider's own page, which is intercepted here.

const password = 'e2e test passphrase'
const SAMPLE_REF = '09123450012345'

async function shot(page: Page, name: string) {
  await page.screenshot({ path: `e2e/.screenshots/${test.info().project.name}-retrieval-${name}.png`, fullPage: true })
}

async function signUp(page: Page, name: string) {
  await page.goto('/sign-up')
  await page.getByLabel('Your name').fill(name)
  await page.getByLabel('Email').fill(`e2e-retrieval-${test.info().project.name}-${Date.now()}@example.com`)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  await page.getByRole('button', { name: 'I’ll do this later' }).click()
  await expect(page.getByRole('heading', { name: `Hello, ${name.split(' ')[0]}` })).toBeVisible()
}

async function api<T>(page: Page, method: string, path: string, body?: unknown): Promise<T> {
  return page.evaluate(
    async ({ method, path, body }) => {
      const csrf = document.cookie.split('; ').find((c) => c.startsWith('bm_csrf='))!.slice(8)
      const res = await fetch('/api/v1' + path, { method, headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf }, body: body ? JSON.stringify(body) : undefined })
      if (!res.ok) throw new Error(`${method} ${path}: ${res.status} ${await res.text()}`)
      return res.status === 204 ? null : res.json()
    },
    { method, path, body },
  )
}

test('fetch the official bill; it merges with the bill already typed in', async ({ page }) => {
  await signUp(page, 'Kamran Shah')
  // The sample connector knows one reference number; each run is a fresh household, so it's free.
  const p = await api<{ id: string }>(page, 'POST', '/profiles', {
    name: 'HESCO Electricity', typeId: 'a0000000-0000-4000-8000-000000000001', providerId: 'b0000000-0000-4000-8000-000000000007', referenceNumber: SAMPLE_REF,
  })
  await api(page, 'POST', '/bills', { profileId: p.id, periodKey: '2026-09', dueDate: '2026-10-05', currentChargesMinor: 330000 })

  await page.goto(`/profiles/${p.id}`)
  const card = page.getByTestId('official-bill')
  await expect(card).toContainText('Check the official site automatically')
  await card.getByRole('switch', { name: /Check the official site automatically/ }).click()
  await expect(card.getByRole('switch', { name: /Check the official site automatically/ })).toHaveAttribute('aria-checked', 'true')

  await card.getByRole('button', { name: 'Fetch now' }).click()
  await expect(page.getByRole('region', { name: /Notifications/ }).getByText(/some details differ/)).toBeVisible()
  await expect(card.getByTestId('last-retrieval')).toContainText('Last checked')
  await shot(page, '01-fetched')
  await page.getByRole('button', { name: 'View bill' }).click()

  // One bill, two copies: typed in and official; the differences wait for a decision.
  const main = page.locator('#main')
  await expect(main.getByTestId('sources')).toContainText('official site')
  await expect(main.getByTestId('sources')).toContainText('typed in')
  await expect(main.getByTestId('conflicts')).toContainText('official site says Rs 3,266')
})

test('assisted: open the provider’s page, then upload the official bill', async ({ page, context }) => {
  // Never touch the real site in tests.
  await context.route('https://bill.pitc.com.pk/**', (route) => route.fulfill({ status: 200, contentType: 'text/html', body: '<h1>PITC (test stand-in)</h1>' }))
  await signUp(page, 'Saba Iqbal')
  const p = await api<{ id: string }>(page, 'POST', '/profiles', {
    name: 'Home Electricity', typeId: 'a0000000-0000-4000-8000-000000000001', providerId: 'b0000000-0000-4000-8000-000000000001', referenceNumber: `35${String(Date.now()).slice(-12)}`,
  })
  await page.goto(`/profiles/${p.id}`)
  const card = page.getByTestId('official-bill')
  await expect(card).toContainText('security check')
  const popup = page.waitForEvent('popup')
  await card.getByRole('button', { name: 'Open the official bill page' }).click()
  expect((await popup).url()).toBe('https://bill.pitc.com.pk/lescobill')
  await (await popup).close()

  await card.getByRole('button', { name: 'Upload the official bill' }).click()
  await page.getByRole('dialog', { name: 'Upload the official bill' }).getByTestId('file-input')
    .setInputFiles({ name: 'lesco-sep.pdf', mimeType: 'application/pdf', buffer: textPdf(['LESCO', 'Official duplicate bill', 'Payable within due date 4,100']) })
  await expect(page.getByTestId('official-copy')).toBeVisible({ timeout: 20_000 })
  await shot(page, '02-official-upload')
  const main = page.locator('#main')
  await main.getByLabel(/^Billing month/).fill('Sep 2026')
  await main.getByLabel(/^Bill amount/).fill('4,100')
  await main.getByRole('button', { name: /Save bill/ }).click()
  await expect(page.getByRole('heading', { name: 'September 2026' })).toBeVisible()
  // A single source shows no comparison table; the history says where it came from.
  await expect(main.getByText('created the bill')).toBeVisible()
})
