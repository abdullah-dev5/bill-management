/// <reference types="vitest/config" />
import { fileURLToPath, URL } from 'node:url'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    // Installable app + service worker (M5). Off in development so tests and HMR see fresh files.
    VitePWA({
      strategies: 'injectManifest',
      srcDir: 'src/sw',
      filename: 'sw.ts',
      registerType: 'autoUpdate',
      injectRegister: false,
      // The on-device OCR engine (~14 MB) is cached on first use, not precached on install.
      injectManifest: { globPatterns: ['**/*.{js,css,html,svg,png,woff2}'], globIgnores: ['tesseract/**'] },
      manifest: {
        name: 'Bill Manager',
        short_name: 'Bills',
        description: 'Every household bill in one place: scan, track, pay on time.',
        start_url: '/',
        scope: '/',
        display: 'standalone',
        orientation: 'portrait',
        background_color: '#ECEBE8',
        theme_color: '#2D2D2D',
        categories: ['finance', 'productivity'],
        icons: [
          { src: '/icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: '/icon-512.png', sizes: '512x512', type: 'image/png' },
          { src: '/icon-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
        shortcuts: [
          { name: 'Scan a bill', short_name: 'Scan', url: '/?add=scan', icons: [{ src: '/icon-192.png', sizes: '192x192' }] },
          { name: 'Notifications', url: '/notifications', icons: [{ src: '/icon-192.png', sizes: '192x192' }] },
        ],
      },
      devOptions: { enabled: false },
    }),
  ],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    port: 5173,
    // Same-origin API in development, mirroring the Cloudflare Pages proxy in production.
    proxy: { '/api': 'http://localhost:8080' },
    // Playwright writes traces and screenshots here; watching them reloads open pages mid-test.
    watch: { ignored: ['**/e2e/.results/**', '**/e2e/.screenshots/**', '**/playwright-report/**'] },
  },
  // `vite preview` serves the production build (with the service worker) against the local API.
  preview: { port: 4173, proxy: { '/api': 'http://localhost:8080' } },
  test: {
    environment: 'jsdom',
    globals: true,
    include: ['src/**/*.test.{ts,tsx}'],
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
})
