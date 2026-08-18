import {
  Cpu,
  Folder,
  Key,
  Menu as MenuIcon,
  Setting,
  User,
} from '@element-plus/icons-vue'

export const menuIcons = {
  Cpu,
  Folder,
  Key,
  Menu: MenuIcon,
  Setting,
  User,
} as const

export type MenuIconKey = keyof typeof menuIcons
