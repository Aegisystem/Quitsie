import createClient from 'openapi-fetch'
import { describe, expect, it } from 'vitest'
import type { paths } from '../api/schema.gen'
import { createDevicePush, type DevicePushManager, type DeviceSubscription, fromBase64Url, type PushBrowser } from './device'

const serverKey = 'BAECAwQ'
const oldKey = 'BAkJCQk'

// The server behind a simulated fetch; null is a server out of reach, and
// saveStatus is what PUT /push/subscription answers.
function fakeServer(key: string | undefined | null, saveStatus = 204) {
  const requests: { method: string; path: string; body?: unknown }[] = []
  const fetch = async (request: Request) => {
    const path = new URL(request.url).pathname.replace('/api/v1', '')
    const text = await request.text()
    requests.push({ method: request.method, path, ...(text && { body: JSON.parse(text) }) })
    if (key === null) throw new TypeError('Failed to fetch')
    if (request.method === 'GET' && path === '/push/key') {
      return key
        ? Response.json({ publicKey: key })
        : new Response(JSON.stringify({ status: 404, code: 'not_found', title: 'off' }), { status: 404, headers: { 'Content-Type': 'application/problem+json' } })
    }
    if (request.method === 'PUT') return new Response(null, { status: saveStatus })
    return new Response(null, { status: 204 })
  }
  return { requests, client: createClient<paths>({ baseUrl: 'http://quitsie.test/api/v1', fetch }) }
}

// A browser with Web Push; `subscribed` is the key of its current subscription.
function fakeBrowser(opts: { permission?: NotificationPermission; subscribed?: string; answer?: NotificationPermission } = {}) {
  let permission = opts.permission ?? 'default'
  let current: DeviceSubscription | null = null
  let count = 0
  const make = (key: Uint8Array): DeviceSubscription => {
    const endpoint = `https://fcm.googleapis.com/fcm/send/${++count}`
    const subscription: DeviceSubscription = {
      endpoint,
      options: { applicationServerKey: key.slice().buffer },
      toJSON: () => ({ endpoint, keys: { p256dh: 'device-key', auth: 'device-secret' } }),
      unsubscribe: async () => {
        if (current === subscription) current = null
        return true
      },
    }
    return subscription
  }
  if (opts.subscribed) current = make(fromBase64Url(opts.subscribed))
  const manager: DevicePushManager = {
    getSubscription: async () => current,
    subscribe: async ({ applicationServerKey }) => {
      // Browsers ask the User when they have not answered yet.
      if (permission === 'default') permission = opts.answer ?? 'granted'
      if (permission !== 'granted') throw new DOMException('denied', 'NotAllowedError')
      current = make(applicationServerKey)
      return current
    },
  }
  const browser: PushBrowser = { supported: true, needsInstall: false, permission: () => permission, pushManager: async () => manager }
  return { browser, current: () => current }
}

describe('the status of notifications', () => {
  it('is off until the User turns them on, then on', async () => {
    const server = fakeServer(serverKey)
    const device = fakeBrowser()
    const push = createDevicePush({ client: server.client, browser: device.browser })
    expect(await push.status()).toBe('off')

    expect(await push.turnOn()).toEqual({ ok: true, value: undefined })
    expect(await push.status()).toBe('on')
    expect(server.requests.at(-2)).toEqual({
      method: 'PUT',
      path: '/push/subscription',
      body: { endpoint: 'https://fcm.googleapis.com/fcm/send/1', keys: { p256dh: 'device-key', auth: 'device-secret' } },
    })
  })

  it('says why they cannot be turned on', async () => {
    const unsupported: PushBrowser = { supported: false, needsInstall: false, permission: () => 'default', pushManager: async () => undefined }
    const safari: PushBrowser = { ...unsupported, needsInstall: true }
    const client = fakeServer(serverKey).client
    expect(await createDevicePush({ client, browser: unsupported }).status()).toBe('unsupported')
    expect(await createDevicePush({ client, browser: safari }).status()).toBe('install')
    expect(await createDevicePush({ client, browser: fakeBrowser({ permission: 'denied' }).browser }).status()).toBe('denied')
    expect(await createDevicePush({ client: fakeServer(undefined).client, browser: fakeBrowser().browser }).status()).toBe('serverOff')
    // No service worker, as under `npm run dev`.
    expect(await createDevicePush({ client, browser: { ...unsupported, supported: true } }).status()).toBe('unsupported')
  })

  it('shows the device’s subscription offline, but cannot turn them on', async () => {
    const push = createDevicePush({ client: fakeServer(null).client, browser: fakeBrowser({ permission: 'granted', subscribed: serverKey }).browser })
    expect(await push.status()).toBe('on')
    const offline = createDevicePush({ client: fakeServer(null).client, browser: fakeBrowser().browser })
    expect(await offline.status()).toBe('off')
    expect(await offline.turnOn()).toEqual({ ok: false, error: { kind: 'offline' } })
  })
})

describe('turning them on and off', () => {
  it('stays off when the User says no', async () => {
    const device = fakeBrowser({ answer: 'denied' })
    const push = createDevicePush({ client: fakeServer(serverKey).client, browser: device.browser })
    await push.status()
    expect(await push.turnOn()).toEqual({ ok: false, error: { kind: 'denied' } })
    expect(device.current()).toBeNull()
  })

  it('drops the subscription if the server cannot save it', async () => {
    const device = fakeBrowser()
    const push = createDevicePush({ client: fakeServer(serverKey, 500).client, browser: device.browser })
    await push.status()
    expect(await push.turnOn()).toMatchObject({ ok: false, error: { kind: 'problem', status: 500 } })
    expect(device.current()).toBeNull()
  })

  it('turns them off on the device and on the server', async () => {
    const server = fakeServer(serverKey)
    const device = fakeBrowser({ permission: 'granted', subscribed: serverKey })
    const push = createDevicePush({ client: server.client, browser: device.browser })
    await push.turnOff()
    expect(device.current()).toBeNull()
    expect(server.requests.at(-1)).toEqual({ method: 'DELETE', path: '/push/subscription' })
  })
})

describe('signing in and out', () => {
  it('sends the subscription again for the new session', async () => {
    const server = fakeServer(serverKey)
    const device = fakeBrowser({ permission: 'granted', subscribed: serverKey })
    await createDevicePush({ client: server.client, browser: device.browser }).refresh()
    expect(server.requests.map((r) => `${r.method} ${r.path}`)).toEqual(['GET /push/key', 'PUT /push/subscription'])
  })

  it('subscribes again when the server’s key changed', async () => {
    const server = fakeServer(serverKey)
    const device = fakeBrowser({ permission: 'granted', subscribed: oldKey })
    const before = device.current()
    await createDevicePush({ client: server.client, browser: device.browser }).refresh()
    expect(device.current()).not.toBe(before)
    expect(new Uint8Array(device.current()?.options.applicationServerKey ?? new ArrayBuffer(0))).toEqual(fromBase64Url(serverKey))
    expect(server.requests.at(-1)?.method).toBe('PUT')
  })

  it('does nothing without a subscription or the permission', async () => {
    const server = fakeServer(serverKey)
    await createDevicePush({ client: server.client, browser: fakeBrowser({ permission: 'granted' }).browser }).refresh()
    await createDevicePush({ client: server.client, browser: fakeBrowser({ permission: 'default' }).browser }).refresh()
    expect(server.requests).toEqual([])
  })

  it('drops the subscription on signing out', async () => {
    const device = fakeBrowser({ permission: 'granted', subscribed: serverKey })
    await createDevicePush({ client: fakeServer(serverKey).client, browser: device.browser }).forget()
    expect(device.current()).toBeNull()
  })
})
