import { describe, expect, it } from 'vitest'

import { createViteConfig } from './vite.config.ts'

const config = createViteConfig('test')

describe('Vite development server', () => {
  it('uses the fixed local port and opens the browser', () => {
    expect(config.server).toMatchObject({
      host: 'localhost',
      port: 16300,
      strictPort: true,
      open: true,
    })
  })

  it('proxies same-origin API requests to the Go server', () => {
    expect(config.server?.proxy).toMatchObject({
      '/api': {
        target: 'http://localhost:16301',
        changeOrigin: true,
      },
    })
  })

  it('maps @ to the src root', () => {
    expect(config.resolve?.alias).toMatchObject({ '@': expect.any(String) })
  })

  it('does not rediscover dependencies while loading lazy pages', () => {
    expect(config.optimizeDeps).toMatchObject({
      noDiscovery: true,
      include: expect.arrayContaining([
        'axios',
        'element-plus',
        'element-plus/es',
        '@element-plus/icons-vue',
        '@wangeditor-next/editor-for-vue',
        'echarts/core',
        'go-captcha-vue',
        'lucide-vue-next',
        'pinia',
        'vue',
        'vue-i18n',
        'vue-router',
      ]),
    })
  })

  it('bounds jsdom suites and runs them in one worker to avoid resource-driven timeouts', () => {
    expect(config.test).toMatchObject({
      pool: 'threads',
      maxWorkers: 1,
      fileParallelism: false,
      testTimeout: 30_000,
    })
  })
})
