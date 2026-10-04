import { expect, test, type Page } from '@playwright/test'
import { hescoLines, textPdf } from './pdf'

const password = 'e2e test passphrase'

async function shot(page: Page, name: string) {
  await page.screenshot({ path: `e2e/.screenshots/${test.info().project.name}-scan-${name}.png`, fullPage: true })
}

async function openAdd(page: Page) {
  const add = test.info().project.name === 'mobile' ? page.getByRole('button', { name: 'Add', exact: true }) : page.locator('aside').getByRole('button', { name: 'Add' })
  await add.click()
}

test('scan a bill, check it, save it to a new profile with its history', async ({ page }) => {
  await page.goto('/sign-up')
  await page.getByLabel('Your name').fill('Hina Qureshi')
  await page.getByLabel('Email').fill(`e2e-scan-${test.info().project.name}-${Date.now()}@example.com`)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  await page.getByRole('button', { name: 'I’ll do this later' }).click()
  await expect(page.getByRole('heading', { name: 'Hello, Hina' })).toBeVisible()

  // ＋ → Scan or upload a bill → choose the PDF.
  await openAdd(page)
  await page.getByRole('dialog').getByRole('button', { name: /Scan or upload a bill/ }).click()
  const capture = page.getByRole('dialog', { name: 'Scan or upload a bill' })
  await capture.getByTestId('file-input').setInputFiles({ name: 'hesco-sep-26.pdf', mimeType: 'application/pdf', buffer: textPdf(hescoLines()) })

  // The review page reads the bill and fills in the form.
  await expect(page.getByRole('heading', { name: 'Check the bill' })).toBeVisible()
  await expect(page.getByText('We filled in what we could read.')).toBeVisible({ timeout: 20_000 })
  const main = page.locator('#main')
  await expect(main.getByLabel('Profile', { exact: true })).toHaveValue('new')
  await expect(main.getByLabel('Name', { exact: true })).toHaveValue('HESCO Electricity')
  await expect(main.getByLabel(/^Reference number/)).toHaveValue('09123450012345')
  await expect(main.getByLabel(/^Billing month/)).toHaveValue('2026-09')
  await expect(main.getByLabel(/^Due date/)).toHaveValue('2026-10-05')
  await expect(main.getByLabel(/^Bill amount/)).toHaveValue('3,266')
  await expect(main.getByLabel(/^Payable after due date/)).toHaveValue('3,389')
  await expect(main.getByLabel(/^That amount applies until/)).toHaveValue('2026-10-08')
  await expect(main.getByLabel(/^Payable after that date/)).toHaveValue('3,512')
  await expect(main.getByRole('switch', { name: /Save 12 months of bill history/ })).toHaveAttribute('aria-checked', 'true')
  await shot(page, '01-review')

  await main.getByRole('button', { name: 'Save bill' }).click()
  await expect(page.getByText('Bill saved')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'September 2026', exact: true })).toBeVisible()
  await expect(main.getByRole('button', { name: /^hesco-sep-26\.pdf/ })).toBeVisible()
  await expect(main.getByText('After due date, until 08 Oct 2026')).toBeVisible()
  await expect(main.getByText('After 08 Oct 2026')).toBeVisible()
  await shot(page, '02-bill')

  // The new profile's dashboard shows the printed history next to the bill.
  await main.getByRole('link', { name: 'HESCO Electricity' }).click()
  await expect(page.getByRole('heading', { name: 'HESCO Electricity', exact: true })).toBeVisible()
  await expect(main.getByRole('heading', { name: 'Bill history' })).toBeVisible()
  await expect(main.getByRole('img', { name: 'Bill amounts by month' })).toBeVisible()
  await expect(main.getByRole('cell', { name: 'Aug 2026' })).toBeVisible()
  await shot(page, '03-profile-history')

  // Scanning the same file again finds the saved bill instead of creating a duplicate.
  await openAdd(page)
  await page.getByRole('dialog').getByRole('button', { name: /Scan or upload a bill/ }).click()
  await page
    .getByRole('dialog', { name: 'Scan or upload a bill' })
    .getByTestId('file-input')
    .setInputFiles({ name: 'hesco-sep-26.pdf', mimeType: 'application/pdf', buffer: textPdf(hescoLines()) })
  await expect(page.getByRole('heading', { name: 'September 2026', exact: true })).toBeVisible()
  await expect(page).toHaveURL(/\/bills\//)
})
