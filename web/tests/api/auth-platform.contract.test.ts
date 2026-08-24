import { describe, expect, it } from 'vitest'

import { YesNo } from '@src/enums/yes-no'
import { ProtocolError } from '@src/types/http'
import {
  parseAuthPlatformDeployment,
  parseAuthPlatformIDResult,
  parseAuthPlatformPage,
  parseAuthPlatformStatusResult,
  parseEmptyAuthPlatformResult,
} from '@src/api/auth-platform.contract'

const timestamp = '2026-08-20T10:00:00Z'

function row(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: 2,
    code: 'admin',
    name: 'Admin',
    policyVersion: 1,
    accessTTLSeconds: 900,
    refreshTTLSeconds: 86_400,
    sessionCacheTTLSeconds: 7_200,
    accessCacheTTLSeconds: 600,
    bindDevice: YesNo.Yes,
    bindIP: YesNo.No,
    maxSessions: 1,
    allowRegister: YesNo.No,
    isEnabled: YesNo.Yes,
    isBuiltin: YesNo.Yes,
    createdAt: timestamp,
    updatedAt: timestamp,
    ...overrides,
  }
}

describe('authentication platform contracts', () => {
  it('parses strict platform pages and deployment status', () => {
    expect(parseAuthPlatformPage({ list: [row()], total: 1, page: 1, pageSize: 20 })).toEqual({
      list: [{ ...row() }], total: 1, page: 1, pageSize: 20,
    })
    expect(parseAuthPlatformDeployment({
      cookieSecure: false, corsOrigin: 'http://localhost:16300',
      trustedProxyMode: 'none', trustedProxyCount: 0, redisStatus: 'up',
    })).toEqual({
      cookieSecure: false, corsOrigin: 'http://localhost:16300',
      trustedProxyMode: 'none', trustedProxyCount: 0, redisStatus: 'up',
    })
    expect(parseAuthPlatformIDResult({ id: 3 })).toEqual({ id: 3 })
    expect(parseAuthPlatformStatusResult({ id: 3, isEnabled: YesNo.No })).toEqual({ id: 3, isEnabled: YesNo.No })
    expect(parseEmptyAuthPlatformResult({})).toEqual({})
  })

  it.each([
    { list: [row({ id: 1 }), row({ id: 2 })], total: 2, page: 1, pageSize: 20 },
    { list: [row(), row({ id: 2, code: 'other' })], total: 2, page: 1, pageSize: 20 },
    { list: [row({ code: 'Admin' })], total: 1, page: 1, pageSize: 20 },
    { list: [row({ bindDevice: 2 })], total: 1, page: 1, pageSize: 20 },
    { list: [row({ accessTTLSeconds: 59 })], total: 1, page: 1, pageSize: 20 },
    { list: [row({ maxSessions: 101 })], total: 1, page: 1, pageSize: 20 },
    { list: [row({ createdAt: 'not-a-time' })], total: 1, page: 1, pageSize: 20 },
    { list: [row({ secret: 'forbidden' })], total: 1, page: 1, pageSize: 20 },
    { list: [row()], total: 0, page: 1, pageSize: 20 },
  ])('rejects invalid platform pages: %j', (value) => {
    expect(() => parseAuthPlatformPage(value)).toThrow(ProtocolError)
  })

  it.each([
    null,
    { cookieSecure: false, corsOrigin: '', trustedProxyMode: 'none', trustedProxyCount: 0, redisStatus: 'up' },
    { cookieSecure: false, corsOrigin: 'http://localhost', trustedProxyMode: 'bad', trustedProxyCount: 0, redisStatus: 'up' },
    { cookieSecure: false, corsOrigin: 'http://localhost', trustedProxyMode: 'none', trustedProxyCount: -1, redisStatus: 'up' },
    { cookieSecure: false, corsOrigin: 'http://localhost', trustedProxyMode: 'none', trustedProxyCount: 0, redisStatus: 'down', secret: 'bad' },
  ])('rejects invalid deployment status: %j', (value) => {
    expect(() => parseAuthPlatformDeployment(value)).toThrow(ProtocolError)
  })
})
