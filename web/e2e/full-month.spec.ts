import { expect, test, type Page } from '@playwright/test'
import { hescoLines, textPdf } from './pdf'

// M3 exit criterion: a full month of real household bills can be tracked end to end.

const password = 'e2e test passphrase'

async function shot(page: Page, name: string) {
  await page.screenshot({ path: `e2e/.screenshots/${test.info().project.name}-fullmonth-${name}.png`, fullPage: true })
}

const monthKey = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
const isoDay = (d: Date, day: number) => `${monthKey(d)}-${String(day).padStart(2, '0')}`

async function openAdd(page: Page) {
  const add = test.info().project.name === 'mobile' ? page.getByRole('button', { name: 'Add', exact: true }) : page.locator('aside').getByRole('button', { name: 'Add' })
  await add.click()
}

async function openProfile(page: Page, name: string) {
  await page.goto('/profiles')
  await page.locator('#main').getByRole('link', { name: new RegExp(`^${name}`) }).first().click()
  await expect(page.getByRole('heading', { name, exact: true })).toBeVisible()
}

async function addBillFromProfile(page: Page, profile: string, fill: () => Promise<void>) {
  await page.locator('#main').getByRole('link', { name: /^Add (bill|entry)$/ }).first().click()
  await expect(page.getByRole('heading', { name: /^New (bill|expense|entry)/ })).toBeVisible()
  await fill()
  await page.locator('#main').getByRole('button', { name: 'Save' }).click()
  // Saving returns to the profile; wait for it before doing anything else.
  await expect(page.getByRole('heading', { name: profile, exact: true })).toBeVisible()
}

test('track a full month of household bills', async ({ page }) => {
  test.setTimeout(180_000)
  const now = new Date()
  const thisMonth = monthKey(now)
  const today = now.toISOString().slice(0, 10)
  const main = page.locator('#main')

  // 1. Sign up and pick the bills this household pays.
  await page.goto('/sign-up')
  await page.getByLabel('Your name').fill('Fatima Raza')
  await page.getByLabel('Email').fill(`e2e-fullmonth-${test.info().project.name}-${Date.now()}@example.com`)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  for (const name of ['Electricity', 'Gas', 'Internet', 'Rent']) await page.getByRole('button', { name, exact: true }).click()
  await page.getByRole('button', { name: 'Create 4 profiles' }).click()
  await expect(page.getByRole('heading', { name: 'Hello, Fatima' })).toBeVisible()

  // 2. Electricity: upload the bill PDF, check it, save it to Home Electricity with its history.
  await openAdd(page)
  await page.getByRole('dialog').getByRole('button', { name: /Scan or upload a bill/ }).click()
  await page
    .getByRole('dialog', { name: 'Scan or upload a bill' })
    .getByTestId('file-input')
    .setInputFiles({ name: 'hesco-sep-26.pdf', mimeType: 'application/pdf', buffer: textPdf(hescoLines()) })
  await expect(page.getByText('We filled in what we could read.')).toBeVisible({ timeout: 20_000 })
  const profileSelect = main.getByLabel('Profile', { exact: true })
  await profileSelect.click()
  await page.getByRole('option', { name: 'Home Electricity' }).click()
  await expect(profileSelect).toHaveText('Home Electricity')
  await main.getByRole('button', { name: 'Save bill' }).click()
  await expect(page.getByRole('heading', { name: 'September 2026', exact: true })).toBeVisible({ timeout: 15_000 })
  const electricityBill = page.url()

  // 3. Gas: typed in with meter readings, paid with a receipt.
  await openProfile(page, 'Home Gas')
  await addBillFromProfile(page, 'Home Gas', async () => {
    await main.getByLabel(/^Billing month/).fill(thisMonth)
    await main.getByLabel(/^Due date/).fill(isoDay(now, 25))
    await main.getByLabel(/^Bill amount/).fill('1,850')
    await main.getByLabel(/^Previous reading/).fill('1200')
    await main.getByLabel(/^Present reading/).fill('1240')
    await expect(main.getByText(/From the readings: 40/)).toBeVisible()
  })
  await expect(page.getByRole('heading', { name: 'Home Gas', exact: true })).toBeVisible()
  await main.getByRole('button', { name: 'Mark as paid' }).click()
  const pay = page.getByRole('dialog', { name: 'Add payment' })
  await pay.getByRole('button', { name: 'Attach receipt' }).click()
  await page
    .getByRole('dialog', { name: 'Attach receipt' })
    .getByTestId('file-input')
    .setInputFiles({ name: 'gas-receipt.pdf', mimeType: 'application/pdf', buffer: textPdf(['SNGPL', 'Paid Rs 1,850']) })
  await expect(pay.getByTestId('receipt-name')).toHaveText('gas-receipt.pdf')
  await pay.getByRole('button', { name: 'Save' }).click()
  await expect(pay).toBeHidden() // the page behind an open sheet is hidden from the test
  await expect(main.getByRole('button', { name: 'Mark as paid' })).toHaveCount(0)

  // 4. Rent: typed in, paid in two parts.
  await openProfile(page, 'Rent')
  await addBillFromProfile(page, 'Rent', async () => {
    await main.getByLabel(/^Billing month/).fill(thisMonth)
    await main.getByLabel(/^Due date/).fill(isoDay(now, 28))
    await main.getByLabel(/^Bill amount/).fill('45,000')
  })
  await main.getByRole('link', { name: 'View' }).click()
  await main.getByRole('button', { name: 'Add payment' }).click()
  await pay.getByLabel(/^Amount/).fill('20,000')
  await pay.getByRole('button', { name: 'Save' }).click()
  await expect(main.getByText('Partially paid').first()).toBeVisible()

  // 5. Internet: typed in as already paid.
  await openProfile(page, 'Home Internet')
  await addBillFromProfile(page, 'Home Internet', async () => {
    await main.getByLabel(/^Billing month/).fill(thisMonth)
    await main.getByLabel(/^Bill amount/).fill('2,500')
    await main.getByRole('switch', { name: 'Already paid' }).click()
  })

  // 6. A one-off expense.
  await openAdd(page)
  await page.getByRole('dialog').getByRole('button', { name: /Quick expense/ }).click()
  await main.getByLabel('What was it for?').fill('Plumber — kitchen tap')
  await main.getByLabel(/^Amount/).fill('1,500')
  await main.getByLabel(/^Date/).fill(today)
  await main.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('heading', { name: 'General expenses', exact: true })).toBeVisible()

  // 7. Home: this month in one figure. 1,850 + 45,000 + 2,500 + 1,500 = 50,850; paid 25,850; left 25,000.
  await page.goto('/')
  await expect(main.getByTestId('month-total')).toHaveText('Rs 50,850')
  await expect(main.getByText('4 bills to pay in total')).toBeVisible()
  await expect(main.getByText(/^Left to pay:/)).toContainText('Rs 25,000')
  await shot(page, '01-home')

  // 8. Finish paying the rent; nothing is left this month.
  await page.goto('/bills?status=unpaid')
  await expect(main.getByRole('button', { name: 'Unpaid', pressed: true })).toBeVisible()
  await page.reload() // the filter lives in the URL
  await expect(main.getByRole('button', { name: 'Unpaid', pressed: true })).toBeVisible()
  await main.getByRole('link', { name: /Rent/ }).first().click()
  await main.getByRole('button', { name: 'Add payment' }).click()
  await expect(pay.getByLabel(/^Amount/)).toHaveValue('25,000')
  await pay.getByRole('button', { name: 'Save' }).click()
  await expect(pay).toBeHidden()
  // Fully paid: the bill no longer offers to add a payment.
  await expect(main.getByRole('button', { name: 'Add payment' })).toHaveCount(0)
  await page.goto('/')
  await expect(main.getByText(/^Left to pay:/)).toContainText('Rs 0')

  // 9. The electricity profile dashboard: bills, payments, documents, history.
  await page.goto(electricityBill)
  await main.getByRole('button', { name: 'Mark as paid' }).click()
  await pay.getByLabel(/^Paid on/).fill('2026-10-03')
  await pay.getByRole('button', { name: 'Save' }).click()
  await expect(main.getByText('added a payment')).toBeVisible()
  await openProfile(page, 'Home Electricity')
  await expect(main.getByRole('heading', { name: 'Bill history' })).toBeVisible()
  await expect(main.getByRole('cell', { name: 'Sept 2026' }).or(main.getByRole('cell', { name: 'Sep 2026' })).first()).toBeVisible()
  await main.getByRole('tab', { name: 'Payments' }).click()
  await expect(main.getByText(/1 payment · Rs.3,266 in total/)).toBeVisible()
  await main.getByRole('tab', { name: 'Documents' }).click()
  await expect(main.getByRole('button', { name: /hesco-sep-26\.pdf/ })).toBeVisible()
  await main.getByRole('tab', { name: 'Bills' }).click()
  await main.getByRole('button', { name: 'Paid', exact: true }).click()
  await expect(page).toHaveURL(/tab=bills.*status=paid|status=paid.*tab=bills/)
  await expect(main.getByText('September 2026').first()).toBeVisible()
  await shot(page, '02-electricity-bills')
})
