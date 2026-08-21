import {
  Cpu,
  Folder,
  Key,
  List,
  Menu as MenuIcon,
  Setting,
  User,
  UserFilled,
} from '@element-plus/icons-vue'

export const menuIcons = {
  Cpu,
  Folder,
  Key,
  List,
  Menu: MenuIcon,
  Setting,
  User,
  UserFilled,
} as const

export type MenuIconKey = keyof typeof menuIcons
