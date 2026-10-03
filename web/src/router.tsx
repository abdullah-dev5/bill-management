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
  validateSearch: (s: Record<string, unknown>): { tab?: 'bills' | 'settings' } => ({
    tab: s.tab === 'bills' || s.tab === 'settings' ? s.tab : undefined,
  }),
})

const billsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/bills',
  component: BillsPage,
  validateSearch: (s: Record<string, unknown>): { status?: string; profileId?: string; archived?: true } => ({
    status: str(s.status),
    profileId: str(s.profileId),
    archived: s.archived === true || s.archived === 'true' ? true : undefined,
  }),
})
const newBillRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/bills/new',
  component: NewBillPage,
  validateSearch: (s: Record<string, unknown>): { profileId?: string } => ({ profileId: str(s.profileId) }),
})
const billRoute = createRoute({ getParentRoute: () => appRoute, path: '/bills/$billId', component: BillDetailPage })
const editBillRoute = createRoute({ getParentRoute: () => appRoute, path: '/bills/$billId/edit', component: EditBillPage })

const settingsRoute = createRoute({ getParentRoute: () => appRoute, path: '/settings', component: SettingsPage })

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
