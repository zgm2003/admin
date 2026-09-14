import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'

export default defineConfig({
  plugins: [
    vue(),
    Components({
      dts: false,
      dirs: ['src/components'],
      resolvers: [
        ElementPlusResolver({ importStyle: process.env.NODE_ENV === 'test' ? false : 'css' }),
      ],
    }),
  ],
  resolve: {
    alias: {
      '@': resolve(process.cwd(), 'src'),
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
    pool: 'threads',
    maxWorkers: 1,
    fileParallelism: false,
  },
})
