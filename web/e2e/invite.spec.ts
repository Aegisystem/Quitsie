import { addExpense, balance, createGroup, expect, expenseRow, inviteLink, serverChanges, signUp, test } from './support'

test('a second User claims a pending Member from the Invite link and inherits its history', async ({ newDevice }) => {
  const ana = await newDevice()
  await signUp(ana, 'Ana')
  await createGroup(ana, 'La casa', { pending: ['Beto'] })
  await addExpense(ana, { description: 'Mercado', amount: '100' })
  await expect(balance(ana)).toContainText('$50')
  const link = await inviteLink(ana)

  // Opened signed out, so the invitation has to survive the sign-up.
  const beto = await newDevice(link)
  await signUp(beto, 'Beto')
  await beto.getByRole('radio', { name: /^Beto/ }).check()
  await beto.getByRole('button', { name: 'Reclamar a Beto' }).click()

  await expect(beto.getByRole('heading', { name: 'La casa' })).toBeVisible()
  await expect(balance(beto)).toContainText('Debes')
  await expect(balance(beto)).toContainText('$50')
  await expect(expenseRow(beto, 'Mercado')).toContainText('Pagó Ana')
  await expect(expenseRow(beto, 'Mercado')).toContainText('te toca $50')

  await ana.goto('/#/')
  await ana.reload()
  await ana.getByRole('link', { name: /La casa/ }).click()
  await ana.getByRole('link', { name: 'Saldos' }).click()
  await expect(ana.getByText('Beto')).toBeVisible()
  await expect(ana.getByText('pendiente', { exact: true })).toBeHidden()
  await expect(balance(ana)).toContainText('$50')

  const { members } = await serverChanges(ana)
  expect(members.find(({ name }) => name === 'Beto')?.userId).toBeTruthy()
})
