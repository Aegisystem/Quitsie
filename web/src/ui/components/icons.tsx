import type { Category } from '../demo/types'

// Two families, never mixed: stroke icons for controls (2px on a 24 grid) and
// solid Isotype-style pictograms for content (people, Groups, Categories).

export type IconName =
  | 'back'
  | 'plus'
  | 'check'
  | 'close'
  | 'offline'
  | 'tray'
  | 'share'
  | 'copy'
  | 'refresh'
  | 'home'
  | 'activity'
  | 'account'
  | 'arrow'
  | 'chevron'
  | 'restore'
  | 'download'

const strokes: Record<IconName, string> = {
  back: 'M15 5l-7 7 7 7',
  plus: 'M12 5v14M5 12h14',
  check: 'M5 12.5l4.5 4.5L19 7.5',
  close: 'M6 6l12 12M18 6L6 18',
  offline: 'M3 3l18 18M8.5 8.6A6 6 0 0 0 6 13.5 4 4 0 0 0 7 21h10M12.3 7.5A6 6 0 0 1 18 12a4 4 0 0 1 2.6 6.4',
  tray: 'M4 13l2.5-7h11L20 13M4 13v6h16v-6M4 13h4.5l1 2.5h5l1-2.5H20',
  share: 'M12 15V3M7.5 7.5L12 3l4.5 4.5M6 11H5v10h14V11h-1',
  copy: 'M9 9h11v11H9zM5 15H4V4h11v1',
  refresh: 'M20 5v5h-5M4 19v-5h5M18.5 10A7 7 0 0 0 6 7.5M5.5 14A7 7 0 0 0 18 16.5',
  home: 'M4 11l8-7 8 7M6 9.5V20h12V9.5',
  activity: 'M4 6h16M4 12h10M4 18h13',
  account: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4.5 20.5a7.5 7.5 0 0 1 15 0',
  arrow: 'M4 12h15M13 6l6 6-6 6',
  chevron: 'M9 5l7 7-7 7',
  restore: 'M4 12a8 8 0 1 0 2.4-5.7M4 4v5h5',
  download: 'M12 3v12M7.5 10.5L12 15l4.5-4.5M5 20h14',
}

export function Icon({ name, size = 24, label }: { name: IconName; size?: number; label?: string }) {
  return (
    <svg
      className="icon"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      role={label ? 'img' : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
    >
      <path d={strokes[name]} />
    </svg>
  )
}

// A person, drawn like an Isotype figure. Solid when linked to a User, hollow
// when pending, faded when they left the Group.
export type FigureState = 'linked' | 'pending' | 'former'

export function Figure({ state = 'linked', size = 28 }: { state?: FigureState; size?: number }) {
  const hollow = state === 'pending'
  return (
    <svg
      className={`figure figure-${state}`}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      aria-hidden="true"
      fill={hollow ? 'none' : 'currentColor'}
      stroke={hollow ? 'currentColor' : 'none'}
      strokeWidth={hollow ? 1.75 : 0}
    >
      <circle cx="12" cy="6.25" r={hollow ? 3.4 : 4.25} />
      <path d={hollow ? 'M6 22.1v-5.6c0-2.6 1.9-4.4 4.4-4.4h3.2c2.5 0 4.4 1.8 4.4 4.4v5.6z' : 'M5 23v-6.6c0-3.1 2.2-5.1 5.2-5.1h3.6c3 0 5.2 2 5.2 5.1V23z'} />
    </svg>
  )
}

// A Group: three figures in a row, the middle one in front.
export function GroupGlyph({ size = 28 }: { size?: number }) {
  return (
    <svg className="figure" width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" fill="currentColor">
      <circle cx="5" cy="8.5" r="2.6" />
      <path d="M1 21v-4.2c0-2 1.4-3.3 3.3-3.3h1.4c1.1 0 2 .4 2.6 1.1V21z" />
      <circle cx="19" cy="8.5" r="2.6" />
      <path d="M23 21v-4.2c0-2-1.4-3.3-3.3-3.3h-1.4c-1.1 0-2 .4-2.6 1.1V21z" />
      <circle cx="12" cy="6" r="3.4" />
      <path d="M6.8 22v-6c0-2.5 1.8-4.2 4.2-4.2h2c2.4 0 4.2 1.7 4.2 4.2v6z" />
    </svg>
  )
}

const pictograms: Record<Category, string> = {
  // Fork and knife
  food: 'M6 2h1.4v6.5h1.2V2H10v6.5h1.2V2h1.4v8a3 3 0 0 1-2.3 2.9V22H8.1v-9.1A3 3 0 0 1 6 10zM16.4 2C18.6 2 20 4.6 20 8.7c0 2.6-.9 4.1-2.2 4.6V22H15.5V2z',
  // Shopping bag
  groceries: 'M8 7V6a4 4 0 0 1 8 0v1h3.5l1 15h-17l1-15zm2 0h4V6a2 2 0 0 0-4 0z',
  // Tram, front view
  transport: 'M9 1h6v2h2.5A2.5 2.5 0 0 1 20 5.5V17a2 2 0 0 1-2 2l1.5 3h-2.3L16 19H8l-1.2 3H4.5L6 19a2 2 0 0 1-2-2V5.5A2.5 2.5 0 0 1 6.5 3H9zM6.5 6v5h11V6zM7.5 14a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3zm9 0a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3z',
  // Bed
  lodging: 'M2 5h2.4v8H11V9h8a3 3 0 0 1 3 3v8h-2.4v-3H4.4v3H2zM5.4 9.6h4.4v2.6H5.4z',
  // House
  home: 'M12 2l10 9h-3v11h-5v-7h-4v7H5V11H2z',
  // Lightning bolt
  utilities: 'M13.5 1L4 13.5h6.5L9 23l10-13h-6.6z',
  // Ticket
  leisure: 'M2 6h20v4a2 2 0 0 0 0 4v4H2v-4a2 2 0 0 0 0-4zm11 2.5v2h2v-2zm0 5v2h2v-2z',
  // Four squares
  other: 'M3 3h8v8H3zm10 0h8v8h-8zM3 13h8v8H3zm10 0h8v8h-8z',
}

export function CategoryGlyph({ category, size = 24 }: { category?: Category; size?: number }) {
  return (
    <svg className="pictogram" width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" fill="currentColor" fillRule="evenodd">
      <path d={pictograms[category ?? 'other']} />
    </svg>
  )
}

// A two-way arrow between people, for Settlements.
export function SettlementGlyph({ size = 24 }: { size?: number }) {
  return (
    <svg className="pictogram" width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" fill="currentColor">
      <path d="M2 7.5h14V3l6 6.2-6 6.3V11H2zM22 16.5H10V13l-6 4.8 6 4.7v-3.5h12z" opacity=".55" />
      <path d="M2 7.5h14V3l6 6.2-6 6.3V11H2z" />
    </svg>
  )
}

export function GoogleMark({ size = 20 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 48 48" aria-hidden="true">
      <path fill="#EA4335" d="M24 9.5c3.5 0 6.6 1.2 9.1 3.6l6.8-6.8C35.8 2.4 30.3 0 24 0 14.6 0 6.6 5.4 2.7 13.2l7.9 6.2C12.5 13.6 17.8 9.5 24 9.5z" />
      <path fill="#4285F4" d="M46.1 24.5c0-1.6-.1-3.1-.4-4.5H24v9h12.4c-.5 2.9-2.2 5.3-4.6 6.9l7.6 5.9c4.4-4.1 6.7-10.1 6.7-17.3z" />
      <path fill="#FBBC05" d="M10.5 28.6A14.5 14.5 0 0 1 9.5 24c0-1.6.3-3.2.8-4.6l-7.9-6.2A24 24 0 0 0 0 24c0 3.9.9 7.5 2.6 10.8z" />
      <path fill="#34A853" d="M24 48c6.5 0 11.9-2.1 15.9-5.8l-7.6-5.9c-2.1 1.4-4.9 2.3-8.3 2.3-6.2 0-11.5-4.2-13.4-9.9l-8 6.2C6.6 42.6 14.6 48 24 48z" />
    </svg>
  )
}

// Quitsie's mark: two equal bars, "we're quits".
export function Mark({ size = 28 }: { size?: number }) {
  return (
    <svg className="mark" width={size} height={size} viewBox="0 0 32 32" aria-hidden="true">
      <rect width="32" height="32" rx="7" fill="var(--even)" />
      <rect x="7" y="10" width="18" height="4.5" rx="2.25" fill="#fff" />
      <rect x="7" y="17.5" width="18" height="4.5" rx="2.25" fill="#fff" />
    </svg>
  )
}
