import { AlertTriangle } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { forwardRef, useId, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes, type TextareaHTMLAttributes } from 'react'
import { cn } from '@/lib/cn'

const control =
  'w-full rounded-field border border-line bg-card px-4 text-ink placeholder:text-muted/80 ' +
  'focus:border-focus focus:outline-2 focus:outline-offset-0 focus:outline-focus aria-[invalid=true]:border-[var(--danger)]'

interface FieldProps {
  label: ReactNode
  hint?: ReactNode
  error?: string
  optional?: boolean
  children: (props: { id: string; 'aria-invalid'?: boolean; 'aria-describedby'?: string }) => ReactNode
  className?: string
}

/** Label + control + hint/error, wired up for screen readers. */
export function Field({ label, hint, error, optional, children, className }: FieldProps) {
  const { t } = useTranslation()
  const id = useId()
  const describedBy = error ? `${id}-error` : hint ? `${id}-hint` : undefined
  return (
    <div className={cn('flex flex-col gap-1.5', className)}>
      <label htmlFor={id} className="text-sm font-medium text-ink">
        {label}
        {optional && (
          <>
            {' '}
            <span className="font-normal text-muted">({t('common.optional')})</span>
          </>
        )}
      </label>
      {children({ id, 'aria-invalid': error ? true : undefined, 'aria-describedby': describedBy })}
      {error ? (
        <p id={`${id}-error`} className="flex items-center gap-1.5 text-sm text-[var(--danger)]">
          <AlertTriangle className="size-4 shrink-0" aria-hidden />
          {error}
        </p>
      ) : hint ? (
        <p id={`${id}-hint`} className="text-sm text-muted">
          {hint}
        </p>
      ) : null}
    </div>
  )
}

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(function Input({ className, ...rest }, ref) {
  return <input ref={ref} className={cn(control, 'h-12', className)} {...rest} />
})

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(function Textarea({ className, ...rest }, ref) {
  return <textarea ref={ref} className={cn(control, 'min-h-24 py-3', className)} {...rest} />
})

export const Select = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement>>(function Select({ className, children, ...rest }, ref) {
  return (
    <select ref={ref} className={cn(control, 'h-12 appearance-none bg-[length:16px] bg-[right_1rem_center] bg-no-repeat pr-10', className)}
      style={{ backgroundImage: "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%235a5f68' stroke-width='2'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E\")" }}
      {...rest}>
      {children}
    </select>
  )
})

/** A labelled on/off switch row. */
export function Toggle({ label, hint, checked, onChange, disabled }: {
  label: ReactNode
  hint?: ReactNode
  checked: boolean
  onChange: (v: boolean) => void
  disabled?: boolean
}) {
  const id = useId()
  return (
    <div className="flex items-center justify-between gap-4 py-1">
      <label htmlFor={id} className="flex flex-col">
        <span className="text-sm font-medium text-ink">{label}</span>
        {hint && <span className="text-sm text-muted">{hint}</span>}
      </label>
      <button
        id={id}
        type="button"
        role="switch"
        aria-checked={checked}
        disabled={disabled}
        onClick={() => onChange(!checked)}
        className={cn(
          'relative h-7 w-12 shrink-0 rounded-full transition disabled:opacity-50',
          checked ? 'bg-featured' : 'bg-card-muted ring-1 ring-line ring-inset',
        )}
      >
        <span className={cn('absolute top-1 left-1 size-5 rounded-full transition', checked ? 'translate-x-5 bg-accent' : 'bg-card shadow')} />
      </button>
    </div>
  )
}
