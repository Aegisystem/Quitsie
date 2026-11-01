import { addExpense, createGroup, deviceRows, expect, serverChanges, setOffline, signIn, signUp, test } from './support'

test('signing out warns about an UnsyncedRecord, lets the User cancel, and empties the device', async ({ newDevice }) => {
  const page = await newDevice()
  const ana = await signUp(page, 'Ana')
  await createGroup(page, 'La casa', { pending: ['Beto'] })

  // Signing out needs the connection, and with it the queue would empty by
  // itself. So the connection comes back but the record still does not get through.
  await setOffline(page, true)
  await addExpense(page, { description: 'Cena', amount: '100' })
  const noRecords = '**/api/v1/records'
  await page.context().route(noRecords, (route) => route.abort())
  await setOffline(page, false)

  await page.goto('/#/account')
  const signOut = page.getByRole('button', { name: 'Cerrar sesión', exact: true })
  const warning = page.getByRole('alertdialog')
  await signOut.click()
  await expect(warning).toContainText('En este dispositivo hay 1 registro sin sincronizar.')

  await warning.getByRole('button', { name: 'Cancelar' }).click()
  await expect(warning).toBeHidden()
  expect((await deviceRows(page)).queue).toBe(1)

  await signOut.click()
  await warning.getByRole('button', { name: 'Cerrar sesión y perderlo' }).click()
  await expect(page.getByRole('heading', { name: 'Entrar' })).toBeVisible()

  const rows = await deviceRows(page)
  expect(Object.keys(rows)).toContain('queue')
  expect(Object.values(rows).every((count) => count === 0)).toBe(true)
  expect(await page.evaluate(() => Object.keys(localStorage).filter((key) => key.startsWith('quitsie.')))).toEqual([])
  expect(await page.context().cookies()).toEqual([])

  // Signing in again brings back what the server has, and only that.
  await page.context().unroute(noRecords)
  await signIn(page, ana)
  await page.getByRole('link', { name: /La casa/ }).click()
  await expect(page.getByText('Ningún gasto todavía.')).toBeVisible()
  expect((await serverChanges(page)).expenses).toEqual([])
})
