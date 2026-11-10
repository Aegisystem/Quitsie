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

// Whether the device's replica has the Expense with a version, which the edit
// form needs. Once the server accepts it, the mark of the queue goes; the
// version only comes with the pull that follows.
function inReplica(page: Page, description: string): Promise<boolean> {
  return page.evaluate(async (description) => {
    const db = await new Promise<IDBDatabase>((resolve, reject) => {
      const open = indexedDB.open('quitsie')
      open.onsuccess = () => resolve(open.result)
      open.onerror = () => reject(open.error)
    })
    const expenses = await new Promise<{ description: string; version?: number }[]>((resolve, reject) => {
      const all = db.transaction('expenses').objectStore('expenses').getAll()
      all.onsuccess = () => resolve(all.result)
      all.onerror = () => reject(all.error)
    })
    db.close()
    return expenses.some((expense) => expense.description === description && expense.version !== undefined)
  }, description)
}

export async function openEditForm(page: Page, description: string) {
  await expect.poll(() => inReplica(page, description)).toBe(true)
  await expenseRow(page, description).getByRole('button', { expanded: false }).click()
  await page.getByRole('link', { name: 'Editar gasto' }).click()
  await expect(page.getByLabel('Descripción')).toHaveValue(description)
}

// From the ledger: the address of the Group's Invite link.
export async function inviteLink(page: Page): Promise<string> {
  await page.getByRole('link', { name: 'Invitar' }).click()
  return (await page.locator('.link-box code').textContent()) ?? ''
}

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
  category?: string
  splitMethod: string
  version: number
  deletedAt?: string
  paidShares: { memberId: string; amount: number }[]
  owedShares: { memberId: string; amount: number }[]
}

// What the server holds for this device's User, whether the device is online or not.
interface ServerMember {
  id: string
  name: string
  userId?: string
  email?: string
}

export async function serverChanges(page: Page): Promise<{ expenses: ServerExpense[]; members: ServerMember[] }> {
  const cookies = await page.context().cookies()
  const pull = () =>
    fetch(`${server}/api/v1/sync?since=0`, {
      headers: { cookie: cookies.map(({ name, value }) => `${name}=${value}`).join('; ') },
    })
  // A User gets one pull at a time, and the device may be running its own.
  let response = await pull()
  while (response.status === 429) response = await pull()
  expect(response.status).toBe(200)
  return (await response.json()) as { expenses: ServerExpense[]; members: ServerMember[] }
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
