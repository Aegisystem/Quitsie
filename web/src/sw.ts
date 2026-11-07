/// <reference lib="webworker" />
import { clientsClaim } from 'workbox-core'
import { cleanupOutdatedCaches, createHandlerBoundToURL, precacheAndRoute } from 'workbox-precaching'
import { NavigationRoute, registerRoute } from 'workbox-routing'
import { fallbackNotice, ledgerPath, noticeFor, readNoticeData, readPayload, tagOf } from './push/notification'

// The service worker of the PWA. It precaches the interface, as the one
// vite-plugin-pwa generated before, and shows the push notifications of new
// Expenses and Settlements (ADR 0013).

declare const self: ServiceWorkerGlobalScope

// Only the interface: data lives in IndexedDB, and /api must always reach the
// server instead of getting index.html.
precacheAndRoute(self.__WB_MANIFEST)
cleanupOutdatedCaches()
registerRoute(new NavigationRoute(createHandlerBoundToURL('index.html'), { denylist: [/^\/api\//] }))

// The first install takes over at once; a new version waits until the User
// taps «Actualizar» (src/pwa.ts), which sends SKIP_WAITING.
clientsClaim()
self.addEventListener('message', (event) => {
  if (event.data?.type === 'SKIP_WAITING') void self.skipWaiting()
})

// Every push must show a notification: browsers revoke the subscription of a
// worker that receives one and shows nothing.
self.addEventListener('push', (event) => {
  event.waitUntil(show(event.data))
})

async function show(data: PushMessageData | null) {
  let payload: unknown
  try {
    payload = data?.json()
  } catch {
    payload = undefined
  }
  const read = readPayload(payload)
  if (!read) {
    await self.registration.showNotification(fallbackNotice.title, { body: fallbackNotice.body, icon: '/pwa-192x192.png' })
    return
  }
  // The notification of the ledger still on screen takes the new records.
  const shown = await self.registration.getNotifications({ tag: tagOf(read.ledgerId) }).catch(() => [])
  const notice = noticeFor(read, readNoticeData(shown[0]?.data), self.navigator.language || 'es')
  await self.registration.showNotification(notice.title, {
    body: notice.body,
    tag: notice.tag,
    data: notice.data,
    lang: 'es',
    icon: '/pwa-192x192.png',
    // The updated notification alerts again: it has new records.
    renotify: true,
  } as NotificationOptions)
}

// Tapping a notification opens its ledger, in the app if it is open.
self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const notice = readNoticeData(event.notification.data)
  const url = new URL(notice ? ledgerPath(notice.ledgerId) : '/', self.location.origin).href
  event.waitUntil(open(url))
})

async function open(url: string) {
  const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
  const app = windows.find((w) => new URL(w.url).origin === self.location.origin)
  if (!app) {
    await self.clients.openWindow(url)
    return
  }
  // The app changes its route itself (src/pwa.ts), without reloading.
  app.postMessage({ type: 'open', url })
  await app.focus()
}
