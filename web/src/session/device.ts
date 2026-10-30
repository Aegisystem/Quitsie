// A Session's device is the User-Agent it signed in with. This reads the
// browser and the system out of it, so the account can say "Safari en iPhone".
export interface Device {
  browser?: string
  os?: string
}

// Order matters: Edge and Opera also say Chrome; Chrome also says Safari.
const browsers: [RegExp, string][] = [
  [/\bEdg(e|A|iOS)?\//, 'Edge'],
  [/\b(OPR|OPT)\/|\bOpera\b/, 'Opera'],
  [/\bSamsungBrowser\//, 'Samsung Internet'],
  [/\b(Firefox|FxiOS)\//, 'Firefox'],
  [/\b(Chrome|CriOS|Chromium)\//, 'Chrome'],
  [/\bVersion\/[\d.]+.*\bSafari\//, 'Safari'],
]

const systems: [RegExp, string][] = [
  [/\biPhone\b/, 'iPhone'],
  [/\biPad\b/, 'iPad'],
  [/\bAndroid\b/, 'Android'],
  [/\bCrOS\b/, 'ChromeOS'],
  [/\bWindows\b/, 'Windows'],
  [/\bMac OS X\b|\bMacintosh\b/, 'Mac'],
  [/\bLinux\b/, 'Linux'],
]

// Undefined when the text is not a User-Agent (say, the server already wrote
// "Safari en iPhone"); the caller then shows it as it is. An installed PWA on
// iOS has no browser in its User-Agent, so only the system comes back.
export function describeDevice(device: string): Device | undefined {
  if (!device.startsWith('Mozilla/')) return undefined
  const find = (table: [RegExp, string][]) => table.find(([pattern]) => pattern.test(device))?.[1]
  return { browser: find(browsers), os: find(systems) }
}
