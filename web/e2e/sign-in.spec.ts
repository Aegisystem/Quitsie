import { randomUUID } from 'node:crypto'
import { expect, loginCode, test } from './support'

test('whoever cannot get another code still enters with the one they have', async ({ newDevice }) => {
  const page = await newDevice()
  const email = `ana-${randomUUID()}@quitsie.test`
  const ask = page.getByRole('button', { name: 'Enviarme un código' })
  const code = page.getByLabel('Código de 6 dígitos')

  // An IP gets ten codes per hour.
  await page.getByLabel('Tu email').fill(email)
  for (let sent = 1; sent <= 10; sent++) {
    await ask.click()
    await expect(page.getByText(`Te enviamos un código de 6 dígitos a ${email}.`)).toBeVisible()
    if (sent < 10) await page.getByRole('button', { name: 'Usar otro email' }).click()
  }
  const last = loginCode(email)
  await page.getByRole('button', { name: 'Usar otro email' }).click()

  // The eleventh sends nothing, and the screen goes on to the code anyway.
  await ask.click()
  await expect(page.getByText(`No enviamos otro código a ${email}.`)).toBeVisible()
  await expect(page.getByText('Pediste demasiados códigos. Vuelve a intentarlo en 60 minutos.')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Reenviar en 60 min' })).toBeDisabled()
  expect(loginCode(email)).toBe(last)

  await code.fill(last)
  await expect(page.getByLabel('Tu nombre')).toBeVisible()
})
