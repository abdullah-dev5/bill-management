import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Toaster } from 'sonner'
import { ApiError } from '@/api/client'
import '@/i18n'
import '@/index.css'
import { applyTheme } from '@/lib/theme'
import { makeRouter } from '@/router'

applyTheme()

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      // Don't retry client errors (401/403/404/…): they won't fix themselves.
      retry: (count, err) => !(err instanceof ApiError && err.status < 500) && count < 2,
    },
  },
})

const router = makeRouter(queryClient)

// A 401 anywhere means the session ended: send the user to sign in.
queryClient.getQueryCache().subscribe((event) => {
  const err = event.query.state.error
  if (event.type === 'updated' && err instanceof ApiError && err.status === 401 && event.query.queryKey[0] !== 'me') {
    queryClient.clear()
    void router.navigate({ to: '/sign-in', search: { redirect: router.state.location.href } })
  }
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
      <Toaster
        position="bottom-center"
        toastOptions={{ className: 'font-sans' }}
        offset={24}
        // Clear the bottom navigation on phones.
        mobileOffset={{ bottom: 'calc(88px + env(safe-area-inset-bottom))' }}
      />
    </QueryClientProvider>
  </StrictMode>,
)
