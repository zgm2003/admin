import { isMenuIconName, type MenuIconName } from '../icons/menu-icons'

export const menuI18nKeyPattern = /^[a-z][a-z0-9]*(?:\.[a-z][a-zA-Z0-9]*)+$/
export const menuCodePattern = /^[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?::[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$/
export const menuPathPattern = /^\/[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:\/[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$/
export const componentPathPattern = /^[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:\/[a-z][a-z0-9]*(?:-[a-z0-9]+)*)*$/
const staticPaths: ReadonlySet<string> = new Set(['/login', '/register', '/dashboard'])

export function isMenuI18nKey(value: string): boolean {
  return value.length <= 128 && menuI18nKeyPattern.test(value)
}

export function isMenuPath(value: string): boolean {
  return value.length <= 255 && !staticPaths.has(value) && menuPathPattern.test(value)
}

export function isComponentPath(value: string): boolean {
  return value.length <= 255 && componentPathPattern.test(value)
}

export function isMenuIcon(value: string): value is MenuIconName {
  return isMenuIconName(value)
}
