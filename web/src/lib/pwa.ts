/** Service worker registration and messages (M5). The worker only exists in production builds. */

export function registerServiceWorker() {
  if (!import.meta.env.PROD || !('serviceWorker' in navigator)) return
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js', { scope: '/' }).catch(() => {
      // No worker means no offline mode; the app still works online.
    })
  })
}

/** Drops the offline copy of household data (on sign-out, and before someone else signs in). */
export function clearOfflineData() {
  navigator.serviceWorker?.controller?.postMessage({ type: 'clear-data' })
}

// --- Install (A2HS) ---------------------------------------------------------------

interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

let installEvent: BeforeInstallPromptEvent | null = null
const installListeners = new Set<() => void>()

if (typeof window !== 'undefined') {
  window.addEventListener('beforeinstallprompt', (e) => {
    e.preventDefault() // we show our own button
    installEvent = e as BeforeInstallPromptEvent
    installListeners.forEach((l) => l())
  })
  window.addEventListener('appinstalled', () => {
    installEvent = null
    installListeners.forEach((l) => l())
  })
}

export type InstallState = 'installed' | 'available' | 'ios' | 'unavailable'

export function installState(): InstallState {
  if (window.matchMedia?.('(display-mode: standalone)').matches || (navigator as { standalone?: boolean }).standalone) return 'installed'
  if (installEvent) return 'available'
  if (/iphone|ipad|ipod/i.test(navigator.userAgent)) return 'ios'
  return 'unavailable'
}

export function onInstallChange(fn: () => void) {
  installListeners.add(fn)
  return () => installListeners.delete(fn)
}

export async function promptInstall(): Promise<boolean> {
  if (!installEvent) return false
  await installEvent.prompt()
  const { outcome } = await installEvent.userChoice
  installEvent = null
  installListeners.forEach((l) => l())
  return outcome === 'accepted'
}

// --- Web Push (FR-REM-03) -----------------------------------------------------------

export const pushSupported = () => 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window

function base64UrlToBytes(s: string): Uint8Array<ArrayBuffer> {
  const pad = '='.repeat((4 - (s.length % 4)) % 4)
  const raw = atob((s + pad).replace(/-/g, '+').replace(/_/g, '/'))
  const out = new Uint8Array(new ArrayBuffer(raw.length))
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i)
  return out
}

async function readyRegistration(): Promise<ServiceWorkerRegistration> {
  const reg = await Promise.race([
    navigator.serviceWorker.ready,
    new Promise<never>((_, reject) => setTimeout(() => reject(new Error('no-service-worker')), 4000)),
  ])
  return reg
}

/** This device's current push subscription, if any. */
export async function currentSubscription(): Promise<PushSubscription | null> {
  if (!pushSupported()) return null
  const reg = await navigator.serviceWorker.getRegistration()
  return (await reg?.pushManager.getSubscription()) ?? null
}

/** Asks permission and subscribes this device. Throws Error('denied' | 'no-service-worker'). */
export async function subscribeDevice(publicKey: string): Promise<PushSubscriptionJSON> {
  const permission = await Notification.requestPermission()
  if (permission !== 'granted') throw new Error('denied')
  const reg = await readyRegistration()
  const sub =
    (await reg.pushManager.getSubscription()) ??
    (await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: base64UrlToBytes(publicKey) }))
  return sub.toJSON()
}
