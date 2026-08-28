import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type AuthState = {
  authenticated: boolean
  setAuthenticated: (authenticated: boolean) => void
}

export const useAuthStore = create<AuthState>()(persist((set) => ({
  authenticated: false,
  setAuthenticated: (authenticated) => set({ authenticated }),
}), { name: 'infinite-canvas:auth_store' }))
