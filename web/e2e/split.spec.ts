import { type Page } from '@playwright/test'
import { createGroup, expect, expenseRow, openEditForm, serverChanges, signUp, test } from './support'

// The split of an Expense opens in screens of its own, over the form (DESIGN.md, Reparto).
const paidLine = (page: Page) => page.getByRole('link', { name: /^Quién pagó:/ })
const splitLine = (page: Page) => page.getByRole('link', { name: /^Cómo se divide:/ })

async function newExpense(page: Page, { description, amount }: { description: string; amount: string }) {
  await page.getByRole('link', { name: 'Añadir gasto' }).click()
  await page.getByLabel('Importe').fill(amount)
  await page.getByLabel('Descripción').fill(description)
}

// What the server keeps of an Expense, with the names of its Participants.
async function saved(page: Page, description: string) {
  let shares: { paid: Record<string, number>; owed: Record<string, number>; method: string } | undefined
  await expect
    .poll(async () => {
      const { expenses, members } = await serverChanges(page)
      const expense = expenses.find((e) => e.description === description)
      if (!expense) return undefined
      const name = (id: string) => members.find((m) => m.id === id)?.name ?? id
      shares = {
        paid: Object.fromEntries(expense.paidShares.map((s) => [name(s.memberId), s.amount])),
        owed: Object.fromEntries(expense.owedShares.map((s) => [name(s.memberId), s.amount])),
        method: expense.splitMethod,
      }
      return shares
    })
    .toBeDefined()
  return shares
}

test('who paid and how it is divided open over the form, and going back keeps everything', async ({ newDevice }) => {
  const ana = await newDevice()
  await signUp(ana, 'Ana')
  await createGroup(ana, 'Viaje', { pending: ['Beto', 'Carla', 'Dani'] })
  await newExpense(ana, { description: 'Cena', amount: '120' })

  // Most Expenses keep the default: paid by the User, in equal parts.
  await expect(paidLine(ana)).toContainText('Pagaste tú')
  await expect(splitLine(ana)).toContainText('Partes iguales entre los 4')
  await expect(splitLine(ana)).toContainText('$30.00 cada uno')

  // Choosing who paid goes back to the form at once.
  await paidLine(ana).click()
  await ana.getByRole('button', { name: 'Beto', exact: true }).click()
  await expect(paidLine(ana)).toContainText('Pagó Beto')
  await expect(ana.getByLabel('Descripción')).toHaveValue('Cena')

  await splitLine(ana).click()
  await ana.getByRole('checkbox', { name: 'Incluir a Dani' }).uncheck()
  await expect(ana.getByLabel('Cuota de Tú')).toHaveText('$40.00')
  await expect(ana.getByText('Cuadra con el total de $120.00')).toBeVisible()

  // A new method starts from the split on screen, with the same people.
  await ana.getByRole('radio', { name: 'Montos exactos' }).check()
  await expect(ana.getByLabel('Valor de Tú')).toHaveValue('40.00')
  await expect(ana.getByLabel('Valor de Dani')).toHaveValue('')
  await ana.getByLabel('Valor de Tú').fill('55')
  await expect(ana.getByText('Te pasas por $15.00')).toBeVisible()

  // The browser's back button returns to the form, which says what to fix.
  await ana.goBack()
  await expect(ana.getByLabel('Descripción')).toHaveValue('Cena')
  await expect(splitLine(ana)).toContainText('Montos exactos entre 3 de 4')
  await expect(splitLine(ana)).toContainText('Te pasas por $15.00')
  // The button stays tappable while something blocks it, to say what (aria-disabled).
  await ana.getByRole('button', { name: /^Guardar/ }).click({ force: true })
  await expect(ana.getByRole('paragraph').filter({ hasText: /^Te pasas por \$15\.00$/ })).toBeVisible()
  await expect(ana.getByLabel('Descripción')).toHaveValue('Cena')

  await splitLine(ana).click()
  await ana.getByRole('radio', { name: 'Proporciones' }).check()
  await expect(ana.getByLabel('Valor de Tú')).toHaveValue('1')
  await ana.getByRole('button', { name: 'Una parte más para Beto' }).click()
  await expect(ana.getByLabel('Cuota de Beto')).toHaveText('$60.00')
  await ana.getByRole('button', { name: 'Listo' }).click()
  await expect(splitLine(ana)).toContainText('Proporciones entre 3 de 4')
  await expect(splitLine(ana)).toContainText('Tú $30.00 · Beto $60.00 · Carla $30.00')

  await ana.getByRole('button', { name: /^Guardar/ }).click()
  await expect(expenseRow(ana, 'Cena')).toBeVisible()

  // Editing brings the split back as it was saved.
  await openEditForm(ana, 'Cena')
  await expect(paidLine(ana)).toContainText('Pagó Beto')
  await splitLine(ana).click()
  await expect(ana.getByRole('radio', { name: 'Proporciones' })).toBeChecked()
  await expect(ana.getByLabel('Valor de Beto')).toHaveValue('2')

  // Asked last: a pull of its own would make the app's wait its turn.
  expect(await saved(ana, 'Cena')).toEqual({ paid: { Beto: 12000 }, owed: { Ana: 3000, Beto: 6000, Carla: 3000 }, method: 'weights' })
})

test('several people can pay, each their own amount', async ({ newDevice }) => {
  const ana = await newDevice()
  await signUp(ana, 'Ana')
  await createGroup(ana, 'Casa', { pending: ['Beto'] })
  await newExpense(ana, { description: 'Mercado', amount: '90' })

  await paidLine(ana).click()
  await ana.getByRole('button', { name: /Pagaron varios/ }).click()
  await ana.getByLabel('Lo que pagaste').fill('50')
  await expect(ana.getByText('Faltan $40.00 por cubrir entre quienes pagaron')).toBeVisible()
  await ana.getByLabel('Lo que pagó Beto').fill('40')
  await expect(ana.getByText('Lo pagado suma el total')).toBeVisible()
  await ana.getByRole('button', { name: 'Listo' }).click()

  await expect(paidLine(ana)).toContainText('Pagaron 2 personas')
  await expect(paidLine(ana)).toContainText('Tú $50.00 · Beto $40.00')
  await expect(splitLine(ana)).toContainText('$45.00 cada uno')
  await ana.getByRole('button', { name: /^Guardar/ }).click()
  await expect(expenseRow(ana, 'Mercado')).toBeVisible()
  expect(await saved(ana, 'Mercado')).toEqual({ paid: { Ana: 5000, Beto: 4000 }, owed: { Ana: 4500, Beto: 4500 }, method: 'equal' })
})
