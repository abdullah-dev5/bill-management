import { expect, test, type Page } from '@playwright/test'

const password = 'e2e test passphrase'

async function shot(page: Page, name: string) {
  await page.screenshot({ path: `e2e/.screenshots/${test.info().project.name}-analytics-${name}.png`, fullPage: true })
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

const monthKey = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
const monthsAgo = (n: number) => {
  const d = new Date()
  return monthKey(new Date(d.getFullYear(), d.getMonth() - n, 1))
}

test('analytics: spending by month, type and profile, with consumption', async ({ page }) => {
  await page.goto('/sign-up')
  await page.getByLabel('Your name').fill('Asma Iqbal')
  await page.getByLabel('Email').fill(`e2e-analytics-${test.info().project.name}-${Date.now()}@example.com`)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  await page.getByRole('button', { name: 'I’ll do this later' }).click()
  await expect(page.getByRole('heading', { name: 'Hello, Asma' })).toBeVisible()

  type Created = { id: string }
  const elec = await api<Created>(page, 'POST', '/profiles', { name: 'Home Electricity', typeId: 'a0000000-0000-4000-8000-000000000001' })
  const rent = await api<Created>(page, 'POST', '/profiles', { name: 'Flat Rent', typeId: 'a0000000-0000-4000-8000-000000000011' })
  // Electricity for the last three months, with readings; rent this month.
  let reading = 10000
  for (const [ago, amount, units] of [[2, 420000, 300], [1, 380000, 260], [0, 300000, 200]] as const) {
    await api(page, 'POST', '/bills', {
      profileId: elec.id, periodKey: monthsAgo(ago), currentChargesMinor: amount,
      previousReading: reading, currentReading: reading + units,
    })
    reading += units
  }
  await api(page, 'POST', '/bills', { profileId: rent.id, periodKey: monthsAgo(0), currentChargesMinor: 4500000 })

  // Bottom bar / sidebar → Analytics; the last 3 months: 4,200 + 3,800 + 3,000 + 45,000 = 56,000.
  await page.goto('/')
  const nav = test.info().project.name === 'mobile' ? page.getByRole('navigation', { name: 'Primary' }) : page.locator('aside')
  await nav.getByRole('link', { name: 'Analytics' }).click()
  await expect(page.getByRole('heading', { name: 'Analytics' })).toBeVisible()
  const main = page.locator('#main')
  await main.getByRole('button', { name: 'Last 3 months' }).click()
  await expect(page).toHaveURL(/preset=3m/)
  await expect(main.getByTestId('analytics-total')).toHaveText('Rs 56,000')
  await expect(main.getByRole('img', { name: 'Amounts by month, stacked by group' })).toBeVisible()
  await expect(main.getByText('Electricity', { exact: true }).first()).toBeVisible()
  await expect(main.getByRole('link', { name: /Home Electricity/ }).first()).toBeVisible()

  // Consumption for the metered profile: 300 + 260 + 200 = 760 units.
  await expect(main.getByTestId('consumption-Home Electricity')).toContainText('760')

  // The data behind the chart.
  await main.getByRole('button', { name: 'Show data' }).click()
  await expect(main.getByRole('table').first()).toContainText('Rs 48,000')
  await shot(page, '01-three-months')

  // This month only, and the cash basis (nothing paid yet).
  await main.getByRole('button', { name: 'This month' }).click()
  await expect(main.getByTestId('analytics-total')).toHaveText('Rs 48,000')
  await main.getByRole('button', { name: 'Cash paid' }).click()
  await expect(page).toHaveURL(/basis=cash/)
  await expect(main.getByText('Nothing to show yet')).toBeVisible()
})
