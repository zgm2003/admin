import { describe, expect, it } from 'vitest'

import config from './vite.config.ts'

describe('Vite development server', () => {
  it('uses the fixed local port and opens the browser', () => {
    expect(config.server).toMatchObject({
      host: 'localhost',
      port: 16300,
      strictPort: true,
      open: true,
    })
  })

  it('maps @ to the src root', () => {
    expect(config.resolve?.alias).toMatchObject({ '@': expect.any(String) })
  })

  it('runs jsdom suites in one worker to avoid resource-driven timeouts', () => {
    expect(config.test).toMatchObject({
      pool: 'threads',
      maxWorkers: 1,
      fileParallelism: false,
    })
  })
})
