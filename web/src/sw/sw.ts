/// <reference lib="webworker" />
// Service worker (docs/07, M5): offline app shell, last-seen data while offline, and Web Push.
import { clientsClaim } from 'workbox-core'
import { ExpirationPlugin } from 'workbox-expiration'
import { cleanupOutdatedCaches, createHandlerBoundToURL, precacheAndRoute } from 'workbox-precaching'
import { NavigationRoute, registerRoute } from 'workbox-routing'
import { CacheFirst, NetworkFirst } from 'workbox-strategies'

declare const self: ServiceWorkerGlobalScope & { __WB_MANIFEST: (string | { url: string; revision: string | null })[] }

export const API_CACHE = 'api-v1'

self.skipWaiting()
clientsClaim()

// The built app (JS, CSS, fonts, icons) is precached: the app opens with no connection.
precacheAndRoute(self.__WB_MANIFEST)
cleanupOutdatedCaches()

// Every in-app page is the single-page app shell.
registerRoute(new NavigationRoute(createHandlerBoundToURL('/index.html'), { denylist: [/^\/api\//, /^\/internal\//] }))

// Read-only API data the screens need: fresh when online, the last copy when offline.
// Files, exports and anything that changes data are never cached.
const CACHEABLE = /^\/api\/v1\/(auth\/me|household|home|profiles|bills|types|providers|notifications|activity|analytics)(\/|\?|$)/
registerRoute(
  ({ url, request }) => request.method === 'GET' && url.origin === self.location.origin && CACHEABLE.test(url.pathname + url.search) && !url.pathname.includes('/file'),
  new NetworkFirst({
    cacheName: API_CACHE,
    networkTimeoutSeconds: 4,
    plugins: [new ExpirationPlugin({ maxEntries: 150, maxAgeSeconds: 7 * 24 * 3600 })],
  }),
)

// The on-device OCR engine and model: downloaded once, then read from the cache (works offline).
registerRoute(
  ({ url }) => url.origin === self.location.origin && url.pathname.startsWith('/tesseract/'),
  new CacheFirst({ cacheName: 'ocr-engine-v1', plugins: [new ExpirationPlugin({ maxEntries: 10 })] }),
)

// Signing out (or in as someone else) clears the cached data.
self.addEventListener('message', (event) => {
  if (event.data?.type === 'clear-data') event.waitUntil(caches.delete(API_CACHE))
})

// --- Web Push (FR-REM-03) ----------------------------------------------------------

interface PushPayload {
  title: string
  body: string
  url?: string
  tag?: string
}

self.addEventListener('push', (event) => {
  let data: PushPayload = { title: 'Bill Manager', body: '' }
  try {
    data = { ...data, ...(event.data?.json() as PushPayload) }
  } catch {
    data.body = event.data?.text() ?? ''
  }
  event.waitUntil(
    self.registration.showNotification(data.title, {
      body: data.body,
      tag: data.tag,
      icon: '/icon-192.png',
      badge: '/icon-192.png',
      data: { url: data.url ?? '/notifications' },
    }),
  )
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = new URL((event.notification.data as { url?: string })?.url ?? '/notifications', self.location.origin).href
  event.waitUntil(
    (async () => {
      const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
      const open = windows.find((w) => new URL(w.url).origin === self.location.origin)
      if (open) {
        await open.focus()
        return open.navigate(url)
      }
      return self.clients.openWindow(url)
    })(),
  )
})
