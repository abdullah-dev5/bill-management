import { readFileSync } from 'node:fs'
import { expect, test, type Page } from '@playwright/test'

const password = 'e2e test passphrase'

async function shot(page: Page, name: string) {
  await page.screenshot({ path: `e2e/.screenshots/${test.info().project.name}-reminders-${name}.png`, fullPage: true })
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

// Tick tokens to try: backend/.env's (a reused local API) and the one playwright.config.ts gives an
// API it starts itself. cron-job.org calls the same endpoint in production.
function tickTokens(): string[] {
  const tokens = ['e2e-tick-token-0123456789abcdef0123']
  try {
    const env = readFileSync(new URL('../../backend/.env', import.meta.url), 'utf8')
    const fromEnv = env.match(/^TICK_TOKEN=(.+)$/m)?.[1]?.trim()
    if (fromEnv) tokens.unshift(fromEnv)
  } catch {
    // no .env (CI)
  }
  return tokens
}

const pad = (n: number) => String(n).padStart(2, '0')
const iso = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`

test('reminders: due soon, recurring bills and settings', async ({ page, request }) => {
  const mobile = test.info().project.name === 'mobile'
  await page.goto('/sign-up')
  await page.getByLabel('Your name').fill('Zara Khan')
  await page.getByLabel('Email').fill(`e2e-reminders-${test.info().project.name}-${Date.now()}@example.com`)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Create account' }).click()
  await page.getByRole('button', { name: 'I’ll do this later' }).click()
  await expect(page.getByRole('heading', { name: 'Hello, Zara' })).toBeVisible()

  const now = new Date()
  const inTwoDays = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 2)
  const month = `${now.getFullYear()}-${pad(now.getMonth() + 1)}`
  const elec = await api<{ id: string }>(page, 'POST', '/profiles', { name: 'Home Electricity', typeId: 'a0000000-0000-4000-8000-000000000001' })
  await api(page, 'POST', '/bills', { profileId: elec.id, periodKey: month, dueDate: iso(inTwoDays), currentChargesMinor: 326600 })
  await api(page, 'POST', '/profiles', {
    name: 'Flat Rent', typeId: 'a0000000-0000-4000-8000-000000000011', kind: 'recurring', expectedAmountMinor: 4000000,
    autoCreateBill: true, typicalIssueDay: 1, typicalDueDay: 28,
  })

  let ticked = false
  for (const token of tickTokens()) {
    const res = await request.post('http://localhost:8080/internal/tick', { headers: { Authorization: `Bearer ${token}` } })
    if (res.ok()) {
      ticked = true
      break
    }
  }
  expect(ticked).toBeTruthy()

  // The bell shows the count; the page lists both reminders.
  await page.goto('/')
  const scope = mobile ? page.locator('#main') : page.locator('aside')
  const bell = scope.getByRole('link', { name: /Notifications, 2 unread/ })
  await expect(bell).toBeVisible()
  await bell.click()
  await expect(page.getByRole('heading', { name: 'Notifications' })).toBeVisible()
  const list = page.getByTestId('notifications')
  await expect(list).toContainText('Due in 2 days')
  await expect(list).toContainText('Rs 3,266')
  await expect(list).toContainText('Flat Rent')
  await expect(list).toContainText('Added automatically · Rs 40,000')
  await shot(page, '01-inbox')

  // Opening one marks it read and goes to the bill.
  await list.getByRole('link', { name: /Flat Rent/ }).click()
  await expect(page.getByRole('heading', { name: new RegExp(new Date(now.getFullYear(), now.getMonth(), 1).toLocaleString('en-GB', { month: 'long', year: 'numeric' })) })).toBeVisible()
  await page.goto('/notifications')
  await expect(page.getByRole('button', { name: 'Mark all as read' })).toBeVisible()
  await page.getByRole('button', { name: 'Mark all as read' }).click()
  await expect(page.getByRole('button', { name: 'Mark all as read' })).toBeHidden()

  // Settings → Reminders: lead days and switches save at once.
  await page.getByRole('link', { name: 'Settings → Reminders' }).click()
  const reminders = page.locator('#reminders')
  await expect(reminders.getByRole('heading', { name: 'Reminders' })).toBeVisible()
  await reminders.getByLabel('Remind me before the due date').click()
  await page.getByRole('option', { name: '7 days before' }).click()
  await expect(page.getByText('Reminder settings saved').first()).toBeVisible()
  await reminders.getByRole('switch', { name: /Overdue bills/ }).click()
  await page.reload()
  await expect(page.locator('#reminders').getByLabel('Remind me before the due date')).toHaveText('7 days before')
  await expect(page.locator('#reminders').getByRole('switch', { name: /Overdue bills/ })).toHaveAttribute('aria-checked', 'false')
  await shot(page, '02-settings')
})
