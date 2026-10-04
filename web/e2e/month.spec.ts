import { expect, test, type Page } from '@playwright/test'
import { textPdf } from './pdf'

const password = 'e2e test passphrase'

async function shot(page: Page, name: string) {
  await page.screenshot({ path: `e2e/.screenshots/${test.info().project.name}-month-${name}.png`, fullPage: true })
}

/** Calls the API with the page's session (for fast test setup). */
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

const monthKey = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`

test('monthly total, paying with a receipt, and arrears settled through a later bill', async ({ page }) => {
  await page.goto('/sign-up')
  await page.getByLabel('Your name').fill('Imran Butt')
  await page.getByLabel('Email').fill(`e2e-month-${test.info().project.name}-${Date.now()}@example.com`)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  await page.getByRole('button', { name: 'I’ll do this later' }).click()
  await expect(page.getByRole('heading', { name: 'Hello, Imran' })).toBeVisible()

  const now = new Date()
  const thisMonth = monthKey(now)
  const lastMonth = monthKey(new Date(now.getFullYear(), now.getMonth() - 1, 1))
  const types = { electricity: 'a0000000-0000-4000-8000-000000000001', gas: 'a0000000-0000-4000-8000-000000000002', rent: 'a0000000-0000-4000-8000-000000000011' }
  type Created = { id: string }
  const elec = await api<Created>(page, 'POST', '/profiles', { name: 'Home Electricity', typeId: types.electricity })
  const gas = await api<Created>(page, 'POST', '/profiles', { name: 'Home Gas', typeId: types.gas })
  const rent = await api<Created>(page, 'POST', '/profiles', { name: 'Flat Rent', typeId: types.rent })

  // Last month's electricity is still unpaid; this month's bill carries it as arrears.
  const prev = await api<Created>(page, 'POST', '/bills', { profileId: elec.id, periodKey: lastMonth, currentChargesMinor: 290000 })
  const cur = await api<Created>(page, 'POST', '/bills', { profileId: elec.id, periodKey: thisMonth, currentChargesMinor: 326600, arrearsMinor: 290000 })
  await api(page, 'POST', '/bills', { profileId: gas.id, periodKey: thisMonth, currentChargesMinor: 185000, paidNow: { paidOn: now.toISOString().slice(0, 10), method: 'cash' } })
  await api(page, 'POST', '/bills', { profileId: rent.id, periodKey: thisMonth, currentChargesMinor: 4500000 })

  // Home: this month's bills in one total (6,166 + 1,850 + 45,000 = 53,016).
  await page.goto('/')
  const main = page.locator('#main')
  await expect(main.getByTestId('month-total')).toHaveText('Rs 53,016')
  await expect(main.getByText('3 bills to pay in total')).toBeVisible()
  await expect(main.getByText(/^Paid:/)).toContainText('Rs 1,850')
  await expect(main.getByText(/^Left to pay:/)).toContainText('Rs 51,166')
  // Last month's unpaid Rs 2,900 is inside this month's arrears, so it isn't listed again.
  await expect(main.getByText(/still owed on earlier months/)).toHaveCount(0)
  await expect(main.getByText('Utilities', { exact: true }).first()).toBeVisible()
  await expect(main.getByText('Household', { exact: true }).first()).toBeVisible()
  await shot(page, '01-home')

  // Pay this month's electricity with a receipt attached.
  await page.goto(`/bills/${cur.id}`)
  await main.getByRole('button', { name: 'Mark as paid' }).click()
  const sheet = page.getByRole('dialog', { name: 'Add payment' })
  await sheet.getByRole('button', { name: 'Attach receipt' }).click()
  await page
    .getByRole('dialog', { name: 'Attach receipt' })
    .getByTestId('file-input')
    .setInputFiles({ name: 'bank-receipt.pdf', mimeType: 'application/pdf', buffer: textPdf(['Payment received', 'Rs 6,166']) })
  await expect(sheet.getByTestId('receipt-name')).toHaveText('bank-receipt.pdf')
  await sheet.getByRole('button', { name: 'Save' }).click()
  await expect(main.getByRole('button', { name: 'View receipt' })).toBeVisible()
  await shot(page, '02-paid-with-receipt')

  // Last month's bill is now paid through this month's.
  await page.goto(`/bills/${prev.id}`)
  await expect(main.getByText('Paid through a later bill, which included this amount as arrears.')).toBeVisible()
  await expect(main.getByText('marked it paid through a later bill')).toBeVisible()
  await shot(page, '03-settled')

  await page.goto('/')
  await expect(main.getByText(/^Left to pay:/)).toContainText('Rs 45,000')
})
