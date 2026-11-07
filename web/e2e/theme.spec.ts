import type { Page } from '@playwright/test'
import { expect, signUp, test } from './support'

const paper = (page: Page) => page.evaluate(() => getComputedStyle(document.body).backgroundColor)
const browserBars = (page: Page) =>
  page.evaluate(() => [...document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')].map((m) => m.content))

test('the theme follows the system until the User picks one, and this device remembers it', async ({ newDevice }) => {
  const page = await newDevice()
  await page.emulateMedia({ colorScheme: 'dark' })
  await signUp(page, 'Ana')
  await page.goto('/#/account')

  const appearance = page.getByRole('radiogroup', { name: 'Apariencia' })
  await expect(appearance.getByRole('radio', { name: 'Sistema' })).toBeChecked()
  expect(await paper(page)).toBe('rgb(18, 20, 23)')

  // Forcing light wins over a dark system, the browser's bar included.
  await appearance.getByRole('radio', { name: 'Claro' }).check()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  expect(await paper(page)).toBe('rgb(244, 245, 242)')
  expect(await browserBars(page)).toEqual(['#1f2328', '#1f2328'])

  // It is there from the first paint after a reload, before the app runs.
  await page.reload()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  await expect(appearance.getByRole('radio', { name: 'Claro' })).toBeChecked()

  await appearance.getByRole('radio', { name: 'Oscuro' }).check()
  await page.emulateMedia({ colorScheme: 'light' })
  expect(await paper(page)).toBe('rgb(18, 20, 23)')
  expect(await browserBars(page)).toEqual(['#0a0b0d', '#0a0b0d'])

  // Back to the system: nothing stays on the device and each bar has its scheme.
  await appearance.getByRole('radio', { name: 'Sistema' }).check()
  await expect(page.locator('html')).not.toHaveAttribute('data-theme')
  expect(await paper(page)).toBe('rgb(244, 245, 242)')
  expect(await browserBars(page)).toEqual(['#1f2328', '#0a0b0d'])
  expect(await page.evaluate(() => localStorage.getItem('quitsie.theme'))).toBeNull()
})
