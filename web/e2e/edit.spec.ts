import { addExpense, createGroup, expect, expenseRow, inviteLink, openEditForm, serverChanges, signUp, test, unsyncedMark } from './support'

test('whoever is editing is warned as soon as someone else saves, and nothing is overwritten', async ({ newDevice }) => {
  const ana = await newDevice()
  await signUp(ana, 'Ana')
  await createGroup(ana, 'La casa')
  const link = await inviteLink(ana)

  const beto = await newDevice(link)
  await signUp(beto, 'Beto')
  await beto.getByRole('button', { name: 'Unirme al grupo' }).click()
  await expect(beto.getByRole('heading', { name: 'La casa' })).toBeVisible()

  // Ana's open device hears that Beto joined (ADR 0013).
  await ana.getByRole('link', { name: 'Volver' }).click()
  await addExpense(ana, { description: 'Cena', amount: '100' })
  await expect(unsyncedMark(expenseRow(ana, 'Cena'))).toBeHidden()

  await openEditForm(ana, 'Cena')
  await openEditForm(beto, 'Cena')

  await ana.getByLabel('Importe').fill('120')
  await ana.getByRole('button', { name: 'Guardar cambios' }).click()
  await expect(ana.getByText('Gasto actualizado.')).toBeVisible()
  await expect(expenseRow(ana, 'Cena')).toContainText('$120')

  // Beto's open form hears about Ana's edit before Beto saves anything.
  await beto.getByLabel('Descripción').fill('Almuerzo')
  const conflict = beto.getByRole('alert').filter({ hasText: 'Este gasto cambió mientras lo editabas' })
  await expect(conflict).toContainText('Tus cambios no se guardaron')
  await expect(conflict).toContainText('Cena · $120.00')
  expect((await serverChanges(beto)).expenses).toMatchObject([{ description: 'Cena', total: 12000 }])

  await beto.getByRole('button', { name: 'Usar la versión nueva' }).click()
  await expect(conflict).toBeHidden()
  await expect(beto.getByLabel('Descripción')).toHaveValue('Cena')
  await expect(beto.getByLabel('Importe')).toHaveValue('120.00')

  await beto.getByLabel('Descripción').fill('Cena con postre')
  await beto.getByRole('button', { name: 'Guardar cambios' }).click()
  await expect(beto.getByText('Gasto actualizado.')).toBeVisible()
  await expect(expenseRow(beto, 'Cena con postre')).toContainText('$120')
  expect((await serverChanges(ana)).expenses).toMatchObject([{ description: 'Cena con postre', total: 12000 }])

  await ana.reload()
  await expect(expenseRow(ana, 'Cena con postre')).toBeVisible()
})
