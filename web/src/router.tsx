import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, createRoute, createRouter, lazyRouteComponent, Link, Outlet, redirect } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ApiError } from '@/api/client'
import { meQuery } from '@/api/hooks'
import { AppShell } from '@/components/layout/app-shell'
import { EmptyState } from '@/components/ui/misc'

// Pages load on demand so the first screen downloads only what it needs.
const auth = () => import('@/features/auth/auth-pages')
const bills = () => import('@/features/bills/bill-pages')
const profiles = () => import('@/features/profiles/profile-pages')
const AuthLayout = lazyRouteComponent(auth, 'AuthLayout')
const SignInPage = lazyRouteComponent(auth, 'SignInPage')
const SignUpPage = lazyRouteComponent(auth, 'SignUpPage')
const ForgotPasswordPage = lazyRouteComponent(auth, 'ForgotPasswordPage')
const ResetPasswordPage = lazyRouteComponent(auth, 'ResetPasswordPage')
const BillsPage = lazyRouteComponent(bills, 'BillsPage')
const NewBillPage = lazyRouteComponent(bills, 'NewBillPage')
const BillDetailPage = lazyRouteComponent(bills, 'BillDetailPage')
const EditBillPage = lazyRouteComponent(bills, 'EditBillPage')
const ProfilesPage = lazyRouteComponent(profiles, 'ProfilesPage')
const NewProfilePage = lazyRouteComponent(profiles, 'NewProfilePage')
const ProfilePage = lazyRouteComponent(profiles, 'ProfilePage')
const HomePage = lazyRouteComponent(() => import('@/features/home/home-page'), 'HomePage')
const OnboardingPage = lazyRouteComponent(() => import('@/features/onboarding/onboarding-page'), 'OnboardingPage')
const SettingsPage = lazyRouteComponent(() => import('@/features/settings/settings-page'), 'SettingsPage')
const ScanPage = lazyRouteComponent(() => import('@/features/scan/scan-page'), 'ScanPage')
const AnalyticsPage = lazyRouteComponent(() => import('@/features/analytics/analytics-page'), 'AnalyticsPage')
const SearchPage = lazyRouteComponent(() => import('@/features/search/search-page'), 'SearchPage')
const NotificationsPage = lazyRouteComponent(() => import('@/features/notifications/notifications'), 'NotificationsPage')

interface RouterContext {
  queryClient: QueryClient
}

const str = (v: unknown) => (typeof v === 'string' && v !== '' ? v : undefined)

function NotFound() {
  const { t } = useTranslation()
  return (
    <EmptyState
      title={t('common.notFound')}
      action={
        <Link to="/" className="font-semibold text-ink underline">
          {t('nav.home')}
        </Link>
      }
    />
  )
}

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: Outlet,
  notFoundComponent: NotFound,
})

// --- Signed-out pages ----------------------------------------------------------------

const publicRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'public',
  component: AuthLayout,
  beforeLoad: async ({ context }) => {
    // Already signed in? Go straight to the app.
    const me = await context.queryClient.ensureQueryData(meQuery).catch(() => null)
    if (me) throw redirect({ to: '/' })
  },
})

const signInRoute = createRoute({
  getParentRoute: () => publicRoute,
  path: '/sign-in',
  component: SignInPage,
  validateSearch: (s: Record<string, unknown>): { redirect?: string; reset?: string } => ({ redirect: str(s.redirect), reset: str(s.reset) }),
})
const signUpRoute = createRoute({ getParentRoute: () => publicRoute, path: '/sign-up', component: SignUpPage })
const forgotRoute = createRoute({ getParentRoute: () => publicRoute, path: '/forgot-password', component: ForgotPasswordPage })
const resetRoute = createRoute({
  getParentRoute: () => publicRoute,
  path: '/reset-password',
  component: ResetPasswordPage,
  validateSearch: (s: Record<string, unknown>): { token?: string } => ({ token: str(s.token) }),
})

// --- Signed-in app --------------------------------------------------------------------

const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'app',
  component: AppShell,
  beforeLoad: async ({ context, location }) => {
    try {
      await context.queryClient.ensureQueryData(meQuery)
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        throw redirect({ to: '/sign-in', search: { redirect: location.href } })
      }
      throw err
    }
  },
})

const homeRoute = createRoute({ getParentRoute: () => appRoute, path: '/', component: HomePage })
const onboardingRoute = createRoute({ getParentRoute: () => appRoute, path: '/onboarding', component: OnboardingPage })

const profilesRoute = createRoute({ getParentRoute: () => appRoute, path: '/profiles', component: ProfilesPage })
const newProfileRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/profiles/new',
  component: NewProfilePage,
  validateSearch: (s: Record<string, unknown>): { typeId?: string } => ({ typeId: str(s.typeId) }),
})
const profileRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/profiles/$profileId',
  component: ProfilePage,
  validateSearch: (s: Record<string, unknown>): { tab?: 'bills' | 'payments' | 'documents' | 'settings'; status?: string } => ({
    tab: s.tab === 'bills' || s.tab === 'payments' || s.tab === 'documents' || s.tab === 'settings' ? s.tab : undefined,
    status: str(s.status),
  }),
})

/** Bills list filters, kept in the URL (FR-SRCH-04). Lists are comma-separated; amounts in whole rupees. */
export interface BillsSearch {
  status?: string
  profileId?: string
  archived?: true
  type?: string
  provider?: string
  min?: number
  max?: number
  source?: string
  from?: string
  to?: string
  review?: true
}
const num = (v: unknown) => {
  const n = typeof v === 'number' ? v : typeof v === 'string' && v.trim() ? Number(v) : NaN
  return Number.isFinite(n) && n >= 0 ? n : undefined
}
const dateStr = (v: unknown) => (typeof v === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(v) ? v : undefined)

const billsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/bills',
  component: BillsPage,
  validateSearch: (s: Record<string, unknown>): BillsSearch => ({
    status: str(s.status),
    profileId: str(s.profileId),
    archived: s.archived === true || s.archived === 'true' ? true : undefined,
    type: str(s.type),
    provider: str(s.provider),
    min: num(s.min),
    max: num(s.max),
    source: str(s.source),
    from: dateStr(s.from),
    to: dateStr(s.to),
    review: s.review === true || s.review === 'true' ? true : undefined,
  }),
})
const newBillRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/bills/new',
  component: NewBillPage,
  validateSearch: (s: Record<string, unknown>): { profileId?: string; period?: string } => ({
    profileId: str(s.profileId),
    period: typeof s.period === 'string' && monthRe.test(s.period) ? s.period : undefined,
  }),
})
const billRoute = createRoute({ getParentRoute: () => appRoute, path: '/bills/$billId', component: BillDetailPage })
const editBillRoute = createRoute({ getParentRoute: () => appRoute, path: '/bills/$billId/edit', component: EditBillPage })

const settingsRoute = createRoute({ getParentRoute: () => appRoute, path: '/settings', component: SettingsPage })
const scanRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/scan/$documentId',
  component: ScanPage,
  // official: the person saved this from the provider's site (assisted retrieval).
  validateSearch: (s: Record<string, unknown>): { official?: true; profileId?: string } => ({
    official: s.official === true || s.official === 'true' ? true : undefined,
    profileId: str(s.profileId),
  }),
})
const monthRe = /^\d{4}-(0[1-9]|1[0-2])$/
const presets = ['month', '3m', 'year', '12m', 'custom'] as const
const analyticsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/analytics',
  component: AnalyticsPage,
  validateSearch: (
    s: Record<string, unknown>,
  ): { preset?: (typeof presets)[number]; from?: string; to?: string; basis?: 'cash' } => ({
    preset: presets.includes(s.preset as (typeof presets)[number]) ? (s.preset as (typeof presets)[number]) : undefined,
    from: typeof s.from === 'string' && monthRe.test(s.from) ? s.from : undefined,
    to: typeof s.to === 'string' && monthRe.test(s.to) ? s.to : undefined,
    basis: s.basis === 'cash' ? 'cash' : undefined,
  }),
})

const searchRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/search',
  component: SearchPage,
  validateSearch: (s: Record<string, unknown>): { q?: string } => ({ q: typeof s.q === 'string' && s.q.trim() ? s.q.slice(0, 200) : undefined }),
})

const notificationsRoute = createRoute({ getParentRoute: () => appRoute, path: '/notifications', component: NotificationsPage })

const routeTree = rootRoute.addChildren([
  publicRoute.addChildren([signInRoute, signUpRoute, forgotRoute, resetRoute]),
  appRoute.addChildren([
    homeRoute,
    onboardingRoute,
    profilesRoute,
    newProfileRoute,
    profileRoute,
    billsRoute,
    newBillRoute,
    billRoute,
    editBillRoute,
    settingsRoute,
    scanRoute,
    analyticsRoute,
    searchRoute,
    notificationsRoute,
  ]),
])

export function makeRouter(queryClient: QueryClient) {
  return createRouter({
    routeTree,
    context: { queryClient },
    defaultPreload: 'intent',
    scrollRestoration: true,
  })
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof makeRouter>
  }
}
