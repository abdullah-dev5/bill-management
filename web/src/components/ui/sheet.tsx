import * as Dialog from '@radix-ui/react-dialog'
import { X } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/cn'

interface SheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: string
  children: ReactNode
  footer?: ReactNode
}

/** Bottom sheet on phones, centred dialog on larger screens — docs/10 §5. */
export function Sheet({ open, onOpenChange, title, description, children, footer }: SheetProps) {
  const { t } = useTranslation()
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/40" />
        <Dialog.Content
          className={cn(
            'fixed z-50 flex max-h-[90dvh] w-full flex-col bg-card text-body shadow-float',
            'inset-x-0 bottom-0 rounded-t-card pb-[env(safe-area-inset-bottom)]',
            'sm:inset-auto sm:top-1/2 sm:left-1/2 sm:max-w-[560px] sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-card sm:pb-0',
          )}
        >
          <div className="mx-auto mt-3 h-1.5 w-10 rounded-full bg-card-muted sm:hidden" aria-hidden />
          <div className="flex items-start justify-between gap-4 px-6 pt-4 sm:pt-6">
            <div>
              <Dialog.Title className="text-xl font-semibold text-ink">{title}</Dialog.Title>
              {description ? (
                <Dialog.Description className="mt-1 text-sm text-muted">{description}</Dialog.Description>
              ) : (
                <Dialog.Description className="sr-only">{title}</Dialog.Description>
              )}
            </div>
            <Dialog.Close className="-mr-2 grid size-11 place-items-center rounded-tile text-muted hover:bg-card-muted" aria-label={t('common.close')}>
              <X className="size-5" />
            </Dialog.Close>
          </div>
          <div className="overflow-y-auto px-6 py-4">{children}</div>
          {footer && <div className="flex flex-col-reverse gap-2 border-t border-line px-6 py-4 sm:flex-row sm:justify-end">{footer}</div>}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
