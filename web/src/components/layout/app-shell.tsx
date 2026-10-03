import { Link, Outlet, useNavigate, useRouterState } from '@tanstack/react-router'
import { LayoutGrid, Plus, Receipt, Settings, House, ChevronRight } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useProfiles } from '@/api/hooks'
import { IconTile } from '@/components/ui/misc'
import { Sheet } from '@/components/ui/sheet'
import { cn } from '@/lib/cn'

const NAV = [
  { to: '/', key: 'home', Icon: House, exact: true },
  { to: '/profiles', key: 'profiles', Icon: LayoutGrid },
  { to: '/bills', key: 'bills', Icon: Receipt },
  { to: '/settings', key: 'settings', Icon: Settings },
] as const

export function AppShell() {
  const { t } = useTranslation()
  const [addOpen, setAddOpen] = useState(false)
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const isActive = (to: string, exact?: boolean) => (exact ? pathname === to : pathname === to || pathname.startsWith(to + '/'))

  return (
    <div className="min-h-dvh lg:flex">
      <a href="#main" className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50 focus:rounded-full focus:bg-accent focus:px-4 focus:py-2 focus:text-on-accent">
        {t('nav.skip')}
      </a>
      <Sidebar isActive={isActive} onAdd={() => setAddOpen(true)} />

      <main id="main" className="mx-auto w-full max-w-[1200px] px-4 pt-[calc(env(safe-area-inset-top)+16px)] pb-28 sm:px-6 lg:px-10 lg:pt-10 lg:pb-12">
        <Outlet />
      </main>

      {/* Bottom navigation — phones and tablets */}
      <nav
        aria-label="Primary"
        className="fixed inset-x-0 bottom-0 z-30 border-t border-line bg-card pb-[env(safe-area-inset-bottom)] lg:hidden"
      >
        <ul className="mx-auto grid h-16 max-w-lg grid-cols-5 items-center">
          {NAV.slice(0, 2).map((n) => (
            <BottomItem key={n.to} to={n.to} label={t(`nav.${n.key}`)} Icon={n.Icon} active={isActive(n.to, 'exact' in n)} />
          ))}
          <li className="flex justify-center">
            <button
              type="button"
              onClick={() => setAddOpen(true)}
              aria-label={t('nav.add')}
              className="-mt-6 grid size-14 place-items-center rounded-full bg-accent text-on-accent shadow-float active:scale-95"
            >
              <Plus className="size-7" strokeWidth={2.25} />
            </button>
          </li>
          {NAV.slice(2).map((n) => (
            <BottomItem key={n.to} to={n.to} label={t(`nav.${n.key}`)} Icon={n.Icon} active={isActive(n.to)} />
          ))}
        </ul>
      </nav>

      <AddSheet open={addOpen} onOpenChange={setAddOpen} />
    </div>
  )
}

function BottomItem({ to, label, Icon, active }: { to: string; label: string; Icon: typeof House; active: boolean }) {
  return (
    <li>
      <Link to={to} className="flex flex-col items-center gap-0.5 py-1 text-xs font-medium text-body" aria-current={active ? 'page' : undefined}>
        <span className={cn('grid h-8 w-14 place-items-center rounded-full transition', active && 'bg-featured text-accent')}>
          <Icon className="size-5" strokeWidth={1.75} aria-hidden />
        </span>
        {label}
      </Link>
    </li>
  )
}

function Sidebar({ isActive, onAdd }: { isActive: (to: string, exact?: boolean) => boolean; onAdd: () => void }) {
  const { t } = useTranslation()
  const { data: profiles } = useProfiles()
  const pinned = (profiles ?? []).filter((p) => p.pinned)
  return (
    <aside className="sticky top-0 hidden h-dvh w-[264px] shrink-0 flex-col gap-6 overflow-y-auto px-5 py-8 lg:flex">
      <Link to="/" className="flex items-center gap-2 px-2 text-lg font-semibold text-ink">
        <img src="/favicon.svg" alt="" className="size-8" />
        {t('app.name')}
      </Link>
      <button
        type="button"
        onClick={onAdd}
        className="flex h-12 items-center justify-center gap-2 rounded-full bg-accent font-semibold text-on-accent hover:brightness-95"
      >
        <Plus className="size-5" aria-hidden /> {t('nav.add')}
      </button>
      <nav aria-label="Primary">
        <ul className="flex flex-col gap-1">
          {NAV.map((n) => {
            const active = isActive(n.to, 'exact' in n)
            return (
              <li key={n.to}>
                <Link
                  to={n.to}
                  aria-current={active ? 'page' : undefined}
                  className={cn(
                    'flex h-11 items-center gap-3 rounded-full px-4 text-sm font-semibold transition',
                    active ? 'bg-featured text-on-featured' : 'text-body hover:bg-card-muted',
                  )}
                >
                  <n.Icon className={cn('size-5', active && 'text-accent')} strokeWidth={1.75} aria-hidden />
                  {t(`nav.${n.key}`)}
                </Link>
              </li>
            )
          })}
        </ul>
      </nav>
      {pinned.length > 0 && (
        <div>
          <h2 className="mb-2 px-2 text-xs font-semibold tracking-wide text-muted">{t('nav.pinned').toUpperCase()}</h2>
          <ul className="flex flex-col gap-1">
            {pinned.map((p) => (
              <li key={p.id}>
                <Link
                  to="/profiles/$profileId"
                  params={{ profileId: p.id }}
                  className="flex items-center gap-3 rounded-tile px-2 py-1.5 text-sm font-medium text-body hover:bg-card-muted"
                  activeProps={{ className: 'bg-card-muted' }}
                >
                  <IconTile icon={p.icon} color={p.color} size="sm" />
                  <span className="truncate">{p.name}</span>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      )}
      <Link to="/profiles/new" className="flex items-center gap-2 px-2 text-sm font-semibold text-body hover:text-ink">
        <Plus className="size-4" aria-hidden /> {t('nav.newProfile')}
      </Link>
    </aside>
  )
}

/** "What are you adding?" — pick a profile, or a quick expense (FR-ACCT-13). */
function AddSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data: profiles } = useProfiles()
  const list = profiles ?? []
  const general = list.find((p) => p.isDefault)
  const others = list.filter((p) => !p.isDefault)

  const go = (profileId: string) => {
    onOpenChange(false)
    void navigate({ to: '/bills/new', search: { profileId } })
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title={t('bills.addWhat')}>
      <div className="flex flex-col gap-2">
        {general && (
          <AddRow onClick={() => go(general.id)} icon={<IconTile icon="receipt" color={general.color} />} title={t('bills.quickExpense')} sub={t('bills.quickExpenseHint')} />
        )}
        {others.map((p) => (
          <AddRow key={p.id} onClick={() => go(p.id)} icon={<IconTile icon={p.icon} color={p.color} />} title={p.name} sub={t(`kind.${p.kind}`)} />
        ))}
        <Link
          to="/profiles/new"
          onClick={() => onOpenChange(false)}
          className="mt-2 flex h-12 items-center justify-center gap-2 rounded-full bg-card-muted text-sm font-semibold text-ink"
        >
          <Plus className="size-4" aria-hidden /> {t('nav.newProfile')}
        </Link>
      </div>
    </Sheet>
  )
}

function AddRow({ onClick, icon, title, sub }: { onClick: () => void; icon: ReactNode; title: string; sub: string }) {
  return (
    <button type="button" onClick={onClick} className="flex items-center gap-3 rounded-tile p-2 text-left hover:bg-card-muted">
      {icon}
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate font-medium text-ink">{title}</span>
        <span className="text-sm text-muted">{sub}</span>
      </span>
      <ChevronRight className="size-5 text-muted" aria-hidden />
    </button>
  )
}

export function PageHeader({ title, back, actions, children }: { title: ReactNode; back?: ReactNode; actions?: ReactNode; children?: ReactNode }) {
  return (
    <header className="mb-6 flex flex-col gap-2">
      {back}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold sm:text-[28px]">{title}</h1>
        {actions && <div className="flex gap-2">{actions}</div>}
      </div>
      {children}
    </header>
  )
}
