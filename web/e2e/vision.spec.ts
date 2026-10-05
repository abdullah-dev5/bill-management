import { expect, test, type Page } from '@playwright/test'

// M4 exit: every question in docs/01-product-vision.md §3 is answered from the UI.

const password = 'e2e test passphrase'
const TYPE = {
  electricity: 'a0000000-0000-4000-8000-000000000001',
  gas: 'a0000000-0000-4000-8000-000000000002',
  internet: 'a0000000-0000-4000-8000-000000000004',
  maintenance: 'a0000000-0000-4000-8000-000000000012',
  electrician: 'a0000000-0000-4000-8000-000000000031',
}
const LESCO = 'b0000000-0000-4000-8000-000000000001'

async function shot(page: Page, name: string) {
  await page.screenshot({ path: `e2e/.screenshots/${test.info().project.name}-vision-${name}.png`, fullPage: true })
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

const now = new Date()
const pad = (n: number) => String(n).padStart(2, '0')
const monthKey = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}`
const monthsAgo = (n: number) => monthKey(new Date(now.getFullYear(), now.getMonth() - n, 1))
const monthName = (key: string) => new Date(Number(key.slice(0, 4)), Number(key.slice(5)) - 1, 1).toLocaleString('en-GB', { month: 'long' })
const today = `${monthKey(now)}-${pad(now.getDate())}`

// The winter (Nov–Feb) that has most recently ended, as the search understands "last winter".
function lastWinter(): string[] {
  const nowIdx = now.getFullYear() * 12 + now.getMonth()
  let start = now.getFullYear() * 12 + 10
  if (start > nowIdx) start -= 12
  if (start + 3 >= nowIdx) start -= 12
  return [0, 1, 2, 3].map((i) => `${Math.floor((start + i) / 12)}-${pad(((start + i) % 12) + 1)}`)
}

test('answers the six questions from the product vision', async ({ page }) => {
  const mobile = test.info().project.name === 'mobile'
  await page.goto('/sign-up')
  await page.getByLabel('Your name').fill('Hina Raza')
  await page.getByLabel('Email').fill(`e2e-vision-${test.info().project.name}-${Date.now()}@example.com`)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  await page.getByRole('button', { name: 'I’ll do this later' }).click()
  await expect(page.getByRole('heading', { name: 'Hello, Hina' })).toBeVisible()

  type Created = { id: string }
  const paid = (key: string) => ({ paidNow: { paidOn: key < monthKey(now) ? `${key}-20` : today, method: 'cash' } })
  const elec = await api<Created>(page, 'POST', '/profiles', { name: 'Home Electricity', typeId: TYPE.electricity, providerId: LESCO })
  const gas = await api<Created>(page, 'POST', '/profiles', { name: 'Kitchen Gas', typeId: TYPE.gas })
  const net = await api<Created>(page, 'POST', '/profiles', { name: 'Home Internet', typeId: TYPE.internet })
  const repairs = await api<Created>(page, 'POST', '/profiles', { name: 'Repairs', typeId: TYPE.maintenance })

  // Electricity: every month this year at Rs 4,000 (this month unpaid), plus last year's September and December.
  const thisYearMonths = now.getMonth() + 1
  for (let m = 1; m <= thisYearMonths; m++) {
    const key = `${now.getFullYear()}-${pad(m)}`
    await api(page, 'POST', '/bills', { profileId: elec.id, periodKey: key, currentChargesMinor: 400000, ...(m < thisYearMonths ? paid(key) : {}) })
  }
  for (const key of [`${now.getFullYear() - 1}-09`, `${now.getFullYear() - 1}-12`]) {
    await api(page, 'POST', '/bills', { profileId: elec.id, periodKey: key, currentChargesMinor: 390000, ...paid(key) })
  }

  // Gas last winter, with meter readings: 30 + 45 + 50 + 35 = 160 units.
  const winter = lastWinter()
  let reading = 5000
  for (const [i, key] of winter.entries()) {
    const units = [30, 45, 50, 35][i]!
    await api(page, 'POST', '/bills', {
      profileId: gas.id, periodKey: key, currentChargesMinor: units * 4000, previousReading: reading, currentReading: reading + units, ...paid(key),
    })
    reading += units
  }

  // Internet: Rs 2,500 for three months, then Rs 3,000 for the last three (this month unpaid).
  for (let ago = 5; ago >= 0; ago--) {
    const key = monthsAgo(ago)
    if (winter.includes(key)) continue
    await api(page, 'POST', '/bills', { profileId: net.id, periodKey: key, currentChargesMinor: ago >= 3 ? 250000 : 300000, ...(ago > 0 ? paid(key) : {}) })
  }

  // Maintenance this year: an electrician (sub-category) and a general repair, Rs 4,000 together.
  const day1 = `${monthKey(now)}-01`
  await api(page, 'POST', '/bills', { profileId: repairs.id, typeId: TYPE.electrician, description: 'Kitchen wiring', expenseDate: day1, currentChargesMinor: 250000, ...paid(monthKey(now)) })
  await api(page, 'POST', '/bills', { profileId: repairs.id, description: 'Door hinge', expenseDate: day1, currentChargesMinor: 150000, ...paid(monthKey(now)) })

  const main = page.locator('#main')

  // 1. How much did electricity cost me this year?
  await page.goto('/analytics?preset=year')
  const byType = main.getByTestId('by-type')
  await expect(byType.getByRole('listitem').filter({ hasText: 'Electricity' })).toContainText(`Rs ${(4000 * thisYearMonths).toLocaleString('en-US')}`)
  // 6. How much did I spend on household maintenance? (sub-categories roll up)
  await expect(byType.getByRole('listitem').filter({ hasText: 'Maintenance' })).toContainText('Rs 4,000')
  await shot(page, '01-this-year')

  // 2. How much gas did I use last winter? Search understands the season…
  await page.goto('/search?q=' + encodeURIComponent('gas last winter'))
  await expect(main.getByTestId('search-understood')).toContainText(`${monthName(winter[0]!)} ${winter[0]!.slice(0, 4)} – ${monthName(winter[3]!)} ${winter[3]!.slice(0, 4)}`)
  await expect(main.getByRole('heading', { name: '4 bills' })).toBeVisible()
  // …and analytics totals the units for those months.
  await page.goto(`/analytics?preset=custom&from=${winter[0]}&to=${winter[3]}`)
  await expect(main.getByTestId('consumption-Kitchen Gas')).toContainText('160')
  await shot(page, '02-gas-winter')

  // 3. Did my internet bill go up? Last 3 months vs the 3 before: Rs 9,000 vs Rs 7,500 = +20%.
  await page.goto('/analytics?preset=3m')
  await expect(main.getByRole('row', { name: /Home Internet/ })).toContainText('+20%')

  // 4. Which bills are unpaid right now?
  await page.goto('/bills')
  await main.getByRole('button', { name: 'Unpaid', exact: true }).click()
  await expect(main.getByRole('link', { name: /Home Electricity|Home Internet/ })).toHaveCount(2)
  await expect(main.getByRole('link', { name: /Kitchen Gas|Repairs/ })).toHaveCount(0)
  await shot(page, '04-unpaid')

  // 5. Where is my September electricity bill?
  if (mobile) await main.getByRole('link', { name: 'Search' }).click()
  else await page.keyboard.press('/')
  await page.getByRole('searchbox', { name: /Search bills/ }).fill('electricity september')
  const sepYear = now.getMonth() >= 8 ? now.getFullYear() : now.getFullYear() - 1
  const sept = main.getByRole('link', { name: new RegExp(`Home Electricity · September ${sepYear}`) })
  await expect(sept).toBeVisible()
  await expect(main.getByRole('heading', { name: '1 bill' })).toBeVisible()
  await sept.click()
  await expect(page.getByRole('heading', { name: `September ${sepYear}` })).toBeVisible()
  await shot(page, '05-september')
})

test('export bills and payments as CSV, and download a full backup', async ({ page }) => {
  await page.goto('/sign-up')
  await page.getByLabel('Your name').fill('Omar Saeed')
  await page.getByLabel('Email').fill(`e2e-export-${test.info().project.name}-${Date.now()}@example.com`)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  await page.getByRole('button', { name: 'I’ll do this later' }).click()
  await expect(page.getByRole('heading', { name: 'Hello, Omar' })).toBeVisible()
  const p = await api<{ id: string }>(page, 'POST', '/profiles', { name: 'Flat Rent', typeId: 'a0000000-0000-4000-8000-000000000011' })
  await api(page, 'POST', '/bills', { profileId: p.id, periodKey: monthKey(now), currentChargesMinor: 4500000, paidNow: { paidOn: today, method: 'bank_transfer' } })

  await page.goto('/settings')
  // The household dropdowns show their saved values (regression: they once showed "Choose…").
  await expect(page.getByLabel('Currency')).toHaveText('PKR — Pakistani rupee')
  await expect(page.getByLabel('Timezone')).toHaveText('Asia/Karachi')
  const data = page.locator('#main').locator('div', { has: page.getByRole('heading', { name: 'Your data' }) }).last()
  await expect(page.getByRole('heading', { name: 'Your data' })).toBeVisible()

  let download = page.waitForEvent('download')
  await page.getByRole('link', { name: 'Bills (CSV)' }).click()
  let file = await download
  expect(file.suggestedFilename()).toBe(`bills-from-${now.getFullYear()}-01-01-to-${now.getFullYear()}-12-31.csv`)
  const csv = await (await file.createReadStream()).toArray().then((c) => Buffer.concat(c).toString('utf8'))
  expect(csv).toContain('Profile,Bill type,Provider,Billing month')
  expect(csv).toContain(`Flat Rent,Rent,,${monthKey(now)}`)
  expect(csv).toContain('45000.00')

  await page.getByRole('button', { name: 'Everything' }).click()
  download = page.waitForEvent('download')
  await page.getByRole('link', { name: 'Payments (CSV)' }).click()
  file = await download
  expect(file.suggestedFilename()).toBe('payments.csv')
  const pays = await (await file.createReadStream()).toArray().then((c) => Buffer.concat(c).toString('utf8'))
  expect(pays).toContain('Bank transfer')

  download = page.waitForEvent('download')
  await page.getByRole('link', { name: 'Download backup (ZIP)' }).click()
  file = await download
  expect(file.suggestedFilename()).toMatch(/^bills-backup-\d{4}-\d{2}-\d{2}\.zip$/)
  const zip = await (await file.createReadStream()).toArray().then((c) => Buffer.concat(c))
  expect(zip.subarray(0, 2).toString()).toBe('PK')
  await expect(data).toBeVisible()
  await shot(page, 'settings-data')
})
