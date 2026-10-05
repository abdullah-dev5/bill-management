import { expect, test, type Page } from '@playwright/test'
import { hescoLines, textPdf } from './pdf'

// M6: a scanned copy of a bill that's already in the app merges into it; where they disagree,
// the person decides, and a wrong merge can be undone.

const password = 'e2e test passphrase'

async function shot(page: Page, name: string) {
  await page.screenshot({ path: `e2e/.screenshots/${test.info().project.name}-reconcile-${name}.png`, fullPage: true })
}

test('a scanned copy merges into the bill; differences are decided by the person', async ({ page }) => {
  await page.goto('/sign-up')
  await page.getByLabel('Your name').fill('Rabia Aslam')
  await page.getByLabel('Email').fill(`e2e-reconcile-${test.info().project.name}-${Date.now()}@example.com`)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  await page.getByRole('button', { name: 'I’ll do this later' }).click()
  await expect(page.getByRole('heading', { name: 'Hello, Rabia' })).toBeVisible()

  // September's bill was typed in with a different amount and no issue date.
  const billId = await page.evaluate(async () => {
    const csrf = document.cookie.split('; ').find((c) => c.startsWith('bm_csrf='))!.slice(8)
    const headers = { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf }
    const p = await (await fetch('/api/v1/profiles', {
      method: 'POST', headers,
      body: JSON.stringify({ name: 'HESCO Electricity', typeId: 'a0000000-0000-4000-8000-000000000001', providerId: 'b0000000-0000-4000-8000-000000000007', referenceNumber: '09123450012345' }),
    })).json()
    const b = await (await fetch('/api/v1/bills', {
      method: 'POST', headers, body: JSON.stringify({ profileId: p.id, periodKey: '2026-09', dueDate: '2026-10-05', currentChargesMinor: 330000 }),
    })).json()
    return b.id as string
  })

  // Scan the bill's PDF; it's recognised as the same bill, and attached.
  // A fresh load (the profile was created behind the app's back), through the installed app's
  // "Scan a bill" shortcut, which opens the add sheet.
  await page.goto('/?add=scan')
  await page.getByRole('dialog').getByRole('button', { name: /Scan or upload a bill/ }).click()
  await page.getByRole('dialog', { name: 'Scan or upload a bill' }).getByTestId('file-input')
    .setInputFiles({ name: 'hesco-sep-26.pdf', mimeType: 'application/pdf', buffer: textPdf(hescoLines()) })
  await expect(page.getByText('We filled in what we could read.')).toBeVisible({ timeout: 20_000 })
  const main = page.locator('#main')
  await main.getByRole('button', { name: /Save bill/ }).click()
  await page.getByRole('button', { name: 'Attach to existing bill' }).click()
  await expect(page.getByText(/details? differs?: check/)).toBeVisible()
  await expect(page).toHaveURL(new RegExp(`/bills/${billId}`))

  // The bill kept its own amount, and shows what differs.
  const conflicts = main.getByTestId('conflicts')
  await expect(conflicts).toContainText('Payable by the due date')
  await expect(conflicts).toContainText('On the bill Rs 3,300 · uploaded file says Rs 3,266')
  const sources = main.getByTestId('sources')
  await expect(sources).toContainText('typed in')
  await expect(sources).toContainText('uploaded file')
  await shot(page, '01-needs-review')

  // Home and the bills list point to it.
  await page.goto('/')
  await page.getByTestId('review-tile').click()
  await expect(page).toHaveURL(/review=true/)
  await main.getByRole('link', { name: /HESCO Electricity · September 2026/ }).click()
  await expect(main.getByTestId('conflicts')).toBeVisible()
  await expect(main.getByTestId('sources')).toBeVisible()

  // Decide: the uploaded copy is right, for every difference.
  while ((await main.getByTestId('conflicts').count()) > 0) {
    await main.getByTestId('conflicts').getByRole('button', { name: 'Decide' }).first().click()
    const sheet = page.getByRole('dialog', { name: /Which .* is right\?/ })
    await expect(sheet).toBeVisible()
    await sheet.getByRole('button', { name: /^Use .* from the uploaded file/ }).click()
    await expect(sheet).toBeHidden()
  }
  await expect(main.getByTestId('conflicts')).toBeHidden()
  await expect(main.getByRole('term').filter({ hasText: 'Payable by due date' }).locator('..')).toContainText('Rs 3,266')
  await shot(page, '02-resolved')

  // Undo the merge: the scan's document comes off the bill, ready to save as its own.
  await main.getByRole('button', { name: /Separate the uploaded file/ }).click()
  const confirm = page.getByRole('dialog', { name: 'Separate this copy?' })
  await expect(confirm).toBeVisible()
  // Regression: the sources table once widened the page on phones (taps then missed their targets).
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(await page.evaluate(() => document.documentElement.clientWidth))
  await confirm.getByRole('button', { name: 'Separate' }).click()
  await expect(page.getByText('Separated. The document is no longer on this bill.')).toBeVisible()
  await expect(main.getByTestId('sources')).toBeHidden()
})
