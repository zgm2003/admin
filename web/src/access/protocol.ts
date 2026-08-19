import { menuIcons, type MenuIconKey } from './menu-icons'
import { routeViews, type RouteViewKey } from './route-views'

export function isMenuIconKey(value: string): value is MenuIconKey {
  return Object.prototype.hasOwnProperty.call(menuIcons, value)
}

export function hasRouteViewKey(value: string): value is RouteViewKey {
  return Object.prototype.hasOwnProperty.call(routeViews, value)
}
