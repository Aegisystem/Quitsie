import { describe, expect, it, vi } from 'vitest'
import { applyUpdate, offerUpdate, subscribeToUpdate, updateAvailable } from './update'

describe('a newer version of the app', () => {
  it('is not offered until the service worker has one waiting', async () => {
    expect(updateAvailable()).toBe(false)
    await expect(applyUpdate()).resolves.toBeUndefined()
  })

  it('tells the banner when it arrives and activates only when asked', async () => {
    const listener = vi.fn()
    const unsubscribe = subscribeToUpdate(listener)
    const activate = vi.fn(async () => {})

    offerUpdate(activate)

    expect(listener).toHaveBeenCalledOnce()
    expect(updateAvailable()).toBe(true)
    expect(activate).not.toHaveBeenCalled()

    await applyUpdate()
    expect(activate).toHaveBeenCalledOnce()
    unsubscribe()
  })
})
