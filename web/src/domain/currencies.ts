// An ISO 4217 code such as 'COP'. Amounts are integers in its minor unit.
export type CurrencyCode = string

function withDigits(digits: number, codes: string): [CurrencyCode, number][] {
  return codes.split(' ').map((code) => [code, digits])
}

// ISO 4217 list one, published 2026-09-17: every current currency, with the
// number of decimals of its minor unit. Codes whose minor unit is N.A. (precious
// metals, SDR, bond units, testing and XXX) are left out: nothing is priced in them.
const minorUnits = new Map<CurrencyCode, number>([
  ...withDigits(0, 'BIF CLP DJF GNF ISK JPY KMF KRW PYG RWF UGX UYI VND VUV XAF XOF XPF'),
  ...withDigits(3, 'BHD IQD JOD KWD LYD OMR TND'),
  ...withDigits(4, 'CLF UYW'),
  ...withDigits(
    2,
    'AED AFN ALL AMD AOA ARS AUD AWG AZN BAM BBD BDT BMD BND BOB BOV BRL BSD BTN BWP ' +
      'BYN BZD CAD CDF CHE CHF CHW CNY COP COU CRC CUP CVE CZK DKK DOP DZD EGP ERN ETB ' +
      'EUR FJD FKP GBP GEL GHS GIP GMD GTQ GYD HKD HNL HTG HUF IDR ILS INR IRR JMD KES ' +
      'KGS KHR KPW KYD KZT LAK LBP LKR LRD LSL MAD MDL MGA MKD MMK MNT MOP MRU MUR MVR ' +
      'MWK MXN MXV MYR MZN NAD NGN NIO NOK NPR NZD PAB PEN PGK PHP PKR PLN QAR RON RSD ' +
      'RUB SAR SBD SCR SDG SEK SGD SHP SLE SOS SRD SSP STN SVC SYP SZL THB TJS TMT TOP ' +
      'TRY TTD TWD TZS UAH USD USN UYU UZS VED VES WST XAD XCD XCG YER ZAR ZMW ZWG',
  ),
])

// Fund codes (IsFund in the ISO 4217 list): units of account such as Chile's
// Unidad de Fomento. They are valid, but nobody pays a dinner with them, so the
// interface does not offer them (docs/reglas-de-negocio.md).
const funds = new Set<CurrencyCode>(['BOV', 'CHE', 'CHW', 'CLF', 'COU', 'MXV', 'USN', 'UYI', 'UYW', 'XAD'])

const offered: readonly CurrencyCode[] = [...minorUnits.keys()].filter((code) => !funds.has(code)).sort()

// The currencies a currency selector offers, in alphabetical order.
export function offeredCurrencies(): readonly CurrencyCode[] {
  return offered
}

export function isCurrency(code: string): code is CurrencyCode {
  return minorUnits.has(code)
}

// Number of decimals of the currency's minor unit: 2 for USD (cents), 0 for JPY.
export function minorUnitDigits(currency: CurrencyCode): number {
  const digits = minorUnits.get(currency)
  if (digits === undefined) {
    throw new Error(`${currency} is not a current ISO 4217 currency`)
  }
  return digits
}
