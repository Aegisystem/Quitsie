import { useTranslation } from 'react-i18next'
import { type CurrencyCode, currencyChoices } from '../../domain'

// The options of a currency selector: the User's own currencies, then the others.
// `value` always shows, even if it is not on offer, rather than a blank selector.
export function CurrencyOptions({
  value,
  preferred,
  used,
  label = (code) => code,
}: {
  value: CurrencyCode
  preferred: CurrencyCode
  used?: readonly CurrencyCode[]
  label?: (code: CurrencyCode) => string
}) {
  const { t } = useTranslation()
  const { first, rest } = currencyChoices(preferred, used)
  const mine = [...first, ...rest].includes(value) ? first : [value, ...first]
  const option = (code: CurrencyCode) => (
    <option key={code} value={code}>
      {label(code)}
    </option>
  )
  return (
    <>
      {mine.length > 0 && <optgroup label={t('currencies.yours')}>{mine.map(option)}</optgroup>}
      <optgroup label={t('currencies.others')}>{rest.map(option)}</optgroup>
    </>
  )
}
