import type { Component } from 'vue'
import type { MenuIconName } from '@/icons/menu-icons'

interface AppDIconSharedProps {
  size?: string | number
  color?: string
}

export type AppDIconProps = AppDIconSharedProps &
  (
    | {
        component: Component
        icon?: never
      }
    | {
        component?: never
        icon: MenuIconName
      }
  )
