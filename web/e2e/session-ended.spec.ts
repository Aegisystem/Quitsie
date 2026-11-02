import { addExpense, createGroup, deviceRows, expect, expenseRow, serverChanges, setOffline, signIn, signUp, test } from './support'

test('when the session ends by itself the replica goes, and the queue waits hidden for its owner', async ({ newDevice }) => {
  const phone = await newDevice()
  const ana = await signUp(phone, 'Ana')
  await createGroup(phone, 'La casa', { pending: ['Beto'] })
  await setOffline(phone, true)
  await addExpense(phone, { description: 'Cena', amount: '100' })

  // Ana lost the phone, and closes its session from her laptop.
  const laptop = await newDevice()
  await signIn(laptop, ana)
  await laptop.goto('/#/account')
  const closePhone = laptop.getByRole('button', { name: /^Cerrar la sesión de/ })
  await closePhone.click()
  await expect(closePhone).toBeHidden()

  // The phone finds out as soon as it reconnects, and keeps only the queue.
  await setOffline(phone, false)
  await expect(phone.getByText('Tu sesión se cerró. Vuelve a entrar.')).toBeVisible()
  await expect.poll(() => deviceRows(phone)).toMatchObject({ ledgers: 0, members: 0, expenses: 0, activities: 0, debts: 0, meta: 0, queue: 1 })

  // Whoever signs in next on the phone finds nothing of Ana's, and sends nothing of hers.
  await signUp(phone, 'Beto')
  await expect(phone.getByText('La casa')).toBeHidden()
  expect(await deviceRows(phone)).toMatchObject({ queue: 0, parked: 1 })
  await phone.goto('/#/account')
  await phone.getByRole('button', { name: 'Cerrar sesión', exact: true }).click()
  await expect(phone.getByRole('heading', { name: 'Entrar' })).toBeVisible()
  expect((await serverChanges(laptop)).expenses).toEqual([])
  expect(await deviceRows(phone)).toMatchObject({ parked: 1 })

  // Back on her phone, Ana's record goes to the server.
  await signIn(phone, ana)
  await phone.getByRole('link', { name: /La casa/ }).click()
  await expect(expenseRow(phone, 'Cena')).toBeVisible()
  await expect.poll(async () => (await serverChanges(laptop)).expenses.map(({ description }) => description)).toEqual(['Cena'])
  await expect.poll(() => deviceRows(phone)).toMatchObject({ queue: 0, parked: 0 })
})
