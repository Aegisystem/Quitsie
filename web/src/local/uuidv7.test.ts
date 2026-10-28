import { expect, test } from 'vitest'
import { uuidv7 } from './uuidv7'

test('matches the UUIDv7 example of RFC 9562 (appendix A.6)', () => {
  // 2022-02-22 19:22:22 UTC, and the example's random bits.
  const random = new Uint8Array([0x0c, 0xc3, 0x18, 0xc4, 0xdc, 0x0c, 0x0c, 0x07, 0x39, 0x8f])
  expect(uuidv7(0x017f22e279b0, random)).toBe('017f22e2-79b0-7cc3-98c4-dc0c0c07398f')
})

test('is a version 7, variant 10 UUID by default', () => {
  expect(uuidv7()).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/)
})
