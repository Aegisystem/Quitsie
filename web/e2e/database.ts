import { randomBytes } from 'node:crypto'
import pg from 'pg'

// The PostgreSQL of compose.yaml (docker compose up -d), or the one
// QUITSIE_TEST_DATABASE_URL names, as for the Go tests.
const server = process.env.QUITSIE_TEST_DATABASE_URL ?? 'postgres://quitsie:quitsie@127.0.0.1:5433/quitsie?sslmode=disable'

// databaseURL is server pointing at the database name.
export function databaseURL(name: string): string {
  const url = new URL(server)
  url.pathname = `/${name}`
  return url.toString()
}

async function admin<T>(fn: (client: pg.Client) => Promise<T>): Promise<T> {
  const client = new pg.Client({ connectionString: server })
  await client.connect()
  try {
    return await fn(client)
  } finally {
    await client.end()
  }
}

// createDatabase makes an empty database for one run; the server migrates it.
export async function createDatabase(): Promise<string> {
  const name = `quitsie_e2e_${randomBytes(8).toString('hex')}`
  await admin((client) => client.query(`CREATE DATABASE ${name}`))
  return name
}

export async function dropDatabase(name: string): Promise<void> {
  await admin((client) => client.query(`DROP DATABASE IF EXISTS ${name} WITH (FORCE)`))
}
