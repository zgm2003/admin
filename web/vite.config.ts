import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@src': resolve(process.cwd(), 'src'),
    },
  },
  server: {
    host: 'localhost',
    port: 16300,
    strictPort: true,
    open: true,
  },
  test: {
    environment: 'jsdom',
    include: ['tests/**/*.{test,spec}.{ts,tsx,js,jsx}'],
  },
})
