import { rmSync } from 'node:fs'

// The database and the log of this run. A failed test keeps its copy of the log.
export default function teardown() {
  const dir = process.env.QUITSIE_E2E_DIR
  if (dir) rmSync(dir, { recursive: true, force: true })
}
