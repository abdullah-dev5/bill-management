import { expect, test, type Page } from '@playwright/test'

const password = 'e2e test passphrase'

function uniqueEmail(tag: string) {
  return `e2e-${tag}-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`
}

function uniqueReference() {
  // LESCO reference numbers are 14 digits.
  return '04' + String(Date.now()).slice(-12)
}

async function shot(page: Page, name: string) {
  await page.screenshot({ path: `e2e/.screenshots/${test.info().project.name}-${name}.png`, fullPage: true })
}

test('sign up, onboard, add a bill, pay it, log an expense', async ({ page }) => {
  const email = uniqueEmail(test.info().project.name)

  // Sign up → onboarding
  await page.goto('/sign-up')
  await shot(page, '01-sign-up')
  await page.getByLabel('Your name').fill('Ayesha Khan')
  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  await expect(page.getByRole('heading', { name: 'What bills do you pay?' })).toBeVisible()

  // Pick Electricity and Maintenance; give the electricity profile a LESCO reference.
  await page.getByRole('button', { name: 'Electricity' }).click()
  await page.getByRole('button', { name: 'Maintenance' }).click()
  await page.getByLabel(/^Provider/).click()
  await page.getByRole('option', { name: 'LESCO — Lahore' }).click()
  await expect(page.getByLabel(/^Provider/)).toHaveText('LESCO — Lahore')
  await page.getByLabel(/^Reference \/ account no\./).fill(uniqueReference())
  await shot(page, '02-onboarding')
  await page.getByRole('button', { name: 'Create 2 profiles' }).click()

  // Home lists the pinned profiles.
  await expect(page.getByRole('heading', { name: 'Hello, Ayesha' })).toBeVisible()
  const main = page.locator('#main')
  await expect(main.getByRole('link', { name: /Home Electricity/ }).first()).toBeVisible()
  await expect(main.getByRole('link', { name: /Maintenance/ }).first()).toBeVisible()
  await shot(page, '03-home-empty')

  // Open the profile dashboard and add September's bill.
  await main.getByRole('link', { name: /Home Electricity/ }).first().click() // the pinned tile (before recent activity)
  await expect(page.getByRole('heading', { name: 'Home Electricity', exact: true })).toBeVisible()
  await main.getByRole('link', { name: 'Add bill' }).first().click()
  await expect(page.getByRole('heading', { name: 'New bill' })).toBeVisible()
  await page.getByLabel(/^Billing month/).fill('2026-09')
  await page.getByLabel(/^Due date/).fill('2026-10-08')
  await page.getByLabel(/^Bill amount/).fill('8420')
  await page.getByLabel(/^Payable after due date/).fill('9262')
  await shot(page, '04-new-bill')
  await page.getByRole('button', { name: 'Save' }).click()

  // Back on the profile: the featured card shows the unpaid bill.
  await expect(page.getByRole('heading', { name: 'Home Electricity', exact: true })).toBeVisible()
  await expect(main.getByText('SEPTEMBER 2026', { exact: true })).toBeVisible()
  await expect(main.getByText('Rs 8,420').first()).toBeVisible()
  await shot(page, '05-profile-unpaid')

  // Mark it paid from the featured card.
  await main.getByRole('button', { name: 'Mark as paid' }).click()
  const sheet = page.getByRole('dialog', { name: 'Add payment' })
  await expect(sheet).toBeVisible()
  await shot(page, '06-payment-sheet')
  await sheet.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('Payment recorded')).toBeVisible()
  await expect(main.getByText('Paid', { exact: true }).first()).toBeVisible()
  await shot(page, '07-profile-paid')

  // Bill detail shows the payment and the history.
  await main.getByRole('link', { name: 'View' }).click()
  await expect(page.getByRole('heading', { name: 'September 2026', exact: true })).toBeVisible()
  await expect(main.getByText('added a payment')).toBeVisible()
  await shot(page, '08-bill-detail')

  // A quick expense from the ＋ sheet goes to General expenses as a paid entry.
  const addButton = test.info().project.name === 'mobile' ? page.getByRole('button', { name: 'Add', exact: true }) : page.locator('aside').getByRole('button', { name: 'Add' })
  await addButton.click()
  await page.getByRole('dialog').getByRole('button', { name: /Quick expense/ }).click()
  await expect(page.getByRole('heading', { name: 'New expense' })).toBeVisible()
  await page.getByLabel('What was it for?').fill('Electrician — kitchen wiring')
  await page.getByLabel(/^Amount/).fill('3,500')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('heading', { name: 'General expenses', exact: true })).toBeVisible()
  await expect(main.getByText('Electrician — kitchen wiring').first()).toBeVisible()

  // Home totals reflect both.
  await page.goto('/')
  await expect(main.getByText('Rs 0').first()).toBeVisible() // nothing outstanding
  await shot(page, '09-home-filled')

  // Sign out, then back in.
  await page.goto('/settings')
  await shot(page, '10-settings')
  await page.getByRole('button', { name: 'Sign out', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Welcome back' })).toBeVisible()
  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByRole('heading', { name: 'Hello, Ayesha' })).toBeVisible()
})

test('signed-out visitors are sent to sign in and returned afterwards', async ({ page }) => {
  await page.goto('/profiles')
  await expect(page.getByRole('heading', { name: 'Welcome back' })).toBeVisible()
  await expect(page).toHaveURL(/redirect=/)
})
