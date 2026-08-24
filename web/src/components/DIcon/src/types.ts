import type { Component } from 'vue'

interface DIconSharedProps {
  size?: string | number
  color?: string
}

export type DIconProps = DIconSharedProps & ({
  component: Component
  icon?: never
} | {
  component?: never
  icon: string
})
