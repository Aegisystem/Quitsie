import { addExpense, createGroup, expect, expenseRow, inviteLink, setOffline, signUp, test } from './support'

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

test('a pull left without an answer, as when a phone loses the network, does not stop later updates', async ({ newDevice }) => {
  const ana = await newDevice()
  await signUp(ana, 'Ana')
  await createGroup(ana, 'El viaje')
  const link = await inviteLink(ana)

  const beto = await newDevice(link)
  await signUp(beto, 'Beto')
  await beto.getByRole('button', { name: 'Unirme al grupo' }).click()
  await expect(beto.getByRole('heading', { name: 'El viaje' })).toBeVisible()
  await ana.getByRole('link', { name: 'Volver' }).click()

  // The next pull of Beto's device never gets an answer.
  let hang = true
  const held = new Promise<void>((resolve) => {
    void beto.route('**/api/v1/sync**', (route) => {
      if (!hang) return route.continue()
      hang = false
      resolve()
    })
  })
  await addExpense(ana, { description: 'Hotel', amount: '120' })
  await held

  // The network comes back, and the open app gets what it missed.
  await setOffline(beto, true)
  await setOffline(beto, false)
  await addExpense(ana, { description: 'Museo', amount: '20' })
  await expect(expenseRow(beto, 'Hotel')).toBeVisible({ timeout: 10_000 })
  await expect(expenseRow(beto, 'Museo')).toBeVisible({ timeout: 10_000 })
})
