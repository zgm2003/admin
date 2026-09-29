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
    build: {
      rollupOptions: {
        output: {
          manualChunks(id) {
            const normalizedId = id.replaceAll('\\\\', '/')
            if (!normalizedId.includes('/node_modules/')) return undefined

            if (
              normalizedId.includes('/vue/') ||
              normalizedId.includes('/@vue/') ||
              normalizedId.includes('/vue-router/') ||
              normalizedId.includes('/pinia/') ||
              normalizedId.includes('/vue-i18n/')
            ) {
              return 'vue-core'
            }
            const elementComponent = normalizedId.match(
              /\/element-plus\/es\/components\/([^/]+)/,
            )?.[1]
            if (elementComponent !== undefined) {
              const group = elementPlusChunkGroups[elementComponent] ?? 'core'
              return `element-plus-${group}`
            }
            if (
              normalizedId.includes('/element-plus/') ||
              normalizedId.includes('/@element-plus/')
            ) {
              return 'element-plus-core'
            }
            if (normalizedId.includes('/echarts/')) return 'echarts'
            if (normalizedId.includes('/@wangeditor-next/editor-for-vue/')) return 'rich-editor-vue'
            if (normalizedId.includes('/@wangeditor-next/editor/')) return 'rich-editor-core'
            if (normalizedId.includes('/@wangeditor-next/')) return 'rich-editor-modules'
            if (
              normalizedId.includes('/axios/') ||
              normalizedId.includes('/lucide-vue-next/') ||
              normalizedId.includes('/go-captcha-vue/')
            ) {
              return 'utility-vendor'
            }
            return 'vendor'
          },
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

const elementPlusChunkGroups: Readonly<Record<string, string>> = {
  aside: 'layout',
  breadcrumb: 'navigation',
  card: 'layout',
  checkbox: 'form',
  'checkbox-button': 'form',
  'checkbox-group': 'form',
  col: 'layout',
  container: 'layout',
  'date-picker': 'form',
  'date-picker-panel': 'form',
  dialog: 'overlay',
  drawer: 'overlay',
  dropdown: 'overlay',
  'dropdown-item': 'overlay',
  'dropdown-menu': 'overlay',
  form: 'form',
  'form-item': 'form',
  header: 'layout',
  input: 'form',
  'input-number': 'form',
  loading: 'overlay',
  main: 'layout',
  menu: 'navigation',
  'menu-item': 'navigation',
  'menu-item-group': 'navigation',
  message: 'overlay',
  'message-box': 'overlay',
  notification: 'overlay',
  pagination: 'data',
  popconfirm: 'overlay',
  popover: 'overlay',
  radio: 'form',
  'radio-button': 'form',
  'radio-group': 'form',
  row: 'layout',
  scrollbar: 'layout',
  select: 'form',
  'select-v2': 'form',
  space: 'layout',
  switch: 'form',
  'tab-pane': 'navigation',
  table: 'data',
  'table-column': 'data',
  tabs: 'navigation',
  tag: 'data',
  tooltip: 'overlay',
  tree: 'data',
  upload: 'form',
}

export default defineConfig(({ mode }) => createViteConfig(mode))
