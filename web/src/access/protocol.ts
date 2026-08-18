import { menuIcons, type MenuIconKey } from './menu-icons'
import { routeViews } from './route-views'

export function isMenuIconKey(value: string): value is MenuIconKey {
  return Object.prototype.hasOwnProperty.call(menuIcons, value)
}

export function hasRouteViewKey(value: string): boolean {
  return Object.prototype.hasOwnProperty.call(routeViews, value)
}
