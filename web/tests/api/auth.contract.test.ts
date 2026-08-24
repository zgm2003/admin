import { describe, expect, it } from 'vitest'

import { ProtocolError } from '@src/types/http'
import { parseAuthPolicy, parseCredential, parseCurrentUser, parseRegisteredUser } from '@src/api/auth.contract'

describe('auth contracts', () => {
  it('parses each exact DTO', () => {
    expect(parseCredential({ accessToken: 'jwt', expiresIn: 900 })).toEqual({
      accessToken: 'jwt',
      expiresIn: 900,
    })
    const user = { userId: 1, username: 'admin', email: 'admin@example.com' }
    expect(parseRegisteredUser(user)).toEqual(user)
    expect(parseCurrentUser(user)).toEqual(user)
    expect(parseAuthPolicy({ code: 'admin', name: 'Admin', allowRegister: 1 })).toEqual({
      code: 'admin', name: 'Admin', allowRegister: 1,
    })
  })

  it.each([
    null,
    {},
    { code: 'other', name: 'Admin', allowRegister: 1 },
    { code: 'admin', name: '  ', allowRegister: 1 },
    { code: 'admin', name: 'Admin', allowRegister: 2 },
    { code: 'admin', name: 'Admin', allowRegister: 1, extra: true },
  ])('rejects invalid authentication policies: %j', (value) => {
    expect(() => parseAuthPolicy(value)).toThrow(ProtocolError)
  })

  it.each([
    null,
    {},
    { accessToken: '', expiresIn: 900 },
    { accessToken: 'jwt', expiresIn: 0 },
    { accessToken: 'jwt', expiresIn: 1.5 },
    { accessToken: 'jwt', expiresIn: 900, refreshToken: 'forbidden' },
  ])('rejects invalid credentials: %j', (value) => {
    expect(() => parseCredential(value)).toThrow(ProtocolError)
  })

  it.each([
    {},
    { userId: 0, username: 'admin', email: 'admin@example.com' },
    { userId: 1, username: '', email: 'admin@example.com' },
    { userId: 1, username: 'admin', email: 1 },
    { userId: 1, username: 'admin', email: 'admin@example.com', role: 'admin' },
  ])('rejects invalid user DTOs: %j', (value) => {
    expect(() => parseRegisteredUser(value)).toThrow(ProtocolError)
    expect(() => parseCurrentUser(value)).toThrow(ProtocolError)
  })
})
