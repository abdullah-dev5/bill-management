import * as Popover from '@radix-ui/react-popover'
import { CalendarDays, ChevronLeft, ChevronRight } from 'lucide-react'
import { forwardRef, useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/cn'
import { currentMonthKey, formatDate, formatMonthShort, parseDate, parseUserDate, parseUserMonth, shiftMonth, shortMonthName, toISODate, todayISO } from '@/lib/dates'
import { control } from './field'

type Mode = 'date' | 'month'

interface DateInputProps {
  id?: string
  name?: string
  /** YYYY-MM-DD (date) or YYYY-MM (month); '' when empty. */
  value: string | undefined
  onChange: (value: string) => void
  onBlur?: () => void
  mode?: Mode
  min?: string
  max?: string
  /** Required fields can't be cleared from the calendar. */
  required?: boolean
  disabled?: boolean
  placeholder?: string
  className?: string
  'aria-invalid'?: boolean
  'aria-describedby'?: string
}

const display = (mode: Mode, v: string | undefined) => (!v ? '' : mode === 'date' ? formatDate(v) : formatMonthShort(v))
const inRange = (v: string, min?: string, max?: string) => (!min || v >= min) && (!max || v <= max)

/**
 * A date or month field in the app's style: type it ("8 Oct 2026", "8/10/2026", "Sep 2026") or pick it
 * from the calendar. Replaces the browser's date inputs, which look different on every platform.
 */
export const DateInput = forwardRef<HTMLInputElement, DateInputProps>(function DateInput(
  { id, name, value, onChange, onBlur, mode = 'date', min, max, required, disabled, placeholder, className, ...aria },
  ref,
) {
  const { t } = useTranslation()
  const [text, setText] = useState(display(mode, value))
  const [open, setOpen] = useState(false)
  const inputRef = useRef<HTMLInputElement | null>(null)

  // Follow outside changes (form reset, a scan filling the field) unless the person is typing.
  useEffect(() => {
    if (document.activeElement !== inputRef.current) setText(display(mode, value))
  }, [value, mode])

  const commit = () => {
    const raw = text.trim()
    if (!raw) {
      if (value) onChange('')
      return
    }
    const parsed = mode === 'date' ? parseUserDate(raw) : parseUserMonth(raw)
    if (parsed && inRange(parsed, min, max)) {
      if (parsed !== value) onChange(parsed)
      setText(display(mode, parsed))
    } else {
      setText(display(mode, value)) // not understood: put back what was there
    }
  }

  const pick = (v: string) => {
    onChange(v)
    setText(display(mode, v))
    setOpen(false)
    inputRef.current?.focus()
  }

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Anchor asChild>
        <div className={cn('relative', className)}>
          <input
            id={id}
            name={name}
            ref={(el) => {
              inputRef.current = el
              if (typeof ref === 'function') ref(el)
              else if (ref) ref.current = el
            }}
            type="text"
            autoComplete="off"
            disabled={disabled}
            value={text}
            placeholder={placeholder ?? (mode === 'date' ? t('picker.datePlaceholder') : t('picker.monthPlaceholder'))}
            onChange={(e) => setText(e.target.value)}
            onBlur={() => {
              commit()
              onBlur?.()
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                commit()
              } else if (e.key === 'ArrowDown' && e.altKey) {
                e.preventDefault()
                setOpen(true)
              }
            }}
            className={cn(control, 'h-12 pr-12 tabular disabled:opacity-60', className && 'h-full')}
            {...aria}
          />
          <Popover.Trigger asChild>
            <button
              type="button"
              disabled={disabled}
              aria-label={mode === 'date' ? t('picker.chooseDate') : t('picker.chooseMonth')}
              className="absolute inset-y-0 right-0 grid w-12 place-items-center rounded-r-field text-muted hover:text-ink disabled:opacity-60"
            >
              <CalendarDays className="size-5" aria-hidden />
            </button>
          </Popover.Trigger>
        </div>
      </Popover.Anchor>
      <Popover.Portal>
        <Popover.Content
          align="end"
          sideOffset={6}
          collisionPadding={12}
          onOpenAutoFocus={(e) => e.preventDefault()}
          className="z-[60] w-[19rem] max-w-[calc(100vw-24px)] rounded-card bg-card p-3 text-body shadow-float ring-1 ring-line"
        >
          {mode === 'date' ? (
            <DayCalendar value={value} min={min} max={max} onPick={pick} />
          ) : (
            <MonthCalendar value={value} min={min} max={max} onPick={pick} autoFocus />
          )}
          <div className="mt-2 flex items-center justify-between border-t border-line pt-2">
            {(() => {
              const now = mode === 'date' ? todayISO() : currentMonthKey()
              return (
                <button
                  type="button"
                  disabled={!inRange(now, min, max)}
                  onClick={() => pick(now)}
                  className="h-9 rounded-full px-3 text-sm font-semibold text-ink hover:bg-card-muted disabled:opacity-40"
                >
                  {mode === 'date' ? t('picker.today') : t('picker.thisMonth')}
                </button>
              )
            })()}
            {!required && value && (
              <button type="button" onClick={() => pick('')} className="h-9 rounded-full px-3 text-sm font-semibold text-muted hover:bg-card-muted hover:text-ink">
                {t('picker.clear')}
              </button>
            )}
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
})

const monthTitle = new Intl.DateTimeFormat('en-GB', { month: 'long', year: 'numeric' })
const dayLabel = new Intl.DateTimeFormat('en-GB', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })
const WEEKDAYS = ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su']

function addDays(iso: string, n: number): string {
  const d = parseDate(iso)
  d.setDate(d.getDate() + n)
  return toISODate(d)
}

function NavButton({ label, onClick, children }: { label: string; onClick: () => void; children: React.ReactNode }) {
  return (
    <button type="button" aria-label={label} onClick={onClick} className="grid size-9 place-items-center rounded-full text-ink hover:bg-card-muted">
      {children}
    </button>
  )
}

/** A month of days, Monday first, with full keyboard support (arrows, Home/End, PageUp/PageDown). */
function DayCalendar({ value, min, max, onPick }: { value?: string; min?: string; max?: string; onPick: (v: string) => void }) {
  const { t } = useTranslation()
  const today = todayISO()
  const [focus, setFocus] = useState(() => value || (max && today > max ? max : min && today < min ? min : today))
  const [picking, setPicking] = useState(false) // month/year view
  const grid = useRef<HTMLDivElement>(null)

  const view = focus.slice(0, 7)
  const first = parseDate(view + '-01')
  const lead = (first.getDay() + 6) % 7 // Monday-first offset
  const start = addDays(view + '-01', -lead)
  const days = Array.from({ length: 42 }, (_, i) => addDays(start, i))

  useEffect(() => {
    // Focus the chosen day when the calendar opens and after keyboard moves.
    grid.current?.querySelector<HTMLButtonElement>(`[data-day="${focus}"]`)?.focus({ preventScroll: true })
  }, [focus, picking])

  if (picking) {
    return (
      <MonthCalendar
        value={view}
        min={min?.slice(0, 7)}
        max={max?.slice(0, 7)}
        autoFocus
        onPick={(m) => {
          const day = Math.min(parseDate(focus).getDate(), new Date(Number(m.slice(0, 4)), Number(m.slice(5, 7)), 0).getDate())
          setFocus(`${m}-${String(day).padStart(2, '0')}`)
          setPicking(false)
        }}
      />
    )
  }

  const onKey = (e: KeyboardEvent) => {
    const steps: Record<string, () => string> = {
      ArrowLeft: () => addDays(focus, -1),
      ArrowRight: () => addDays(focus, 1),
      ArrowUp: () => addDays(focus, -7),
      ArrowDown: () => addDays(focus, 7),
      Home: () => addDays(focus, -((parseDate(focus).getDay() + 6) % 7)),
      End: () => addDays(focus, 6 - ((parseDate(focus).getDay() + 6) % 7)),
      PageUp: () => shiftDay(focus, e.shiftKey ? -12 : -1),
      PageDown: () => shiftDay(focus, e.shiftKey ? 12 : 1),
    }
    const step = steps[e.key]
    if (step) {
      e.preventDefault()
      setFocus(step())
    }
  }

  return (
    <div>
      <div className="mb-2 flex items-center justify-between">
        <NavButton label={t('picker.previousMonth')} onClick={() => setFocus(shiftDay(focus, -1))}>
          <ChevronLeft className="size-5" aria-hidden />
        </NavButton>
        <button
          type="button"
          onClick={() => setPicking(true)}
          aria-label={`${monthTitle.format(first)}. ${t('picker.pickMonthYear')}`}
          className="h-9 rounded-full px-3 text-sm font-semibold text-ink hover:bg-card-muted"
          aria-live="polite"
        >
          {monthTitle.format(first)}
        </button>
        <NavButton label={t('picker.nextMonth')} onClick={() => setFocus(shiftDay(focus, 1))}>
          <ChevronRight className="size-5" aria-hidden />
        </NavButton>
      </div>
      <div className="grid grid-cols-7 text-center text-xs text-muted" aria-hidden>
        {WEEKDAYS.map((d) => (
          <span key={d} className="py-1">
            {d}
          </span>
        ))}
      </div>
      <div ref={grid} role="grid" aria-label={monthTitle.format(first)} className="grid grid-cols-7 gap-y-0.5" onKeyDown={onKey}>
        {days.map((d) => {
          const outside = !d.startsWith(view)
          const disabled = !inRange(d, min, max)
          const selected = d === value
          return (
            <button
              key={d}
              type="button"
              role="gridcell"
              data-day={d}
              tabIndex={d === focus ? 0 : -1}
              disabled={disabled}
              aria-selected={selected}
              aria-current={d === today ? 'date' : undefined}
              aria-label={dayLabel.format(parseDate(d))}
              onClick={() => onPick(d)}
              className={cn(
                'mx-auto grid size-10 place-items-center rounded-full text-sm tabular transition outline-none',
                'focus-visible:outline-2 focus-visible:outline-focus',
                selected ? 'bg-featured font-semibold text-on-featured' : 'text-ink hover:bg-card-muted',
                !selected && outside && 'text-muted/70',
                !selected && d === today && 'ring-1 ring-[var(--text-muted)] ring-inset font-semibold',
                disabled && 'cursor-not-allowed opacity-30 hover:bg-transparent',
              )}
            >
              {parseDate(d).getDate()}
            </button>
          )
        })}
      </div>
    </div>
  )
}

/** Same day in another month, clamped to that month's length. */
function shiftDay(iso: string, months: number): string {
  const key = shiftMonth(iso.slice(0, 7), months)
  const last = new Date(Number(key.slice(0, 4)), Number(key.slice(5, 7)), 0).getDate()
  return `${key}-${String(Math.min(parseDate(iso).getDate(), last)).padStart(2, '0')}`
}

/** A year of months (3 × 4), with year navigation and arrow-key movement. */
function MonthCalendar({ value, min, max, onPick, autoFocus }: { value?: string; min?: string; max?: string; onPick: (v: string) => void; autoFocus?: boolean }) {
  const { t } = useTranslation()
  const now = currentMonthKey()
  const [focus, setFocus] = useState(() => value || (max && now > max ? max : min && now < min ? min : now))
  const grid = useRef<HTMLDivElement>(null)
  const year = Number(focus.slice(0, 4))

  useEffect(() => {
    if (autoFocus) grid.current?.querySelector<HTMLButtonElement>(`[data-month="${focus}"]`)?.focus({ preventScroll: true })
  }, [focus, autoFocus])

  const onKey = (e: KeyboardEvent) => {
    const delta: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -3, ArrowDown: 3, PageUp: -12, PageDown: 12 }
    const d = delta[e.key]
    if (d) {
      e.preventDefault()
      setFocus(shiftMonth(focus, d))
    }
  }

  return (
    <div>
      <div className="mb-2 flex items-center justify-between">
        <NavButton label={t('picker.previousYear')} onClick={() => setFocus(shiftMonth(focus, -12))}>
          <ChevronLeft className="size-5" aria-hidden />
        </NavButton>
        <span className="text-sm font-semibold text-ink tabular" aria-live="polite">
          {year}
        </span>
        <NavButton label={t('picker.nextYear')} onClick={() => setFocus(shiftMonth(focus, 12))}>
          <ChevronRight className="size-5" aria-hidden />
        </NavButton>
      </div>
      <div ref={grid} role="grid" aria-label={String(year)} className="grid grid-cols-3 gap-1.5" onKeyDown={onKey}>
        {Array.from({ length: 12 }, (_, i) => {
          const key = `${year}-${String(i + 1).padStart(2, '0')}`
          const selected = key === value
          const disabled = !inRange(key, min, max)
          return (
            <button
              key={key}
              type="button"
              role="gridcell"
              data-month={key}
              tabIndex={key === focus ? 0 : -1}
              disabled={disabled}
              aria-selected={selected}
              aria-current={key === now ? 'date' : undefined}
              aria-label={monthTitle.format(parseDate(key + '-01'))}
              onClick={() => onPick(key)}
              className={cn(
                'h-11 rounded-full text-sm transition outline-none focus-visible:outline-2 focus-visible:outline-focus',
                selected ? 'bg-featured font-semibold text-on-featured' : 'text-ink hover:bg-card-muted',
                !selected && key === now && 'ring-1 ring-[var(--text-muted)] ring-inset font-semibold',
                disabled && 'cursor-not-allowed opacity-30 hover:bg-transparent',
              )}
            >
              {shortMonthName(i)}
            </button>
          )
        })}
      </div>
    </div>
  )
}
