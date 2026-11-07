import { fail, ok, type Result } from '../domain/result'
import { type ApiClient, type CallError, type CallResult, call } from '../session/api'

// Push notifications on this device (ADR 0013). The browser holds the
// subscription; the server keeps a copy for the current session and deletes
// it when the session ends. Signing out also drops it here, so whoever signs
// in next on this browser turns notifications on again for themselves.

// What the Account screen shows:
//   unsupported  this browser has no Web Push;
//   install      an iPhone or iPad outside the Home Screen app, where Safari
//                has no Web Push;
//   serverOff    the server sends none (it has no VAPID keys);
//   denied       the User blocked notifications in the browser;
//   off, on      whether this device gets them.
export type PushStatus = 'unsupported' | 'install' | 'serverOff' | 'denied' | 'off' | 'on'

// The parts of PushSubscription and PushManager this module uses.
export interface DeviceSubscription {
  readonly endpoint: string
  readonly options: { readonly applicationServerKey: ArrayBuffer | null }
  toJSON(): { endpoint?: string; keys?: Record<string, string> }
  unsubscribe(): Promise<boolean>
}

export interface DevicePushManager {
  getSubscription(): Promise<DeviceSubscription | null>
  subscribe(options: { userVisibleOnly: true; applicationServerKey: Uint8Array<ArrayBuffer> }): Promise<DeviceSubscription>
}

// The browser, simulated in the tests.
export interface PushBrowser {
  supported: boolean
  needsInstall: boolean
  permission(): NotificationPermission
  // undefined when there is no service worker to receive the notifications.
  pushManager(): Promise<DevicePushManager | undefined>
}

// Why turnOn failed: the User said no, the browser could not subscribe, or
// the server did not answer.
export type TurnOnError = { kind: 'denied' } | { kind: 'failed' } | CallError

export interface DevicePush {
  status(): Promise<PushStatus>
  // Call it straight from a tap, after status(): browsers ask for the
  // permission only then, and nothing may wait before the request.
  turnOn(): Promise<Result<void, TurnOnError>>
  turnOff(): Promise<void>
  // On every sign-in: sends the subscription to the server again, for this
  // session, and subscribes again if the server's key changed.
  refresh(): Promise<void>
  // On every sign-out: drops the subscription of this browser.
  forget(): Promise<void>
}

export function createDevicePush({ client, browser }: { client: ApiClient; browser: PushBrowser }): DevicePush {
  // Loaded by status(), so turnOn() starts with the request of permission.
  let ready: { manager: DevicePushManager; key: Uint8Array<ArrayBuffer> } | undefined

  async function serverKey(): Promise<CallResult<Uint8Array<ArrayBuffer> | undefined>> {
    const answer = await call(() => client.GET('/push/key'))
    if (answer.ok) return ok(fromBase64Url(answer.value.publicKey))
    if (answer.error.kind === 'problem' && answer.error.status === 404) return ok(undefined)
    return answer
  }

  function save(subscription: DeviceSubscription): Promise<CallResult<void>> {
    const { endpoint, keys } = subscription.toJSON()
    return call(() =>
      client.PUT('/push/subscription', { body: { endpoint: endpoint ?? subscription.endpoint, keys: { p256dh: keys?.p256dh ?? '', auth: keys?.auth ?? '' } } }),
    )
  }

  return {
    async status() {
      if (!browser.supported) return browser.needsInstall ? 'install' : 'unsupported'
      if (browser.permission() === 'denied') return 'denied'
      const manager = await browser.pushManager()
      if (!manager) return 'unsupported'
      const current = await manager.getSubscription()
      const key = await serverKey()
      if (key.ok && !key.value) return 'serverOff'
      if (key.ok && key.value) ready = { manager, key: key.value }
      return current ? 'on' : 'off'
    },

    async turnOn(): Promise<Result<void, TurnOnError>> {
      if (!ready) return fail({ kind: 'offline' })
      let subscription: DeviceSubscription
      try {
        subscription = await ready.manager.subscribe({ userVisibleOnly: true, applicationServerKey: ready.key })
      } catch {
        return fail({ kind: browser.permission() === 'denied' ? 'denied' : 'failed' })
      }
      const saved = await save(subscription)
      if (!saved.ok) await subscription.unsubscribe().catch(() => false)
      return saved
    },

    async turnOff() {
      const manager = await browser.pushManager()
      const subscription = await manager?.getSubscription()
      // Once unsubscribed, the push service refuses the endpoint and the
      // server deletes its copy at the next notification, even if this call
      // never arrives.
      await subscription?.unsubscribe().catch(() => false)
      await call(() => client.DELETE('/push/subscription'))
    },

    async refresh() {
      if (!browser.supported || browser.permission() !== 'granted') return
      const manager = await browser.pushManager()
      let subscription = await manager?.getSubscription()
      if (!manager || !subscription) return
      const key = await serverKey()
      if (!key.ok) return
      if (!key.value) {
        await subscription.unsubscribe().catch(() => false)
        return
      }
      if (!sameBytes(subscription.options.applicationServerKey, key.value)) {
        await subscription.unsubscribe().catch(() => false)
        try {
          subscription = await manager.subscribe({ userVisibleOnly: true, applicationServerKey: key.value })
        } catch {
          // The browser wants a tap: the Account screen shows them off.
          return
        }
      }
      await save(subscription)
    },

    async forget() {
      if (!browser.supported) return
      const manager = await browser.pushManager()
      const subscription = await manager?.getSubscription()
      await subscription?.unsubscribe().catch(() => false)
    },
  }
}

export function fromBase64Url(text: string): Uint8Array<ArrayBuffer> {
  const base64 = text.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(text.length / 4) * 4, '=')
  const binary = atob(base64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes
}

function sameBytes(a: ArrayBuffer | null, b: Uint8Array): boolean {
  if (!a || a.byteLength !== b.byteLength) return false
  const view = new Uint8Array(a)
  return view.every((byte, i) => byte === b[i])
}

// How long to wait for the service worker. Under `npm run dev` there is none,
// and navigator.serviceWorker.ready would never settle.
const workerWait = 5000

export function browserPush(): PushBrowser {
  if (typeof window === 'undefined') return { supported: false, needsInstall: false, permission: () => 'default', pushManager: async () => undefined }
  const supported = 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window
  const ios = /iPad|iPhone|iPod/.test(navigator.userAgent) || (navigator.userAgent.includes('Macintosh') && navigator.maxTouchPoints > 1)
  const standalone = window.matchMedia('(display-mode: standalone)').matches || (navigator as { standalone?: boolean }).standalone === true
  return {
    supported,
    needsInstall: !supported && ios && !standalone,
    permission: () => Notification.permission,
    pushManager: () =>
      Promise.race([
        navigator.serviceWorker.ready.then((registration) => registration.pushManager as DevicePushManager),
        new Promise<undefined>((resolve) => setTimeout(() => resolve(undefined), workerWait)),
      ]),
  }
}
