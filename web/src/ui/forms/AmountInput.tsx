import { type InputHTMLAttributes, useLayoutEffect, useRef } from 'react'
import { formatAmountTyping } from '../../domain'
import { regionLocale } from '../money'

type Props = Omit<InputHTMLAttributes<HTMLInputElement>, 'value' | 'onChange' | 'inputMode'> & {
  value: string
  onChange: (text: string) => void
  currency: string
  // Ajustes may take something off; every other amount is positive.
  allowNegative?: boolean
}

// An amount field that shows the region's thousands separators while the
// Usuario types, keeping the caret among the same digits (formatAmountTyping).
export function AmountInput({ value, onChange, currency, allowNegative, ...rest }: Props) {
  const input = useRef<HTMLInputElement>(null)
  const caret = useRef<number | null>(null)

  // React writes the new value and leaves the caret at the end; put it back
  // where the Usuario was typing.
  useLayoutEffect(() => {
    const field = input.current
    if (caret.current === null || !field) return
    if (document.activeElement === field) field.setSelectionRange(caret.current, caret.current)
    caret.current = null
  })

  return (
    <input
      {...rest}
      ref={input}
      inputMode="decimal"
      value={value}
      onChange={(e) => {
        const native = e.nativeEvent as InputEvent
        const typed = formatAmountTyping(
          {
            text: e.target.value,
            caret: e.target.selectionStart ?? e.target.value.length,
            inputType: native.inputType,
            data: native.data,
          },
          currency,
          regionLocale(),
          { allowNegative },
        )
        caret.current = typed.caret
        onChange(typed.text)
      }}
    />
  )
}
