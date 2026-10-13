import type { RouteObject } from 'react-router-dom'
import type { PermissionMenuNode } from '@/api/permission/permission'
import type { PermissionState } from '@/store/permission'
import { ProfilePage } from '@/views/user/profile'
import { WorkbenchPage } from '@/views/canvas/workspace'

export function getPermissionRoutes(access: PermissionState): RouteObject[] {
  if (access.status !== 'ready') return []
  const routes: RouteObject[] = []
  const visit = (nodes: PermissionMenuNode[]) => {
    for (const node of nodes) {
      if (node.menuType === 'directory') { visit(node.children); continue }
      if (!access.permissionCodes.includes(node.code)) continue
      if (node.componentPath === 'user/profile' && node.code === 'user:profile:view' && node.path === '/user/profile') routes.push({ path: node.path, element: <ProfilePage /> })
      if (node.componentPath === 'canvas/workspace' && node.code === 'canvas:workspace:view' && node.path === '/canvas/workspace') routes.push({ path: node.path, element: <WorkbenchPage /> })
    }
  }
  visit(access.menuTree)
  return routes
}
