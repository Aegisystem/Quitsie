// The theme is a choice of this device, not of the User: it lives in
// localStorage and survives signing out. public/theme.js applies it before the
// first paint (the CSP allows no inline script); this module changes it later.
// Keep the key and the bar colours in step with that file.

export type ThemeChoice = 'system' | 'light' | 'dark'

export const THEME_KEY = 'quitsie.theme'
export const themeChoices: readonly ThemeChoice[] = ['system', 'light', 'dark']

// The ink of the top bar in each scheme (--bar in styles.css), which is what
// the browser's own bar shows through <meta name="theme-color">.
export const barColor = { light: '#1f2328', dark: '#0a0b0d' } as const

export function parseTheme(value: string | null): ThemeChoice {
  return value === 'light' || value === 'dark' ? value : 'system'
}

export function readTheme(): ThemeChoice {
  try {
    return parseTheme(localStorage.getItem(THEME_KEY))
  } catch {
    return 'system'
  }
}

// Each theme-color meta keeps its media query; a forced theme gives all of
// them the same colour, and "system" gives each the one of its scheme.
export function applyTheme(choice: ThemeChoice, doc: Document = document) {
  const root = doc.documentElement
  if (choice === 'system') delete root.dataset.theme
  else root.dataset.theme = choice
  for (const meta of doc.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')) {
    const scheme = choice !== 'system' ? choice : meta.media.includes('dark') ? 'dark' : 'light'
    meta.content = barColor[scheme]
  }
}

export function saveTheme(choice: ThemeChoice) {
  try {
    if (choice === 'system') localStorage.removeItem(THEME_KEY)
    else localStorage.setItem(THEME_KEY, choice)
  } catch {
    // Storage blocked: the choice holds until the page closes.
  }
  applyTheme(choice)
}

// Another tab of this device changed the theme.
export function followOtherTabs() {
  window.addEventListener('storage', (e) => {
    if (e.key === THEME_KEY || e.key === null) applyTheme(parseTheme(e.key === null ? null : e.newValue))
  })
}
