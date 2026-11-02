import { createGroup, expect, inviteLink, signUp, test } from './support'

test('whoever was removed cannot come back through the Invite link until a Member readmits them', async ({ newDevice }) => {
  const ana = await newDevice()
  await signUp(ana, 'Ana')
  await createGroup(ana, 'La casa')
  const link = await inviteLink(ana)

  const beto = await newDevice(link)
  await signUp(beto, 'Beto')
  await beto.getByRole('button', { name: 'Unirme al grupo' }).click()
  await expect(beto.getByRole('heading', { name: 'La casa', level: 1 })).toBeVisible()

  // Opening the app again syncs, which brings Beto to Ana's device.
  await ana.goBack()
  await ana.reload()
  await ana.getByRole('link', { name: 'Ajustes' }).click()
  await ana.getByRole('button', { name: 'Sacar a Beto del grupo' }).click()
  await expect(ana.getByText('hasta que alguien del grupo lo readmita')).toBeVisible()
  await ana.getByRole('button', { name: 'Sacar del grupo', exact: true }).click()
  const removed = ana.getByRole('region', { name: 'Sacados del grupo' })
  await expect(removed).toContainText('Beto')

  // The link he kept in his chat no longer lets him in.
  await beto.goto(link)
  await expect(beto.getByText('Te sacaron de este grupo.')).toBeVisible()
  await expect(beto.getByRole('button', { name: 'Unirme al grupo' })).toBeHidden()

  await removed.getByRole('button', { name: 'Readmitir a Beto' }).click()
  await removed.getByRole('button', { name: 'Readmitir', exact: true }).click()
  await expect(ana.getByText('Beto ya puede volver con el enlace de invitación.')).toBeVisible()
  await expect(removed).toBeHidden()

  // Readmitting does not put him back: he comes back himself, with the same link.
  await beto.reload()
  await beto.getByRole('button', { name: 'Unirme al grupo' }).click()
  await expect(beto.getByRole('heading', { name: 'La casa', level: 1 })).toBeVisible()

  await ana.goBack()
  await ana.getByRole('navigation', { name: 'Secciones del grupo' }).getByRole('link', { name: 'Actividad' }).click()
  await expect(ana.getByText('Readmitiste a Beto')).toBeVisible()
})
