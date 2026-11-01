import type { LoginMethods } from '../../api/client'
import type { CallResult } from '../../session'

type Answer = CallResult<LoginMethods> | undefined

// Whether the sign-in screen offers Google. Only when the server said it can:
// without its OAuth client, the button would only lead to an error. Until the
// answer comes, or when it cannot come, the code is the way in.
export function offersGoogle(answer: Answer): boolean {
  return answer?.ok === true && answer.value.google
}

// How long an answer holds. The server reads its methods from its
// configuration when it starts, so an answer holds while the app runs. A
// failure is stale at once: the query asks again when the connection comes
// back or the User returns to the app.
export function loginMethodsStaleTime(answer: Answer): number {
  return answer?.ok ? Infinity : 0
}
