import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: 'localhost',
    port: 16300,
    strictPort: true,
    open: true,
  },
  test: {
    environment: 'jsdom',
  },
})
