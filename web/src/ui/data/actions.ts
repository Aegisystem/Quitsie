import type { Api, SyncEngine } from '../../sync'

// The operations of the read screens that go straight to the API, online only
// (ADR 0001). None of them assumes it worked: the replica only changes through
// the sync that follows a yes from the server.

export type RecordKind = 'expense' | 'settlement'

// `formerMember`: a record with a former member can be neither deleted nor
// restored, as that would change their Balance, which is zero.
// `balance`: deleting or restoring it would take a Balance past what Quitsie
// can hold (docs/reglas-de-negocio.md).
// `gone`: the server no longer has it, or the User no longer sees it.
// `offline`: no answer came back at all.
export type ActionResult = 'done' | 'formerMember' | 'balance' | 'gone' | 'offline' | 'failed'

interface Answer {
  error?: unknown
  response: Response
}

const codeOf = (error: unknown) => (typeof error === 'object' && error !== null && 'code' in error ? error.code : undefined)

export function createActions({ api, engine }: { api: Api; engine: Pick<SyncEngine, 'sync'> }) {
  async function run(send: () => Promise<Answer>): Promise<ActionResult> {
    let answer: Answer
    try {
      answer = await send()
    } catch {
      return 'offline'
    }
    const { error, response } = answer
    if (response.status === 409 && codeOf(error) === 'invalid_participants') return 'formerMember'
    if (response.status === 400 && codeOf(error) === 'invalid_amounts') return 'balance'
    if (!response.ok && response.status !== 404) return 'failed'
    await engine.sync()
    return response.ok ? 'done' : 'gone'
  }

  return {
    deleteRecord: (kind: RecordKind, id: string) =>
      run(() =>
        kind === 'expense'
          ? api.DELETE('/expenses/{expenseId}', { params: { path: { expenseId: id } } })
          : api.DELETE('/settlements/{settlementId}', { params: { path: { settlementId: id } } }),
      ),

    restoreRecord: (kind: RecordKind, id: string) =>
      run(() =>
        kind === 'expense'
          ? api.POST('/expenses/{expenseId}/restore', { params: { path: { expenseId: id } } })
          : api.POST('/settlements/{settlementId}/restore', { params: { path: { settlementId: id } } }),
      ),
  }
}
