import { createGroup, expect, inviteLink, serverChanges, signUp, test } from './support'

test('only who added a pending Member’s email sees it, and someone else’s edit keeps it', async ({ newDevice }) => {
  const ana = await newDevice()
  await signUp(ana, 'Ana')
  await createGroup(ana, 'La casa')
  await ana.getByRole('link', { name: 'Ajustes' }).click()
  await ana.getByRole('button', { name: 'Añadir miembro pendiente' }).click()
  await ana.getByLabel('Nombre', { exact: true }).fill('Pepa')
  await ana.getByLabel('Email (opcional)').fill('pepa@quitsie.test')
  await expect(ana.getByText('Solo tú verás este email.')).toBeVisible()
  await ana.getByRole('button', { name: 'Añadir miembro pendiente' }).click()
  await expect(ana.getByText('pepa@quitsie.test')).toBeVisible()
  await ana.goBack()
  const link = await inviteLink(ana)
  await expect(ana.getByText('pepa@quitsie.test')).toBeVisible()

  // Anyone with the link gets in, and must not collect the email.
  const beto = await newDevice(link)
  await signUp(beto, 'Beto')
  await beto.getByRole('radio', { name: /No estoy en la lista/ }).check()
  await beto.getByRole('button', { name: 'Unirme al grupo' }).click()
  await beto.getByRole('link', { name: 'Ajustes' }).click()
  const pepa = beto.getByRole('listitem').filter({ hasText: 'Pepa' })
  await expect(pepa).toContainText('pendiente')
  await expect(beto.getByText('pepa@quitsie.test')).toHaveCount(0)

  // Beto renames her without seeing the email, and the email stays Ana's.
  await pepa.getByRole('button', { name: 'Editar' }).click()
  await expect(beto.getByLabel('Email (opcional)')).toHaveValue('')
  await expect(beto.getByText('Si alguien más le puso un email, solo esa persona lo ve')).toBeVisible()
  await beto.getByLabel('Nombre', { exact: true }).fill('Pepa Ríos')
  await beto.getByRole('button', { name: 'Guardar' }).click()
  await expect(beto.getByText('Pepa Ríos')).toBeVisible()

  const seenBy = async (page: typeof ana) => (await serverChanges(page)).members.find(({ name }) => name === 'Pepa Ríos')
  expect(await seenBy(beto)).not.toHaveProperty('email')
  expect(await seenBy(ana)).toHaveProperty('email', 'pepa@quitsie.test')
})
