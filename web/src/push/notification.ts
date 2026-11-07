import { formatAmount } from '../domain/money'
import { es } from '../i18n/es'

// The text of a push notification, from what the server sends (Payload in
// api/internal/push). The service worker composes it, so the amounts follow
// the device's region, as in the app. Each ledger keeps one notification: the
// next records of that ledger join it instead of piling up.

export interface PushItem {
  kind: 'expense' | 'settlement'
  // The Member who wrote it; empty if the server did not know.
  actor: string
  amount: number
  currency: string
  // Of an Expense: its description and the recipient's OwedShare.
  description?: string
  share?: number
  // Of a Settlement: whether the recipient paid or received it, the other
  // Member, and whether that Member wrote it.
  you?: 'paid' | 'received'
  counterpart?: string
  byCounterpart?: boolean
}

export interface PushPayload {
  ledgerId: string
  // The Group's name, or the friend's in a Friendship.
  title: string
  items: PushItem[]
  // Older records the server left out.
  more?: number
}

// What a notification on screen keeps, to add the next records to it.
export interface NoticeData {
  ledgerId: string
  lines: string[]
  more: number
}

export interface Notice {
  title: string
  body: string
  tag: string
  data: NoticeData
}

// The most records a notification lists; it counts the older ones.
const maxLines = 4

const texts = es.pushNotice

function fill(text: string, values: Record<string, string>): string {
  return text.replace(/\{\{(\w+)\}\}/g, (_, key: string) => values[key] ?? '')
}

function money(amount: number, currency: string, locale: string): string {
  try {
    return formatAmount(amount, currency, locale, { hideZeroDecimals: true })
  } catch {
    // A currency this version does not know yet.
    return `${amount} ${currency}`
  }
}

export function lineOf(item: PushItem, locale: string): string {
  const actor = item.actor || texts.someone
  const amount = money(item.amount, item.currency, locale)
  if (item.kind === 'expense') {
    return fill(texts.expense, {
      actor,
      description: item.description ?? '',
      total: amount,
      share: money(item.share ?? 0, item.currency, locale),
    })
  }
  const counterpart = item.counterpart || texts.someone
  if (item.you === 'received') {
    return fill(item.byCounterpart ? texts.youReceived : texts.youReceivedNoted, { actor, counterpart, amount })
  }
  return fill(item.byCounterpart ? texts.youPaid : texts.youPaidNoted, { actor, counterpart, amount })
}

export function tagOf(ledgerId: string): string {
  return `ledger:${ledgerId}`
}

// The notification for payload, added to the one of its ledger still on
// screen, if any.
export function noticeFor(payload: PushPayload, previous: NoticeData | undefined, locale: string): Notice {
  const lines = [...(previous?.lines ?? []), ...payload.items.map((item) => lineOf(item, locale))]
  let more = (previous?.more ?? 0) + (payload.more ?? 0)
  if (lines.length > maxLines) {
    more += lines.length - maxLines
    lines.splice(0, lines.length - maxLines)
  }
  const extra = more ? [fill(more === 1 ? texts.more_one : texts.more_other, { count: String(more) })] : []
  return {
    title: payload.title || 'Quitsie',
    body: [...lines, ...extra].join('\n') || texts.fallbackBody,
    tag: tagOf(payload.ledgerId),
    data: { ledgerId: payload.ledgerId, lines, more },
  }
}

// What a push that cannot be read shows: every push must show something.
export const fallbackNotice = { title: 'Quitsie', body: texts.fallbackBody }

// The payload, if data has its shape; a newer server may add fields.
export function readPayload(data: unknown): PushPayload | undefined {
  if (typeof data !== 'object' || data === null) return undefined
  const p = data as Partial<PushPayload>
  if (typeof p.ledgerId !== 'string' || typeof p.title !== 'string' || !Array.isArray(p.items)) return undefined
  const items = p.items.filter(
    (i): i is PushItem =>
      typeof i === 'object' && i !== null && (i.kind === 'expense' || i.kind === 'settlement') && typeof i.amount === 'number' && typeof i.currency === 'string',
  )
  return { ledgerId: p.ledgerId, title: p.title, items, more: typeof p.more === 'number' ? p.more : 0 }
}

// The data of a notification on screen, if it is one of ours.
export function readNoticeData(data: unknown): NoticeData | undefined {
  if (typeof data !== 'object' || data === null) return undefined
  const d = data as Partial<NoticeData>
  if (typeof d.ledgerId !== 'string' || !Array.isArray(d.lines)) return undefined
  return { ledgerId: d.ledgerId, lines: d.lines.filter((l) => typeof l === 'string'), more: typeof d.more === 'number' ? d.more : 0 }
}

// Where tapping the notification of a ledger opens the app: the ledger's
// screen, a route of src/ui/router.ts.
export function ledgerPath(ledgerId: string): string {
  return `/#/ledgers/${encodeURIComponent(ledgerId)}`
}
