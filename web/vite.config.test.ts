import { afterEach, describe, expect, it, vi } from 'vitest'

import { createViteConfig } from './vite.config.ts'

const config = createViteConfig('test')

describe('Vite development server', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('uses the fixed local port and opens the browser', () => {
    expect(config.server).toMatchObject({
      host: 'localhost',
      port: 16300,
      strictPort: true,
      open: true,
    })
  })

  it('proxies same-origin API requests to the Go server', () => {
    vi.stubEnv('API_PROXY_TARGET', 'http://127.0.0.1:16401')
    const proxy = createViteConfig('test').server?.proxy
    expect(proxy).toEqual({
      '/api': {
        target: 'http://127.0.0.1:16401',
        changeOrigin: true,
        ws: true,
      },
    })
  })

  it.each([
    '',
    '   ',
    'localhost:16301',
    'ws://localhost:16301',
    'http://localhost:16301/api',
    'http://localhost:16301?target=api',
    'http://localhost:16301#api',
    'http://user:password@localhost:16301',
  ])('rejects an invalid development proxy origin: %j', (target) => {
    vi.stubEnv('API_PROXY_TARGET', target)
    expect(() => createViteConfig('test')).toThrow(/API_PROXY_TARGET/)
  })

  it('accepts an HTTPS upstream origin', () => {
    vi.stubEnv('API_PROXY_TARGET', 'https://localhost:16301/')
    expect(createViteConfig('test').server?.proxy?.['/api']).toMatchObject({
      target: 'https://localhost:16301',
      ws: true,
    })
  })

  it('builds production assets without a development upstream', () => {
    vi.stubEnv('API_PROXY_TARGET', '')
    expect(createViteConfig('production', 'build').server?.proxy).toBeUndefined()
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
