import * as RadixSelect from '@radix-ui/react-select'
import { AlertTriangle, Check, ChevronDown, ChevronUp } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { forwardRef, useId, type InputHTMLAttributes, type ReactNode, type TextareaHTMLAttributes } from 'react'
import { cn } from '@/lib/cn'

export const control =
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

export interface SelectOption {
  value: string
  label: string
  /** A short line under the label explaining the choice. */
  hint?: string
  /** Options sharing a group are listed together under its heading, in order of first appearance. */
  group?: string
  disabled?: boolean
}

interface SelectProps {
  id?: string
  name?: string
  value: string | undefined
  onChange: (value: string) => void
  onBlur?: () => void
  options: SelectOption[]
  placeholder?: string
  disabled?: boolean
  className?: string
  'aria-invalid'?: boolean
  'aria-describedby'?: string
  'aria-label'?: string
}

// Radix reserves '' for "nothing selected"; options with an empty value (e.g. "None") use this instead.
const EMPTY = '__empty__'
const toItem = (v: string) => (v === '' ? EMPTY : v)
const fromItem = (v: string) => (v === EMPTY ? '' : v)

/** A dropdown in the app's style, with optional group headings and a hint under each option. */
export const Select = forwardRef<HTMLButtonElement, SelectProps>(function Select(
  { id, name, value, onChange, onBlur, options, placeholder, disabled, className, ...aria },
  ref,
) {
  const { t } = useTranslation()
  const groups: { name?: string; options: SelectOption[] }[] = []
  for (const o of options) {
    const g = groups.find((x) => x.name === o.group)
    if (g) g.options.push(o)
    else groups.push({ name: o.group, options: [o] })
  }
  const known = value != null && options.some((o) => o.value === value)
  return (
    <RadixSelect.Root
      name={name}
      value={known ? toItem(value) : ''}
      onValueChange={(v) => onChange(fromItem(v))}
      onOpenChange={(open) => !open && onBlur?.()}
      disabled={disabled}
    >
      <RadixSelect.Trigger
        ref={ref}
        id={id}
        className={cn(control, 'flex h-12 items-center justify-between gap-2 text-left disabled:opacity-60 data-[placeholder]:text-muted', className)}
        {...aria}
      >
        <span className="min-w-0 truncate">
          <RadixSelect.Value placeholder={placeholder ?? t('picker.choose')} />
        </span>
        <RadixSelect.Icon>
          <ChevronDown className="size-4 shrink-0 text-muted" aria-hidden />
        </RadixSelect.Icon>
      </RadixSelect.Trigger>
      <RadixSelect.Portal>
        <RadixSelect.Content
          position="popper"
          sideOffset={6}
          collisionPadding={12}
          className="z-[60] max-h-[min(22rem,var(--radix-select-content-available-height))] w-[var(--radix-select-trigger-width)] min-w-56 overflow-hidden rounded-field bg-card text-body shadow-float ring-1 ring-line"
        >
          <RadixSelect.ScrollUpButton className="flex h-7 items-center justify-center text-muted">
            <ChevronUp className="size-4" aria-hidden />
          </RadixSelect.ScrollUpButton>
          <RadixSelect.Viewport className="p-1.5">
            {options.length === 0 && <p className="px-3 py-2 text-sm text-muted">{t('picker.noOptions')}</p>}
            {groups.map((g, gi) => (
              <RadixSelect.Group key={g.name ?? `_${gi}`}>
                {gi > 0 && <RadixSelect.Separator className="mx-2 my-1.5 h-px bg-line" />}
                {g.name && <RadixSelect.Label className="px-3 pt-1.5 pb-1 text-xs font-semibold tracking-wide text-muted uppercase">{g.name}</RadixSelect.Label>}
                {g.options.map((o) => (
                  <RadixSelect.Item
                    key={o.value}
                    value={toItem(o.value)}
                    disabled={o.disabled}
                    textValue={o.label}
                    className={cn(
                      'relative flex min-h-11 cursor-pointer flex-col justify-center rounded-tile py-2 pr-9 pl-3 outline-none select-none',
                      'data-[highlighted]:bg-card-muted data-[disabled]:cursor-not-allowed data-[disabled]:opacity-40',
                    )}
                  >
                    <RadixSelect.ItemText>
                      <span className="text-sm text-ink">{o.label}</span>
                    </RadixSelect.ItemText>
                    {o.hint && <span className="text-xs text-muted">{o.hint}</span>}
                    <RadixSelect.ItemIndicator className="absolute top-1/2 right-3 -translate-y-1/2">
                      <Check className="size-4 text-ink" aria-hidden />
                    </RadixSelect.ItemIndicator>
                  </RadixSelect.Item>
                ))}
              </RadixSelect.Group>
            ))}
          </RadixSelect.Viewport>
          <RadixSelect.ScrollDownButton className="flex h-7 items-center justify-center text-muted">
            <ChevronDown className="size-4" aria-hidden />
          </RadixSelect.ScrollDownButton>
        </RadixSelect.Content>
      </RadixSelect.Portal>
    </RadixSelect.Root>
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
