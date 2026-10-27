// Mistakes the Usuario can make in the form come back as a Result, so the
// interface can explain them. Mistakes in the calling code throw.
export type Result<T, E> = { ok: true; value: T } | { ok: false; error: E }

export function ok<T>(value: T): { ok: true; value: T } {
  return { ok: true, value }
}

export function fail<E>(error: E): { ok: false; error: E } {
  return { ok: false, error }
}
