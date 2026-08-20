import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAuthStore } from './auth'

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
    const user = { userId: 1, username: 'admin', email: 'admin@example.com' }
    store.setCredential({ accessToken: 'jwt', expiresIn: 900 }, 1_000)
    store.setAuthenticated(user)
    expect(store.status).toBe('authenticated')
    expect(store.user).toEqual(user)
  })

  it('clears every auth value when anonymous or failed', () => {
    const store = useAuthStore()
    store.setCredential({ accessToken: 'jwt', expiresIn: 900 }, 1_000)
    store.setAuthenticated({ userId: 1, username: 'admin', email: 'admin@example.com' })
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

	it('updates only the authenticated current username', () => {
		const store = useAuthStore()
		expect(store.updateUsername(7, 'ignored')).toBe(false)
		store.setCredential({ accessToken: 'jwt', expiresIn: 900 }, 1_000)
		store.setAuthenticated({ userId: 7, username: 'old', email: 'user@example.com' })
		expect(store.updateUsername(7, 'new')).toBe(true)
		expect(store.user).toEqual({ userId: 7, username: 'new', email: 'user@example.com' })
		expect(store.updateUsername(8, 'ignored')).toBe(false)
		expect(store.user?.username).toBe('new')
		expect(store.accessToken).toBe('jwt')
		expect(store.accessExpiresAt).toBe(901_000)
		expect(store.status).toBe('authenticated')
	})
})
