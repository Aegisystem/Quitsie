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

// Each run gets its own port and database. Playwright loads this file again in
// every worker, which inherit these variables and so agree with the first load.
process.env.QUITSIE_E2E_PORT ??= String(await freePort())
process.env.QUITSIE_E2E_DIR ??= mkdtempSync(join(tmpdir(), 'quitsie-e2e-'))
process.env.QUITSIE_E2E_DATABASE ??= await createDatabase()
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
    },
  },
})
