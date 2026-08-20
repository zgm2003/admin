import { describe, expect, it } from 'vitest'

import { YesNo } from '../enums/yes-no'
import { ProtocolError } from '../types/http'
import {
  parseEmptyUserResult,
  parseUpdatedUsername,
  parseUserPage,
  parseUserRoleOptions,
  parseUserRoleResult,
  parseUserRoles,
  parseUserStatusResult,
} from './user.contract'

describe('user contract', () => {
  it('parses every exact user response', () => {
    const page = userPage()
    expect(parseUserPage(page)).toEqual(page)
    expect(parseUserRoleOptions({ roles: page.list[0].roles })).toEqual({ roles: page.list[0].roles })
    expect(parseUpdatedUsername({ id: 7, username: 'alice', updatedAt: '2026-08-20T00:00:00Z' })).toEqual({ id: 7, username: 'alice', updatedAt: '2026-08-20T00:00:00Z' })
    expect(parseUserStatusResult({ id: 7, isEnabled: YesNo.No })).toEqual({ id: 7, isEnabled: YesNo.No })
    expect(parseEmptyUserResult({})).toEqual({})
    expect(parseUserRoles(userRoles())).toEqual(userRoles())
    expect(parseUserRoleResult({ id: 7, roleCount: 2 })).toEqual({ id: 7, roleCount: 2 })
  })

  it('rejects malformed pages, nested roles, ordering, and duplicates', () => {
    const page = userPage()
    const row = page.list[0]
    const invalid: unknown[] = [
      null, [], { ...page, extra: true }, { ...page, page: 0 }, { ...page, total: -1 },
      { ...page, list: null }, { ...page, list: [{ ...row, id: 0 }] },
      { ...page, list: [{ ...row, isEnabled: 2 }] }, { ...page, list: [{ ...row, roles: [] }] },
      { ...page, list: [{ ...row, roles: [...row.roles].reverse() }] },
      { ...page, list: [row, { ...row }] }, { ...page, list: [{ ...row, unexpected: true }] },
      { ...page, list: [{ ...row, createdAt: 'not-time' }] },
    ]
    for (const value of invalid) expect(() => parseUserPage(value)).toThrow(ProtocolError)
  })

  it('rejects malformed role options, selections, and mutation results', () => {
    const roles = userPage().list[0].roles
    const valid = userRoles()
    const invalidParsers: Array<() => unknown> = [
      () => parseUserRoleOptions({ roles: [roles[0], roles[0]] }),
      () => parseUserRoleOptions({ roles: [...roles].reverse() }),
      () => parseUpdatedUsername({ id: 7, username: ' alice ', updatedAt: '2026-08-20T00:00:00Z' }),
      () => parseUserStatusResult({ id: 7, isEnabled: 2 }),
      () => parseEmptyUserResult({ id: 7 }),
      () => parseUserRoles({ ...valid, roleIds: [2, 2] }),
      () => parseUserRoles({ ...valid, roleIds: [99] }),
      () => parseUserRoles({ ...valid, roles: [] }),
      () => parseUserRoleResult({ id: 7, roleCount: -1 }),
    ]
    for (const parse of invalidParsers) expect(parse).toThrow(ProtocolError)
  })
})

function userPage() {
  return {
    list: [{
      id: 7, username: 'alice', email: 'alice@example.com', isEnabled: YesNo.Yes,
      roles: [
        { id: 3, code: 'ai_tester', name: 'AI Tester', isEnabled: YesNo.No },
        { id: 2, code: 'member', name: 'Member', isEnabled: YesNo.Yes },
      ].sort((left, right) => left.code.localeCompare(right.code) || left.id - right.id),
      createdAt: '2026-08-20T00:00:00Z', updatedAt: '2026-08-20T01:00:00.123Z',
    }],
    total: 1, page: 1, pageSize: 20,
  }
}

function userRoles() {
  const row = userPage().list[0]
  return {
    user: { id: row.id, username: row.username, email: row.email, isEnabled: row.isEnabled },
    roles: row.roles,
    roleIds: [2, 3],
  }
}
