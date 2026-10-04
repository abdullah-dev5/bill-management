import { expect, test, type Page } from '@playwright/test'

const password = 'e2e test passphrase'

// Viewport only: a full-page capture resizes the window, which closes open popovers.
async function shot(page: Page, name: string) {
  await page.screenshot({ path: `e2e/.screenshots/${test.info().project.name}-pickers-${name}.png` })
}

test('themed date, month and dropdown pickers', async ({ page }) => {
  await page.goto('/sign-up')
  await page.getByLabel('Your name').fill('Bilal Ahmed')
  await page.getByLabel('Email').fill(`e2e-pickers-${test.info().project.name}-${Date.now()}@example.com`)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  await page.getByRole('button', { name: 'I’ll do this later' }).click()
  await expect(page.getByRole('heading', { name: 'Hello, Bilal' })).toBeVisible()

  // A gas profile: the provider list explains each choice and SSGC asks for its "Customer No.".
  await page.goto('/profiles/new')
  await page.getByLabel(/^Bill type/).click()
  await expect(page.getByRole('listbox')).toContainText('Utilities')
  await page.getByRole('option', { name: 'Gas' }).click()
  await page.getByLabel('Name', { exact: true }).fill('Home Gas')
  await page.getByLabel(/^Provider/).click()
  await expect(page.getByRole('option', { name: /SSGC/ })).toContainText('Sui Southern Gas Company')
  await shot(page, '01-provider-list')
  await page.getByRole('option', { name: /SSGC/ }).click()
  await expect(page.getByText('Printed on your bill as “Customer No.”.')).toBeVisible()
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByRole('heading', { name: 'Home Gas', exact: true })).toBeVisible()
  const profileId = page.url().match(/profiles\/([0-9a-f-]{36})/)![1]

  // New bill: pick the month from the month grid, the due date from the calendar, and type the issue date.
  await page.goto(`/bills/new?profileId=${profileId}`)
  const main = page.locator('#main')
  await main.getByRole('button', { name: 'Choose a month' }).click()
  const months = page.getByRole('grid')
  await page.getByRole('button', { name: 'Previous year' }).click()
  await months.getByRole('gridcell', { name: /^September/ }).click()
  const month = main.getByLabel(/^Billing month/)
  await expect(month).toHaveValue(/^Sep \d{4}$/)
  const year = (await month.inputValue()).slice(4)

  await main.getByLabel(/^Issue date/).fill(`26/9/${year}`)
  await main.getByLabel(/^Issue date/).press('Enter')
  await expect(main.getByLabel(/^Issue date/)).toHaveValue(`26 Sep ${year}`)

  await main.getByRole('button', { name: 'Choose a date' }).nth(1).click()
  // Opens on today; jump to October of the bill's year through the month/year view.
  await page.getByRole('button', { name: /Pick month and year/ }).click()
  while (!(await page.getByText(year, { exact: true }).isVisible())) await page.getByRole('button', { name: 'Previous year' }).click()
  await page.getByRole('gridcell', { name: /^October/ }).click()
  await shot(page, '02-calendar')
  await page.getByRole('gridcell', { name: new RegExp(`11 October ${year}$`) }).click()
  await expect(main.getByLabel(/^Due date/)).toHaveValue(`11 Oct ${year}`)

  // Keyboard: arrows move the day, Enter picks it.
  await main.getByRole('button', { name: 'Choose a date' }).nth(1).click()
  await expect(page.getByRole('gridcell', { name: new RegExp(`11 October ${year}`) })).toBeFocused()
  await page.keyboard.press('ArrowRight')
  await page.keyboard.press('Enter')
  await expect(main.getByLabel(/^Due date/)).toHaveValue(`12 Oct ${year}`)

  // Text that isn't a date is put back rather than saved.
  await main.getByLabel(/^Due date/).fill('next week')
  await main.getByLabel(/^Issue date/).click()
  await expect(main.getByLabel(/^Due date/)).toHaveValue(`12 Oct ${year}`)
})
