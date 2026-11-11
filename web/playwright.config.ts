import { generateKeyPairSync, randomBytes } from 'node:crypto'
import { mkdtempSync } from 'node:fs'
import { createServer } from 'node:net'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig, devices } from '@playwright/test'
import { createDatabase, databaseURL } from './e2e/database'

function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const server = createServer()
    server.once('error', reject)
    server.listen(0, '127.0.0.1', () => {
      const address = server.address()
      server.close(() => (address && typeof address === 'object' ? resolve(address.port) : reject(new Error('no port'))))
    })
  })
}

// A VAPID key pair for this run, so the server sends push notifications.
function vapidKeys(): { publicKey: string; privateKey: string } {
  const { publicKey, privateKey } = generateKeyPairSync('ec', { namedCurve: 'prime256v1' })
  const pub = publicKey.export({ format: 'jwk' })
  const raw = Buffer.concat([Buffer.from([4]), Buffer.from(pub.x ?? '', 'base64url'), Buffer.from(pub.y ?? '', 'base64url')])
  return { publicKey: raw.toString('base64url'), privateKey: privateKey.export({ format: 'jwk' }).d ?? '' }
}

// Each run gets its own port and database. Playwright loads this file again in
// every worker, which inherit these variables and so agree with the first load.
process.env.QUITSIE_E2E_PORT ??= String(await freePort())
process.env.QUITSIE_E2E_DIR ??= mkdtempSync(join(tmpdir(), 'quitsie-e2e-'))
process.env.QUITSIE_E2E_DATABASE ??= await createDatabase()
if (!process.env.QUITSIE_E2E_VAPID_PUBLIC_KEY) {
  const keys = vapidKeys()
  process.env.QUITSIE_E2E_VAPID_PUBLIC_KEY = keys.publicKey
  process.env.QUITSIE_E2E_VAPID_PRIVATE_KEY = keys.privateKey
}
// What Cloudflare adds in production to every request it forwards; with it the
// server believes the address each device says it comes from (e2e/support.ts).
process.env.QUITSIE_E2E_CLOUDFLARE_SECRET ??= randomBytes(32).toString('hex')
const port = process.env.QUITSIE_E2E_PORT
const dir = process.env.QUITSIE_E2E_DIR

// A *.localhost host of its own, because cookies ignore the port: on plain
// localhost, any other Quitsie server would sign the test Users out. Browsers
// treat it as a secure context, so the service worker and the Secure cookie work.
const origin = `http://quitsie-e2e.localhost:${port}`

// The binary with the PWA embedded, as CI builds it: `npm run e2e` rebuilds both.
const binary = fileURLToPath(new URL('../api/bin/quitsie', import.meta.url))

export default defineConfig({
  testDir: 'e2e',
  outputDir: 'e2e/results',
  globalTeardown: './e2e/teardown.ts',
  timeout: 30_000,
  expect: { timeout: 10_000 },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['list'], ['github']] : 'list',
  use: {
    ...devices['Pixel 7'],
    baseURL: origin,
    locale: 'es-MX',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  webServer: {
    // The login codes go to the log, where the tests read them (e2e/support.ts).
    command: `exec '${binary}' 2>> '${join(dir, 'server.log')}'`,
    url: `http://127.0.0.1:${port}/api/v1/health`,
    reuseExistingServer: false,
    gracefulShutdown: { signal: 'SIGTERM', timeout: 2_000 },
    env: {
      QUITSIE_ENV: 'development',
      QUITSIE_MAIL: 'log',
      QUITSIE_ADDR: `127.0.0.1:${port}`,
      QUITSIE_PUBLIC_ORIGIN: origin,
      QUITSIE_DATABASE_URL: databaseURL(process.env.QUITSIE_E2E_DATABASE),
      // The tests sign one User in on two devices within seconds.
      QUITSIE_LOGIN_CODE_COOLDOWN: '1ms',
      QUITSIE_CLOUDFLARE_SECRET: process.env.QUITSIE_E2E_CLOUDFLARE_SECRET,
      QUITSIE_VAPID_PUBLIC_KEY: process.env.QUITSIE_E2E_VAPID_PUBLIC_KEY ?? '',
      QUITSIE_VAPID_PRIVATE_KEY: process.env.QUITSIE_E2E_VAPID_PRIVATE_KEY ?? '',
    },
  },
})
