import type { TFunction } from 'i18next'
import { useState } from 'react'
import type { ProblemCode } from '../../api/client'
import type { CallError, CallResult } from '../../session'

// Every operation on a Group or a Friendship is online only (ADR 0001), so the
// screens share how one runs and how its refusal reads.

export function problemText(t: TFunction, error: CallError, worded: Partial<Record<ProblemCode, string>> = {}): string {
  if (error.kind === 'offline') return t('common.onlineOnly')
  const text = error.code && worded[error.code]
  if (text) return text
  if (error.code === 'validation_failed') return t('problems.invalid')
  if (error.code === 'rate_limited') return t('problems.rateLimited')
  if (error.status === 404) return t('problems.gone')
  return t('problems.failed')
}

export function useOperation() {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<CallError>()

  async function run<T>(send: () => Promise<CallResult<T>>, done: (value: T) => void, refused?: (error: CallError) => void) {
    setBusy(true)
    setError(undefined)
    const result = await send()
    setBusy(false)
    if (result.ok) {
      done(result.value)
    } else {
      setError(result.error)
      refused?.(result.error)
    }
  }

  return { busy, error, run, reset: () => setError(undefined) }
}
