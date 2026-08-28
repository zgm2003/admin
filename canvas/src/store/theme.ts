import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type ThemeName = 'light' | 'dark'
interface ThemeState { theme: ThemeName; setTheme: (theme: ThemeName) => void }
export const useThemeStore = create<ThemeState>()(persist((set) => ({ theme: 'dark', setTheme: (theme) => set({ theme }) }), { name: 'infinite-canvas:theme_store' }))
