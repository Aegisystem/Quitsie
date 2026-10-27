import i18n from 'i18next'

export function todayISO(): string {
  const d = new Date()
  return new Date(d.getTime() - d.getTimezoneOffset() * 60_000).toISOString().slice(0, 10)
}

// "Hoy", "Ayer" or "12 de octubre" for a YYYY-MM-DD date.
export function formatDay(date: string): string {
  const today = todayISO()
  if (date === today) return i18n.t('common.today')
  const y = new Date(`${today}T12:00:00`)
  y.setDate(y.getDate() - 1)
  if (date === y.toISOString().slice(0, 10)) return i18n.t('common.yesterday')
  const d = new Date(`${date}T12:00:00`)
  const sameYear = d.getFullYear() === new Date().getFullYear()
  return new Intl.DateTimeFormat(i18n.language, { day: 'numeric', month: 'long', year: sameYear ? undefined : 'numeric' }).format(d)
}

// "hace 5 minutos", "hace 2 días".
export function formatRelative(iso: string): string {
  const seconds = (new Date(iso).getTime() - Date.now()) / 1000
  const rtf = new Intl.RelativeTimeFormat(i18n.language, { numeric: 'auto' })
  const steps: [Intl.RelativeTimeFormatUnit, number][] = [
    ['second', 60],
    ['minute', 60],
    ['hour', 24],
    ['day', 30],
    ['month', 12],
    ['year', Infinity],
  ]
  let value = seconds
  for (const [unit, size] of steps) {
    if (Math.abs(value) < size) return rtf.format(Math.round(value), unit)
    value /= size
  }
  return ''
}
