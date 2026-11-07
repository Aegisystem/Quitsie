import { registerSW } from 'virtual:pwa-register'
import { offerUpdate } from './ui/update'

// How often an app left open asks the server for a newer version.
const hourly = 60 * 60 * 1000

// Registers the service worker and looks for a newer version when the app
// opens, whenever it comes back to the screen, and every hour. An installed PWA
// is usually resumed rather than opened again, so the browser's own check,
// which only runs on navigation, would leave it on an old version for days.
export function startServiceWorker() {
  followNotificationTaps()
  const activate = registerSW({
    immediate: true,
    onNeedRefresh() {
      offerUpdate(async () => {
        // workbox-window only reloads for a version it found in its first
        // minute; one found later, when the app comes back to the screen, would
        // leave the page on the old code. So the reload is ours, with a fallback
        // in case the new version never takes over.
        navigator.serviceWorker.addEventListener('controllerchange', () => window.location.reload(), { once: true })
        setTimeout(() => window.location.reload(), 5000)
        await activate(false)
      })
    },
    onRegisteredSW(_url, registration) {
      if (!registration) return
      const check = () => {
        if (!navigator.onLine || registration.installing) return
        // Offline or with the server down, the next check tries again.
        registration.update().catch(() => {})
      }
      document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible') check()
      })
      setInterval(check, hourly)
    },
  })
}

// A tap on a notification, with the app open, opens its ledger here (src/sw.ts).
function followNotificationTaps() {
  if (!('serviceWorker' in navigator)) return
  navigator.serviceWorker.addEventListener('message', (event) => {
    if (event.data?.type !== 'open' || typeof event.data.url !== 'string') return
    const url = new URL(event.data.url, window.location.href)
    if (url.origin === window.location.origin) window.location.hash = url.hash
  })
}
