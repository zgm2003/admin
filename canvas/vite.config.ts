import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { '@': resolve(__dirname, 'src') },
  },
  server: {
    host: 'localhost',
    port: 16302,
    strictPort: true,
    proxy: {
      '/api': {
        target: process.env.VITE_API_BASE_URL ?? 'http://localhost:16301',
        changeOrigin: true,
        // The API currently has the Admin local origin in its allow-list. The
        // proxy keeps local Canvas requests same-origin in the browser while
        // preserving that explicit server-side origin check.
        headers: { Origin: process.env.VITE_AUTH_ORIGIN ?? 'http://localhost:16300' },
        ws: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./tests/setup.ts'],
    include: ['tests/**/*.{test,spec}.{ts,tsx}'],
  },
})
