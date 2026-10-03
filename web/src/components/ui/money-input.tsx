import { useState, type InputHTMLAttributes } from 'react'
import { cn } from '@/lib/cn'
import { currencySymbol, formatAmount, parseAmount } from '@/lib/money'

interface MoneyInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'value' | 'onChange' | 'type'> {
  /** Minor units, or null when empty. NaN marks unparseable text. */
  value: number | null | undefined
  onChange: (minor: number | null) => void
  currency?: string
  allowNegative?: boolean
}

/** "Rs" prefix, right-aligned digits, numeric keyboard; formats on blur — docs/10 §5 Inputs. */
export function MoneyInput({ value, onChange, currency = 'PKR', allowNegative, className, onBlur, ...rest }: MoneyInputProps) {
  const [text, setText] = useState(() => (value == null || Number.isNaN(value) ? '' : formatAmount(value)))
  const [lastValue, setLastValue] = useState(value)

  // Follow external value changes (e.g. a form reset or a pre-filled amount) without
  // fighting the user's typing: only rewrite the text when it no longer matches the value.
  if (value !== lastValue) {
    setLastValue(value)
    const parsed = parseAmount(text)
    if (value == null && parsed !== null) setText('')
    else if (value != null && !Number.isNaN(value) && parsed !== value) setText(formatAmount(value))
  }

  return (
    <div
      className={cn(
        'flex h-12 items-center rounded-field border border-line bg-card px-4 focus-within:border-focus focus-within:outline-2 focus-within:outline-focus',
        'has-[[aria-invalid=true]]:border-[var(--danger)]',
        className,
      )}
    >
      <span className="mr-2 text-muted" aria-hidden>
        {currencySymbol(currency)}
      </span>
      <input
        {...rest}
        type="text"
        inputMode="decimal"
        autoComplete="off"
        value={text}
        className="tabular h-full w-full min-w-0 bg-transparent text-right text-ink outline-none"
        onChange={(e) => {
          const raw = allowNegative ? e.target.value : e.target.value.replace('-', '')
          setText(raw)
          onChange(parseAmount(raw))
        }}
        onBlur={(e) => {
          const parsed = parseAmount(text)
          if (parsed !== null && !Number.isNaN(parsed)) setText(formatAmount(parsed))
          onBlur?.(e)
        }}
      />
    </div>
  )
}
