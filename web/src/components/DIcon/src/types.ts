import type { Component } from 'vue'
import type { MenuIconName } from '../../../icons/menu-icons'

interface DIconSharedProps {
  size?: string | number
  color?: string
}

export type DIconProps = DIconSharedProps & ({
  component: Component
  icon?: never
} | {
  component?: never
  icon: MenuIconName
})
