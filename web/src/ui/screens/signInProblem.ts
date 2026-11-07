import type { TFunction } from 'i18next'
import type { CallError } from '../../session'

const HOUR = 3600

// What the sign-in screen says when asking for a code, or checking one, fails
// for a reason other than the code itself.
export function signInProblemText(t: TFunction, error: CallError): string {
  if (error.kind === 'offline') return t('signIn.offline')
  const wait = error.retryAfter
  if (error.code === 'rate_limited') {
    if (!wait) return t('signIn.rateLimited')
    if (wait > HOUR) return t('signIn.rateLimitedForHours', { count: Math.ceil(wait / HOUR) })
    // An email waits a minute between codes.
    if (wait < 60) return t('signIn.rateLimitedForSeconds', { count: wait })
    return t('signIn.rateLimitedFor', { count: Math.ceil(wait / 60) })
  }
  // Nobody gets a code until then, so it is not the person's doing.
  if (error.code === 'mail_budget_spent') {
    if (!wait) return t('signIn.mailBudgetSpent')
    if (wait > HOUR) return t('signIn.mailBudgetSpentForHours', { count: Math.ceil(wait / HOUR) })
    return t('signIn.mailBudgetSpentFor', { count: Math.ceil(wait / 60) })
  }
  if (error.code === 'validation_failed') return t('signIn.invalidEmail')
  return t('signIn.failed')
}
