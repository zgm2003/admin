import { defineConfig, type ViteUserConfig } from 'vitest/config'
import { loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'

export function createViteConfig(mode: string): ViteUserConfig {
  const env = loadEnv(mode, process.cwd(), '')

  return {
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
    optimizeDeps: {
      // Lazy pages and Element Plus auto-imported styles must not restart the dev page.
      noDiscovery: true,
      include: [
        '@element-plus/icons-vue',
        '@wangeditor-next/editor-for-vue',
        'axios',
        'echarts/charts',
        'echarts/components',
        'echarts/core',
        'echarts/renderers',
        'element-plus',
        'element-plus/es',
        'go-captcha-vue',
        'lucide-vue-next',
        'pinia',
        'vue',
        'vue-i18n',
        'vue-router',
      ],
    },
    server: {
      host: 'localhost',
      port: 16300,
      strictPort: true,
      open: true,
      proxy: {
        '/api': {
          target: env.VITE_API_BASE_URL,
          changeOrigin: true,
          ws: true,
        },
      },
    },
    test: {
      environment: 'jsdom',
      include: ['tests/**/*.{test,spec}.{ts,tsx,js,jsx}', 'vite.config.test.ts'],
      pool: 'threads',
      maxWorkers: 1,
      fileParallelism: false,
      testTimeout: 30_000,
    },
  }
}

export default defineConfig(({ mode }) => createViteConfig(mode))
