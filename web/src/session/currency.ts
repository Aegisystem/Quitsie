import type { CurrencyCode } from '../domain'

function inRegions(currency: CurrencyCode, regions: string): [string, CurrencyCode][] {
  return regions.split(' ').map((region) => [region, currency])
}

// The currency people use day to day in each ISO 3166 region, after CLDR. Funds
// (CLF, COU, UYI, BOV, MXV…) never appear: they are units of account, not money
// anyone pays with (docs/reglas-de-negocio.md). Where two circulate, the one
// prices are usually given in: USD in Panama and El Salvador.
const regionCurrency = new Map<string, CurrencyCode>([
  ...inRegions('EUR', 'AD AT AX BE BG BL CY DE EE ES FI FR GF GP GR HR IE IT LT LU LV MC ME MF MQ MT NL PM PT RE SI SK SM TF VA XK YT'),
  ...inRegions('USD', 'AS BQ EC FM GU IO MH MP PA PR PW SV TC TL UM US VG VI'),
  ...inRegions('GBP', 'GB GG GS IM JE'),
  ...inRegions('AUD', 'AU CC CX HM KI NF NR TV'),
  ...inRegions('NZD', 'CK NU NZ PN TK'),
  ...inRegions('CHF', 'CH LI'),
  ...inRegions('DKK', 'DK FO GL'),
  ...inRegions('NOK', 'BV NO SJ'),
  ...inRegions('XCD', 'AG AI DM GD KN LC MS VC'),
  ...inRegions('XOF', 'BF BJ CI GW ML NE SN TG'),
  ...inRegions('XAF', 'CF CG CM GA GQ TD'),
  ...inRegions('XPF', 'NC PF WF'),
  ...inRegions('XCG', 'CW SX'),
  ...inRegions('MAD', 'EH MA'),
  ...inRegions('ILS', 'IL PS'),
  // One region each.
  ...[
    'AE AED', 'AF AFN', 'AL ALL', 'AM AMD', 'AO AOA', 'AR ARS', 'AW AWG', 'AZ AZN', 'BA BAM', 'BB BBD',
    'BD BDT', 'BH BHD', 'BI BIF', 'BM BMD', 'BN BND', 'BO BOB', 'BR BRL', 'BS BSD', 'BT BTN', 'BW BWP',
    'BY BYN', 'BZ BZD', 'CA CAD', 'CD CDF', 'CL CLP', 'CN CNY', 'CO COP', 'CR CRC', 'CU CUP', 'CV CVE',
    'CZ CZK', 'DJ DJF', 'DO DOP', 'DZ DZD', 'EG EGP', 'ER ERN', 'ET ETB', 'FJ FJD', 'FK FKP', 'GE GEL',
    'GH GHS', 'GI GIP', 'GM GMD', 'GN GNF', 'GT GTQ', 'GY GYD', 'HK HKD', 'HN HNL', 'HT HTG', 'HU HUF',
    'ID IDR', 'IN INR', 'IQ IQD', 'IR IRR', 'IS ISK', 'JM JMD', 'JO JOD', 'JP JPY', 'KE KES', 'KG KGS',
    'KH KHR', 'KM KMF', 'KP KPW', 'KR KRW', 'KW KWD', 'KY KYD', 'KZ KZT', 'LA LAK', 'LB LBP', 'LK LKR',
    'LR LRD', 'LS LSL', 'LY LYD', 'MD MDL', 'MG MGA', 'MK MKD', 'MM MMK', 'MN MNT', 'MO MOP', 'MR MRU',
    'MU MUR', 'MV MVR', 'MW MWK', 'MX MXN', 'MY MYR', 'MZ MZN', 'NA NAD', 'NG NGN', 'NI NIO', 'NP NPR',
    'OM OMR', 'PE PEN', 'PG PGK', 'PH PHP', 'PK PKR', 'PL PLN', 'PY PYG', 'QA QAR', 'RO RON', 'RS RSD',
    'RU RUB', 'RW RWF', 'SA SAR', 'SB SBD', 'SC SCR', 'SD SDG', 'SE SEK', 'SG SGD', 'SH SHP', 'SL SLE',
    'SO SOS', 'SR SRD', 'SS SSP', 'ST STN', 'SY SYP', 'SZ SZL', 'TH THB', 'TJ TJS', 'TM TMT', 'TN TND',
    'TO TOP', 'TR TRY', 'TT TTD', 'TW TWD', 'TZ TZS', 'UA UAH', 'UG UGX', 'UY UYU', 'UZ UZS', 'VE VES',
    'VN VND', 'VU VUV', 'WS WST', 'YE YER', 'ZA ZAR', 'ZM ZMW', 'ZW ZWG',
  ].map((pair) => pair.split(' ') as [string, CurrencyCode]),
])

// The Preferred currency to propose from a browser language such as 'es-MX'.
// A language without a region ('es') takes its likely one ('ES'); a region with
// no single currency ('419', Latin America) gives undefined.
export function suggestCurrency(language: string): CurrencyCode | undefined {
  let region: string | undefined
  try {
    const locale = new Intl.Locale(language)
    region = locale.region ?? locale.maximize().region
  } catch {
    return undefined
  }
  return region ? regionCurrency.get(region) : undefined
}

// The currencies a User can pick, by code: those of some region, so never a fund.
export const currencyOptions: readonly CurrencyCode[] = [...new Set(regionCurrency.values())].sort()
