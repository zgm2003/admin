import { defineStore } from 'pinia'

import type { AccessCredential, CurrentUser } from '@/api/auth/login'

export type AuthStatus = 'unknown' | 'anonymous' | 'authenticated' | 'error'

interface AuthState {
  status: AuthStatus
  accessToken: string
  accessExpiresAt: number
  isNewUser: boolean
  passwordSetRequired: boolean
  user: CurrentUser | null
  errorMessage: string
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    status: 'unknown',
    accessToken: '',
    accessExpiresAt: 0,
    isNewUser: false,
    passwordSetRequired: false,
    user: null,
    errorMessage: '',
  }),
  actions: {
    setCredential(credential: AccessCredential, nowMilliseconds = Date.now()) {
      this.accessToken = credential.accessToken
      this.accessExpiresAt = nowMilliseconds + credential.expiresIn * 1_000
      this.isNewUser = credential.isNewUser ?? false
      this.passwordSetRequired = credential.passwordSetRequired ?? false
      this.errorMessage = ''
    },
    setAuthenticated(user: CurrentUser) {
      this.user = user
      this.passwordSetRequired = user.passwordSetRequired
      this.status = 'authenticated'
      this.errorMessage = ''
    },
    markPasswordSet() {
      this.passwordSetRequired = false
      if (this.user !== null) this.user = { ...this.user, passwordSetRequired: false }
    },
    updateProfile(
      userId: number,
      username: string,
      phone: string | null,
      avatar?: string,
    ): boolean {
      if (this.user === null || this.user.userId !== userId) return false
      this.user = {
        userId: this.user.userId,
        username,
        email: this.user.email,
        phone,
        avatar: avatar ?? this.user.avatar,
        passwordSetRequired: this.user.passwordSetRequired,
      }
      return true
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
      this.isNewUser = false
      this.passwordSetRequired = false
      this.user = null
      this.errorMessage = ''
    },
  },
})
