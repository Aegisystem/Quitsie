import { describe, expect, it } from 'vitest'
import { applyTheme, barColor, parseTheme } from './theme'

// Tests run in Node (vitest.config.ts): a document with the two theme-color
// metas of index.html is enough for applyTheme.
function fakeDocument() {
  const metas = [
    { media: '(prefers-color-scheme: light)', content: barColor.light },
    { media: '(prefers-color-scheme: dark)', content: barColor.dark },
  ]
  const dataset: Record<string, string> = {}
  const doc = { documentElement: { dataset }, querySelectorAll: () => metas } as unknown as Document
  return { doc, dataset, metas }
}

describe('parseTheme', () => {
  it('keeps a forced theme and falls back to the system for anything else', () => {
    expect(parseTheme('light')).toBe('light')
    expect(parseTheme('dark')).toBe('dark')
    expect(parseTheme(null)).toBe('system')
    expect(parseTheme('sepia')).toBe('system')
  })
})

describe('applyTheme', () => {
  it('forces the scheme on the root and paints both bars with it', () => {
    const { doc, dataset, metas } = fakeDocument()
    applyTheme('dark', doc)
    expect(dataset.theme).toBe('dark')
    expect(metas.map((m) => m.content)).toEqual([barColor.dark, barColor.dark])

    applyTheme('light', doc)
    expect(dataset.theme).toBe('light')
    expect(metas.map((m) => m.content)).toEqual([barColor.light, barColor.light])
  })

  it('gives each bar its own scheme again when back to the system', () => {
    const { doc, dataset, metas } = fakeDocument()
    applyTheme('dark', doc)
    applyTheme('system', doc)
    expect(dataset.theme).toBeUndefined()
    expect(metas.map((m) => m.content)).toEqual([barColor.light, barColor.dark])
  })
})
