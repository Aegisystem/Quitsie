import type { Page } from '@playwright/test'
import { expect, signUp, test } from './support'

// Chromium under test has no push service and always denies notifications,
// so the page gets a simulated PushManager and permission. Its subscription
// lives in localStorage, as a browser's outlives a reload, and its keys are
// real ones, which the server checks.
function simulatePushService() {
  Object.defineProperty(Notification, 'permission', { get: () => 'granted' })
  const saved = 'e2e.push'
  const base64 = (bytes: Uint8Array) => btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
  const bytes = (text: string) => Uint8Array.from(atob(text.replace(/-/g, '+').replace(/_/g, '/')), (c) => c.charCodeAt(0))
  type Saved = { endpoint: string; p256dh: string; auth: string; key: string }
  const subscription = (s: Saved) => ({
    endpoint: s.endpoint,
    options: { applicationServerKey: bytes(s.key).buffer },
    toJSON: () => ({ endpoint: s.endpoint, keys: { p256dh: s.p256dh, auth: s.auth } }),
    unsubscribe: async () => {
      localStorage.removeItem(saved)
      return true
    },
  })
  PushManager.prototype.getSubscription = async function () {
    const raw = localStorage.getItem(saved)
    return raw ? (subscription(JSON.parse(raw)) as unknown as PushSubscription) : null
  }
  PushManager.prototype.subscribe = async function (options?: PushSubscriptionOptionsInit) {
    if (Notification.permission !== 'granted') throw new DOMException('Permission denied', 'NotAllowedError')
    const pair = await crypto.subtle.generateKey({ name: 'ECDH', namedCurve: 'P-256' }, true, ['deriveBits'])
    const s: Saved = {
      endpoint: `https://fcm.googleapis.com/fcm/send/e2e-${crypto.randomUUID()}`,
      p256dh: base64(new Uint8Array(await crypto.subtle.exportKey('raw', pair.publicKey))),
      auth: base64(crypto.getRandomValues(new Uint8Array(16))),
      key: base64(new Uint8Array(options?.applicationServerKey as ArrayBuffer)),
    }
    localStorage.setItem(saved, JSON.stringify(s))
    return subscription(s) as unknown as PushSubscription
  }
}

async function withPushService(page: Page) {
  await page.addInitScript(simulatePushService)
  await page.reload()
}

const subscriptionSaved = (page: Page, method: 'PUT' | 'DELETE') =>
  page.waitForResponse((r) => r.url().endsWith('/api/v1/push/subscription') && r.request().method() === method && r.status() === 204)

test('a device turns push notifications on and off from Account', async ({ newDevice }) => {
  const ana = await newDevice()
  await signUp(ana, 'Ana')
  await withPushService(ana)
  await ana.getByRole('link', { name: 'Cuenta' }).click()

  const section = ana.getByRole('region', { name: 'Notificaciones' })
  await expect(section).toContainText('Avisan de un gasto nuevo en el que participas')
  const saved = subscriptionSaved(ana, 'PUT')
  await section.getByRole('button', { name: 'Activar en este dispositivo' }).click()
  await saved
  await expect(section).toContainText('Activadas en este dispositivo.')

  // Opening the app again sends the subscription again, for this session.
  const again = subscriptionSaved(ana, 'PUT')
  await ana.reload()
  await again
  await expect(section).toContainText('Activadas en este dispositivo.')

  const deleted = subscriptionSaved(ana, 'DELETE')
  await section.getByRole('button', { name: 'Desactivar' }).click()
  await deleted
  await expect(section.getByRole('button', { name: 'Activar en este dispositivo' })).toBeVisible()
})

test('the service worker shows a notification per ledger, with each record in it', async ({ newDevice }) => {
  const ana = await newDevice()
  await signUp(ana, 'Ana')
  await ana.evaluate(() => navigator.serviceWorker.ready)

  // The worker keeps what it would show, since this Chromium shows nothing.
  const [worker] = ana.context().serviceWorkers()
  if (!worker) throw new Error('no service worker')
  await worker.evaluate(() => {
    type Shown = { title: string; tag?: string; body?: string; data?: unknown }
    const scope = self as unknown as { shown: Shown[]; registration: ServiceWorkerRegistration }
    scope.shown = []
    scope.registration.showNotification = async (title: string, options: NotificationOptions = {}) => {
      const notification = { title, tag: options.tag, body: options.body, data: options.data }
      const same = scope.shown.findIndex((n) => n.tag === options.tag)
      if (same >= 0) scope.shown[same] = notification
      else scope.shown.push(notification)
    }
    scope.registration.getNotifications = async (filter?: GetNotificationOptions) =>
      scope.shown.filter((n) => n.tag === filter?.tag) as unknown as Notification[]
  })
  const shown = () =>
    worker.evaluate(() =>
      (self as unknown as { shown: { title: string; tag?: string; body?: string }[] }).shown.map((n) => ({ title: n.title, tag: n.tag, body: n.body?.replace(/\u00a0/g, ' ') })),
    )

  // DevTools delivers a push to the service worker, as a push service would.
  const devtools = await ana.context().newCDPSession(ana)
  const registration = new Promise<string>((resolve) => {
    devtools.on('ServiceWorker.workerRegistrationUpdated', ({ registrations }) => {
      const ours = registrations.find((r) => !r.isDeleted)
      if (ours) resolve(ours.registrationId)
    })
  })
  await devtools.send('ServiceWorker.enable')
  const registrationId = await registration
  const origin = new URL(ana.url()).origin
  const deliver = (items: object[]) =>
    devtools.send('ServiceWorker.deliverPushMessage', { origin, registrationId, data: JSON.stringify({ ledgerId: 'trip', title: 'Viaje a Cartagena', items }) })

  await deliver([{ kind: 'expense', actor: 'Beto', description: 'Cena', amount: 9000, currency: 'USD', share: 4500 }])
  await expect.poll(shown).toEqual([{ title: 'Viaje a Cartagena', tag: 'ledger:trip', body: 'Beto añadió «Cena» por USD 90. Tu parte: USD 45.' }])

  // A minute later, the next one joins the notification still on screen.
  await deliver([{ kind: 'settlement', actor: 'Beto', amount: 2000, currency: 'USD', you: 'received', counterpart: 'Beto', byCounterpart: true }])
  await expect
    .poll(shown)
    .toEqual([{ title: 'Viaje a Cartagena', tag: 'ledger:trip', body: 'Beto añadió «Cena» por USD 90. Tu parte: USD 45.\nBeto te pagó USD 20.' }])
})
