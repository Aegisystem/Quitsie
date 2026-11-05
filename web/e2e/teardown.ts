import { rmSync } from 'node:fs'
import { dropDatabase } from './database'

// The database and the log of this run. A failed test keeps its copy of the log.
export default async function teardown() {
  const dir = process.env.QUITSIE_E2E_DIR
  if (dir) rmSync(dir, { recursive: true, force: true })
  const database = process.env.QUITSIE_E2E_DATABASE
  if (database) await dropDatabase(database)
}
