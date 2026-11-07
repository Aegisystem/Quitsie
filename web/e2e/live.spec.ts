import { addExpense, createGroup, expect, expenseRow, inviteLink, signUp, test } from './support'

test('an open device shows at once what someone else adds to a shared Group', async ({ newDevice }) => {
  const ana = await newDevice()
  await signUp(ana, 'Ana')
  await createGroup(ana, 'La casa')
  const link = await inviteLink(ana)

  const beto = await newDevice(link)
  await signUp(beto, 'Beto')
  await beto.getByRole('button', { name: 'Unirme al grupo' }).click()
  await expect(beto.getByRole('heading', { name: 'La casa' })).toBeVisible()

  // Ana's device hears that Beto joined, without reloading.
  await ana.getByRole('link', { name: 'Volver' }).click()
  await addExpense(ana, { description: 'Mercado', amount: '80' })

  // Beto keeps the Group open and the Expense shows up on its own.
  await expect(expenseRow(beto, 'Mercado')).toBeVisible({ timeout: 10_000 })
})
