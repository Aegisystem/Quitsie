import { useSyncExternalStore } from 'react'

// A newer version of the app waits in the service worker until the User asks
// for it: switching on its own would reload the page and could throw away a
// form half filled in. ../pwa.ts offers it here and the banner in Shell shows it.

type Apply = () => Promise<void>

let pending: Apply | undefined
const listeners = new Set<() => void>()

export function offerUpdate(apply: Apply) {
  pending = apply
  for (const listener of listeners) listener()
}

export function updateAvailable(): boolean {
  return pending !== undefined
}

export function subscribeToUpdate(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

// Activates the new version, which reloads the page.
export async function applyUpdate(): Promise<void> {
  await pending?.()
}

export function useUpdateAvailable(): boolean {
  return useSyncExternalStore(subscribeToUpdate, updateAvailable)
}
