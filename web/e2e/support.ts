import { randomInt, randomUUID } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { test as base, expect, type BrowserContext, type Locator, type Page } from '@playwright/test'

const serverLog = join(process.env.QUITSIE_E2E_DIR ?? '', 'server.log')
// Node may not resolve *.localhost, so what the tests ask the server goes straight to it.
const server = `http://127.0.0.1:${process.env.QUITSIE_E2E_PORT}`

export const test = base.extend<{ newDevice: (path?: string) => Promise<Page> }>({
  // A device is a browser context of its own: storage, cookies, service worker.
  newDevice: async ({ browser }, use, testInfo) => {
    const contexts: BrowserContext[] = []
    const crashes: string[] = []
    await use(async (path = '/') => {
      // Each one is a visitor of its own, as behind the tunnel. All on 127.0.0.1,
      // a run with its precaching would reach the 300 requests per minute per IP.
      const context = await browser.newContext({ extraHTTPHeaders: { 'CF-Connecting-IP': `198.51.${randomInt(256)}.${randomInt(256)}` } })
      contexts.push(context)
      const page = await context.newPage()
      page.on('pageerror', (error) => crashes.push(error.stack ?? error.message))
      await page.goto(path)
      return page
    })
    await Promise.all(contexts.map((context) => context.close()))
    if (testInfo.status !== testInfo.expectedStatus) await testInfo.attach('server.log', { path: serverLog })
    expect(crashes).toEqual([])
  },
})

export { expect }

export interface Person {
  name: string
  email: string
}

const newPerson = (name: string): Person => ({ name, email: `${name.toLowerCase()}-${randomUUID()}@quitsie.test` })

// The last code the server logged for this email, which replaces the earlier ones.
function loginCode(email: string): string {
  const line = readFileSync(serverLog, 'utf8')
    .split('\n')
    .findLast((text) => text.includes(`to=${email} `))
  return line?.match(/subject="[^"]*(\d{6})"/)?.[1] ?? ''
}

export async function signIn(page: Page, { email }: Person) {
  await page.getByLabel('Tu email').fill(email)
  await page.getByRole('button', { name: 'Enviarme un código' }).click()
  // The server logs the code before it answers, and only then this field shows.
  const code = page.getByLabel('Código de 6 dígitos')
  await expect(code).toBeVisible()
  await code.fill(loginCode(email))
}

// A new User, from the code to the end of the sign-up.
export async function signUp(page: Page, name: string): Promise<Person> {
  const person = newPerson(name)
  await signIn(page, person)
  await page.getByLabel('Tu nombre').fill(name)
  await page.getByRole('button', { name: 'Empezar' }).click()
  await expect(page.getByLabel('Tu nombre')).toBeHidden()
  return person
}

export async function createGroup(page: Page, name: string, { pending = [] }: { pending?: string[] } = {}) {
  await page.getByRole('link', { name: 'Nuevo grupo' }).click()
  await page.getByLabel('Nombre del grupo').fill(name)
  for (const person of pending) {
    await page.getByLabel('Nombre', { exact: true }).fill(person)
    await page.getByRole('button', { name: 'Añadir a la lista' }).click()
  }
  await page.getByRole('button', { name: 'Crear grupo' }).click()
  await expect(page.getByRole('heading', { name, level: 1 })).toBeVisible()
}

// From the ledger, with the default split: paid by the User, equal parts.
export async function addExpense(page: Page, { description, amount }: { description: string; amount: string }) {
  await page.getByRole('link', { name: 'Añadir gasto' }).click()
  await page.getByLabel('Importe').fill(amount)
  await page.getByLabel('Descripción').fill(description)
  await page.getByRole('button', { name: /^Guardar/ }).click()
  await expect(expenseRow(page, description)).toBeVisible()
}

export const expenseRow = (page: Page, description: string): Locator => page.getByRole('listitem').filter({ hasText: description })

export const unsyncedMark = (row: Locator): Locator => row.getByRole('img', { name: 'Sin sincronizar' })

// The band with the User's own Saldo at the top of the ledger.
export const balance = (page: Page): Locator => page.locator('.band').first()

// The tray in the top bar, which says how many records are waiting.
export const recordsTray = (page: Page): Locator => page.getByRole('link', { name: /^Registros/ })

// Installed and active, so everything it precaches is already in its cache.
export async function offlineReady(page: Page) {
  await page.evaluate(async () => {
    await navigator.serviceWorker.ready
  })
}

export const setOffline = (page: Page, offline: boolean) => page.context().setOffline(offline)

interface ServerExpense {
  id: string
  description: string
  total: number
  version: number
  deletedAt?: string
  paidShares: { memberId: string; amount: number }[]
  owedShares: { memberId: string; amount: number }[]
}

// What the server holds for this device's User, whether the device is online or not.
export async function serverChanges(page: Page): Promise<{ expenses: ServerExpense[]; members: { name: string; userId?: string }[] }> {
  const cookies = await page.context().cookies()
  const response = await fetch(`${server}/api/v1/sync?since=0`, {
    headers: { cookie: cookies.map(({ name, value }) => `${name}=${value}`).join('; ') },
  })
  expect(response.status).toBe(200)
  return (await response.json()) as { expenses: ServerExpense[]; members: { name: string; userId?: string }[] }
}

// How many rows each IndexedDB table of the device holds.
export function deviceRows(page: Page): Promise<Record<string, number>> {
  return page.evaluate(async () => {
    const db = await new Promise<IDBDatabase>((resolve, reject) => {
      const open = indexedDB.open('quitsie')
      open.onsuccess = () => resolve(open.result)
      open.onerror = () => reject(open.error)
    })
    const tables = [...db.objectStoreNames]
    const read = db.transaction(tables)
    const counts = await Promise.all(
      tables.map(
        (table) =>
          new Promise<[string, number]>((resolve, reject) => {
            const count = read.objectStore(table).count()
            count.onsuccess = () => resolve([table, count.result])
            count.onerror = () => reject(count.error)
          }),
      ),
    )
    db.close()
    return Object.fromEntries(counts)
  })
}
