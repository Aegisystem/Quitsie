import type { Page, Response } from '@playwright/test'
import { addExpense, balance, createGroup, deviceRows, expect, expenseRow, offlineReady, recordsTray, serverChanges, setOffline, signUp, test, unsyncedMark } from './support'

const loadedFonts = (page: Page) => page.evaluate(() => [...document.fonts].filter((font) => font.status === 'loaded').map((font) => font.family))

test('an Expense saved offline counts in the Saldo and reaches the server on reconnecting', async ({ newDevice }) => {
  const page = await newDevice()
  await signUp(page, 'Ana')
  await createGroup(page, 'Viaje a la costa', { pending: ['Beto'] })

  await setOffline(page, true)
  await expect(page.getByText('Sin conexión.')).toBeVisible()

  await addExpense(page, { description: 'Cena', amount: '120' })
  await expect(page.getByText('Guardado sin conexión.')).toBeVisible()
  await expect(unsyncedMark(expenseRow(page, 'Cena'))).toBeVisible()
  await expect(recordsTray(page)).toHaveAccessibleName('Registros: 1 registro sin sincronizar')
  await expect(balance(page)).toContainText('Te deben')
  await expect(balance(page)).toContainText('$60')
  expect((await deviceRows(page)).queue).toBe(1)
  expect((await serverChanges(page)).expenses).toEqual([])

  await setOffline(page, false)
  await expect(recordsTray(page)).toHaveAccessibleName('Registros')
  await expect(unsyncedMark(expenseRow(page, 'Cena'))).toBeHidden()
  await expect(balance(page)).toContainText('$60')
  expect((await deviceRows(page)).queue).toBe(0)
  expect((await serverChanges(page)).expenses).toMatchObject([{ description: 'Cena', total: 12000 }])
})

test('the app reloads offline from the service worker, with its font and the replica', async ({ newDevice }) => {
  const page = await newDevice()
  await signUp(page, 'Ana')
  await createGroup(page, 'Viaje a la costa', { pending: ['Beto'] })
  await addExpense(page, { description: 'Cena', amount: '120' })
  await expect(unsyncedMark(expenseRow(page, 'Cena'))).toBeHidden()
  await offlineReady(page)

  await setOffline(page, true)
  await addExpense(page, { description: 'Taxi', amount: '30' })

  // The files of the interface are immutable, so the HTTP cache of the browser
  // would also serve them offline: only this tells the service worker did it.
  const isInterface = (url: string) => !new URL(url).pathname.startsWith('/api/')
  const served: Response[] = []
  const failed: string[] = []
  page.on('response', (response) => isInterface(response.url()) && served.push(response))
  page.on('requestfailed', (request) => isInterface(request.url()) && failed.push(request.url()))
  await page.reload()

  await expect(page.getByText('Sin conexión.')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Viaje a la costa' })).toBeVisible()
  await expect(expenseRow(page, 'Cena')).toBeVisible()
  await expect(unsyncedMark(expenseRow(page, 'Taxi'))).toBeVisible()
  await expect(balance(page)).toContainText('$75')
  await expect.poll(() => loadedFonts(page)).toContain('Jost Variable')
  expect(failed).toEqual([])
  expect(served.filter((response) => !response.fromServiceWorker()).map((response) => response.url())).toEqual([])
  expect(served.some((response) => response.url().endsWith('.woff2'))).toBe(true)

  await setOffline(page, false)
  await expect(recordsTray(page)).toHaveAccessibleName('Registros')
  const { expenses } = await serverChanges(page)
  expect(expenses.map(({ description }) => description).sort()).toEqual(['Cena', 'Taxi'])
})
