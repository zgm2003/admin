import { defineStore } from 'pinia'

import type { AccessCredential, CurrentUser } from '../api/auth.contract'

export type AuthStatus = 'unknown' | 'anonymous' | 'authenticated' | 'error'

interface AuthState {
  status: AuthStatus
  accessToken: string
  accessExpiresAt: number
  user: CurrentUser | null
  errorMessage: string
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    status: 'unknown',
    accessToken: '',
    accessExpiresAt: 0,
    user: null,
    errorMessage: '',
  }),
  actions: {
    setCredential(credential: AccessCredential, nowMilliseconds = Date.now()) {
      this.accessToken = credential.accessToken
      this.accessExpiresAt = nowMilliseconds + credential.expiresIn * 1_000
      this.errorMessage = ''
    },
    setAuthenticated(user: CurrentUser) {
      this.user = user
      this.status = 'authenticated'
      this.errorMessage = ''
    },
    setAnonymous() {
      this.clearAuthValues()
      this.status = 'anonymous'
    },
    setError(message: string) {
      this.clearAuthValues()
      this.status = 'error'
      this.errorMessage = message
    },
    clearAuthValues() {
      this.accessToken = ''
      this.accessExpiresAt = 0
      this.user = null
      this.errorMessage = ''
    },
  },
})
