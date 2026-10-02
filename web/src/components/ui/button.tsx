import { Loader2 } from 'lucide-react'
import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { cn } from '@/lib/cn'

type Variant = 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger' | 'accent'
type Size = 'md' | 'sm' | 'icon'

const variants: Record<Variant, string> = {
  primary: 'bg-featured text-on-featured hover:brightness-110',
  secondary: 'bg-card-muted text-ink hover:brightness-95',
  outline: 'border border-ink/80 text-ink hover:border-accent',
  ghost: 'text-ink hover:bg-card-muted',
  danger: 'bg-[#9b1c1c] text-white hover:brightness-110',
  accent: 'bg-accent text-on-accent hover:brightness-95',
}

const sizes: Record<Size, string> = {
  md: 'h-12 px-6 text-sm',
  sm: 'h-10 px-4 text-sm',
  icon: 'h-11 w-11 rounded-tile',
}

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  loading?: boolean
  icon?: ReactNode
}

/** Pill buttons, at least 44px tall — docs/10-design-system.md §5. */
export function Button({ variant = 'primary', size = 'md', loading, icon, className, children, disabled, ...rest }: ButtonProps) {
  return (
    <button
      type="button"
      className={cn(
        'inline-flex shrink-0 items-center justify-center gap-2 font-semibold whitespace-nowrap transition active:scale-[0.98]',
        'disabled:pointer-events-none disabled:opacity-50',
        size === 'icon' ? '' : 'rounded-full',
        variants[variant],
        sizes[size],
        className,
      )}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading ? <Loader2 className="size-4 animate-spin" aria-hidden /> : icon}
      {children}
    </button>
  )
}
