import type { Client } from 'openapi-fetch'
import type { components, paths } from '../../api/schema.gen'

export type ExpenseUpdate = components['schemas']['ExpenseUpdate']

// What happened to an edit, in the terms the form explains them.
// `conflict`: someone changed or deleted the Expense first, so nothing was saved.
// `currency`: the Group's currency changed. `participants`: someone left the Group
// or a former member's shares would change. `gone`: the User cannot see it any more.
// `offline`: no answer came back.
export type UpdateOutcome = 'saved' | 'conflict' | 'currency' | 'participants' | 'gone' | 'offline' | 'failed'

// What is left of a failed save once the User edits the form again: what they
// change may be the fix, and the next save says so if it is not. A conflict
// stays, because only loading the new version solves it.
export function afterEdit(outcome: UpdateOutcome | undefined): UpdateOutcome | undefined {
  return outcome === 'conflict' ? outcome : undefined
}

// Editing is online only (ADR 0001): PATCH with the version the User was editing.
export async function updateExpense(client: Client<paths>, expenseId: string, update: ExpenseUpdate): Promise<UpdateOutcome> {
  try {
    const { error, response } = await client.PATCH('/expenses/{expenseId}', { params: { path: { expenseId } }, body: update })
    if (response.ok) return 'saved'
    if (response.status === 404) return 'gone'
    switch (error?.code) {
      case 'version_conflict':
        return 'conflict'
      case 'currency_mismatch':
        return 'currency'
      case 'invalid_participants':
        return 'participants'
      default:
        return 'failed'
    }
  } catch {
    return 'offline'
  }
}
