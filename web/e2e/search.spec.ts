import { expect, test, type Page } from '@playwright/test'

const password = 'e2e test passphrase'

async function shot(page: Page, name: string) {
  await page.screenshot({ path: `e2e/.screenshots/${test.info().project.name}-search-${name}.png`, fullPage: true })
}

async function api<T>(page: Page, method: string, path: string, body?: unknown): Promise<T> {
  return page.evaluate(
    async ({ method, path, body }) => {
      const csrf = document.cookie.split('; ').find((c) => c.startsWith('bm_csrf='))!.slice(8)
      const res = await fetch('/api/v1' + path, {
        method,
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf },
        body: body ? JSON.stringify(body) : undefined,
      })
      if (!res.ok) throw new Error(`${method} ${path}: ${res.status} ${await res.text()}`)
      return res.status === 204 ? null : res.json()
    },
    { method, path, body },
  )
}

const lastYear = new Date().getFullYear() - 1
const reference = () => `09${String(Date.now()).slice(-12)}`

test('search, bill filters and recent activity', async ({ page }) => {
  const mobile = test.info().project.name === 'mobile'
  await page.goto('/sign-up')
  await page.getByLabel('Your name').fill('Sana Malik')
  await page.getByLabel('Email').fill(`e2e-search-${test.info().project.name}-${Date.now()}@example.com`)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  await page.getByRole('button', { name: 'I’ll do this later' }).click()
  await expect(page.getByRole('heading', { name: 'Hello, Sana' })).toBeVisible()

  type Created = { id: string }
  const ref = reference()
  const elec = await api<Created>(page, 'POST', '/profiles', {
    name: 'Home Electricity', typeId: 'a0000000-0000-4000-8000-000000000001', providerId: 'b0000000-0000-4000-8000-000000000001', referenceNumber: ref,
  })
  const gas = await api<Created>(page, 'POST', '/profiles', { name: 'Kitchen Gas', typeId: 'a0000000-0000-4000-8000-000000000002' })
  for (const m of ['08', '09', '10']) {
    await api(page, 'POST', '/bills', { profileId: elec.id, periodKey: `${lastYear}-${m}`, currentChargesMinor: 410000 })
  }
  await api(page, 'POST', '/bills', { profileId: gas.id, periodKey: `${lastYear}-12`, currentChargesMinor: 95000, notes: 'Heater season' })
  const gasBill = await api<Created>(page, 'POST', '/bills', { profileId: gas.id, periodKey: `${lastYear}-11`, currentChargesMinor: 60000 })
  await api(page, 'POST', `/bills/${gasBill.id}/payments`, { paidOn: `${lastYear}-12-05`, amountMinor: 60000, method: 'easypaisa' })

  // Home: recent activity lists the payment and the bills.
  await page.reload()
  const activity = page.getByTestId('recent-activity')
  await expect(activity).toContainText('Payment of Rs 600')
  await expect(activity).toContainText('Easypaisa')
  await expect(activity.getByText('Bill added').first()).toBeVisible()

  // Open search: "/" on desktop, the search button on phones.
  if (mobile) await page.locator('#main').getByRole('link', { name: 'Search' }).click()
  else await page.keyboard.press('/')
  await expect(page.getByRole('heading', { name: 'Search' })).toBeVisible()
  const box = page.getByRole('searchbox', { name: /Search bills/ })
  await expect(box).toBeFocused()

  // A natural phrase becomes type + month.
  await box.fill(`electricity september ${lastYear}`)
  const understood = page.getByTestId('search-understood')
  await expect(understood).toContainText('Electricity')
  await expect(understood).toContainText(`September ${lastYear}`)
  await expect(page).toHaveURL(/q=electricity/)
  const main = page.locator('#main')
  await expect(main.getByRole('heading', { name: '1 bill' })).toBeVisible()
  await expect(main.getByRole('link', { name: new RegExp(`Home Electricity · September ${lastYear}`) })).toBeVisible()
  await shot(page, '01-phrase')

  // Reference number (with spaces) finds the profile.
  await box.fill(`${ref.slice(0, 2)} ${ref.slice(2, 7)} ${ref.slice(7)}`)
  await expect(understood).toContainText('Reference number')
  await expect(main.getByRole('link', { name: /Home Electricity.*•••• / })).toBeVisible()
  await expect(main.getByRole('heading', { name: '3 bills' })).toBeVisible()

  // Notes are searched as text.
  await box.fill('heater')
  await expect(main.getByRole('link', { name: new RegExp(`Kitchen Gas · December ${lastYear}`) })).toBeVisible()

  // Nothing found.
  await box.fill('zzqx')
  await expect(page.getByText('Nothing found')).toBeVisible()

  // Bills page: filter by type and amount through the sheet; filters show as chips and live in the URL.
  await page.goto('/bills')
  await main.getByRole('button', { name: 'Filters' }).click()
  const sheet = page.getByRole('dialog', { name: 'Filter bills' })
  await sheet.getByRole('button', { name: 'Gas', exact: true }).click()
  await sheet.getByLabel('Up to').fill('700')
  await shot(page, '02-filter-sheet')
  await sheet.getByRole('button', { name: 'Show bills' }).click()
  await expect(page).toHaveURL(/type=a0000000-0000-4000-8000-000000000002/)
  await expect(page).toHaveURL(/max=700/)
  const chips = main.getByRole('list', { name: 'Active filters' })
  await expect(chips.getByRole('button', { name: 'Remove filter: Gas' })).toBeVisible()
  await expect(main.getByRole('link', { name: /Kitchen Gas/ })).toHaveCount(1)
  await shot(page, '03-filtered')

  // Remove the amount: both gas bills.
  await chips.getByRole('button', { name: /Remove filter: Up to/ }).click()
  await expect(main.getByRole('link', { name: /Kitchen Gas/ })).toHaveCount(2)
  await expect(main.getByRole('link', { name: /Home Electricity/ })).toHaveCount(0)
  await chips.getByRole('button', { name: 'Clear all' }).click()
  await expect(main.getByRole('link', { name: /Home Electricity/ })).toHaveCount(3)
})
