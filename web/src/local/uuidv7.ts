// UUIDv7 (RFC 9562): 48 bits of Unix time in milliseconds, then random bits.
// The client gives each Expense and Settlement its id, so resending the queue
// never duplicates anything (ADR 0001), and ids sort by creation time.
export function uuidv7(now = Date.now(), random = crypto.getRandomValues(new Uint8Array(10))): string {
  const bytes = new Uint8Array(16)
  for (let i = 0; i < 6; i++) bytes[i] = Math.floor(now / 2 ** (8 * (5 - i))) & 0xff
  bytes.set(random, 6)
  bytes[6] = 0x70 | (bytes[6]! & 0x0f) // version 7
  bytes[8] = 0x80 | (bytes[8]! & 0x3f) // variant 10
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}
