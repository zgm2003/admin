import type { LucideIcon } from 'lucide-react'

interface DIconProps {
  icon: LucideIcon
  size?: number
}

export function DIcon({ icon: Icon, size = 18 }: DIconProps) {
  return <Icon size={size} strokeWidth={1.8} aria-hidden="true" />
}
