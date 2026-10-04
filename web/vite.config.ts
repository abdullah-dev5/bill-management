/// <reference types="vitest/config" />
import { fileURLToPath, URL } from 'node:url'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
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
  test: {
    environment: 'jsdom',
    globals: true,
    include: ['src/**/*.test.{ts,tsx}'],
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
})
