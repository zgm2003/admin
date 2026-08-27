import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAuthStore } from '@src/store/auth'

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('starts unknown and stores an in-memory credential', () => {
    const store = useAuthStore()
    expect(store.status).toBe('unknown')
    store.setCredential({ accessToken: 'jwt', expiresIn: 900 }, 1_000)
    expect(store.accessToken).toBe('jwt')
    expect(store.accessExpiresAt).toBe(901_000)
    expect(store.status).toBe('unknown')
  })

  it('becomes authenticated only after applying the current user', () => {
    const store = useAuthStore()
    const user = { userId: 1, username: 'admin', email: 'admin@example.com', phone: null }
    store.setCredential({ accessToken: 'jwt', expiresIn: 900 }, 1_000)
    store.setAuthenticated(user)
    expect(store.status).toBe('authenticated')
    expect(store.user).toEqual(user)
  })

  it('clears every auth value when anonymous or failed', () => {
    const store = useAuthStore()
    store.setCredential({ accessToken: 'jwt', expiresIn: 900 }, 1_000)
    store.setAuthenticated({ userId: 1, username: 'admin', email: 'admin@example.com', phone: null })
    store.setAnonymous()
    expect(store.status).toBe('anonymous')
    expect(store.accessToken).toBe('')
    expect(store.accessExpiresAt).toBe(0)
    expect(store.user).toBeNull()

    store.setCredential({ accessToken: 'jwt-2', expiresIn: 900 }, 2_000)
    store.setError('服务暂未就绪')
    expect(store.status).toBe('error')
    expect(store.errorMessage).toBe('服务暂未就绪')
    expect(store.accessToken).toBe('')
    expect(store.accessExpiresAt).toBe(0)
    expect(store.user).toBeNull()
  })

	it('updates the authenticated current user profile atomically', () => {
		const store = useAuthStore()
		expect(store.updateProfile(7, 'ignored', '+86 138-0000-0000')).toBe(false)
		store.setCredential({ accessToken: 'jwt', expiresIn: 900 }, 1_000)
		store.setAuthenticated({ userId: 7, username: 'old', email: 'user@example.com', phone: null })
		expect(store.updateProfile(7, 'new', '+86 138-0000-0000')).toBe(true)
		expect(store.user).toEqual({ userId: 7, username: 'new', email: 'user@example.com', phone: '+86 138-0000-0000' })
		expect(store.updateProfile(8, 'ignored', null)).toBe(false)
		expect(store.user?.username).toBe('new')
		expect(store.accessToken).toBe('jwt')
		expect(store.accessExpiresAt).toBe(901_000)
		expect(store.status).toBe('authenticated')
	})
})
