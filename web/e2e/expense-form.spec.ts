import { createGroup, expect, expenseRow, serverChanges, signUp, test } from './support'

test('amounts show thousands separators as they are typed, and the description suggests a Category', async ({ newDevice }) => {
  const ana = await newDevice()
  await signUp(ana, 'Ana')
  await createGroup(ana, 'Viaje', { pending: ['Beto'] })

  await ana.getByRole('link', { name: 'Añadir gasto' }).click()
  const amount = ana.getByLabel('Importe')
  // The browser runs in es-MX: commas group thousands and a point starts the decimals.
  await amount.pressSequentially('1234567.5')
  await expect(amount).toHaveValue('1,234,567.5')

  const suggested = ana.getByText('Sugerida por la descripción')
  await ana.getByLabel('Descripción').fill('Uber al aeropuerto')
  await expect(ana.getByRole('radio', { name: 'Transporte' })).toBeChecked()
  await expect(suggested).toBeVisible()

  // A Category picked by hand stays, whatever the description says next.
  await ana.getByRole('radio', { name: 'Ocio' }).check()
  await expect(suggested).toBeHidden()
  await ana.getByLabel('Descripción').fill('Uber al hotel')
  await expect(ana.getByRole('radio', { name: 'Ocio' })).toBeChecked()

  await ana.getByRole('button', { name: /^Guardar/ }).click()
  await expect(expenseRow(ana, 'Uber al hotel')).toBeVisible()
  await expect
    .poll(async () => (await serverChanges(ana)).expenses)
    .toMatchObject([{ description: 'Uber al hotel', total: 123456750, category: 'entertainment' }])

  // The User's own choice for the same description wins over the keywords.
  await ana.getByRole('link', { name: 'Añadir gasto' }).click()
  await ana.getByLabel('Descripción').fill('uber al hotel')
  await expect(ana.getByRole('radio', { name: 'Ocio' })).toBeChecked()
  await expect(suggested).toBeVisible()
})
