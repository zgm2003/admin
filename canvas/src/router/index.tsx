import { useEffect, useSyncExternalStore } from 'react'
import { createBrowserRouter, Navigate, Outlet, RouterProvider, useLocation } from 'react-router-dom'

import { LoginPage } from '@/views/auth/login'
import { WorkbenchPage } from '@/views/canvas/workspace'
import { getPermissionRoutes } from './permissionRoutes'
import { authStore, permissionStore } from '@/store'

function AuthBootstrap() {
  const snapshot = useSyncExternalStore(authStore.subscribe.bind(authStore), authStore.getSnapshot, authStore.getSnapshot)
  useEffect(() => {
    if (snapshot.status === 'unknown') void authStore.bootstrap().catch(() => undefined)
  }, [snapshot.status])
  return <Outlet />
}

function ProtectedRoute() {
  const snapshot = useSyncExternalStore(authStore.subscribe.bind(authStore), authStore.getSnapshot, authStore.getSnapshot)
  const access = useSyncExternalStore(permissionStore.subscribe.bind(permissionStore), permissionStore.getSnapshot, permissionStore.getSnapshot)
  useEffect(() => {
    if (snapshot.status === 'authenticated' && access.status === 'idle') void permissionStore.load().catch(() => undefined)
  }, [snapshot.status, access.status])
  if (snapshot.status === 'unknown' || snapshot.status === 'loading') {
    return <div className="route-loading" aria-label="Loading" />
  }
  if (snapshot.status !== 'authenticated') return <Navigate to="/login" replace />
  return <Outlet />
}

function PermissionRoutes() {
  const access = useSyncExternalStore(permissionStore.subscribe.bind(permissionStore), permissionStore.getSnapshot, permissionStore.getSnapshot)
  const location = useLocation()
  const route = getPermissionRoutes(access).find(candidate => candidate.path === location.pathname)
  if (access.status === 'idle' || access.status === 'loading') return <div className="route-loading" aria-label="Loading" />
  if (access.status === 'error') return <section className="profile-page"><div className="profile-card"><p role="alert">{access.error}</p><button type="button" onClick={() => void permissionStore.load(true).catch(() => undefined)}>重新加载权限</button></div></section>
  return route?.element ?? <Navigate to="/" replace />
}

function PublicRoute() {
  const snapshot = useSyncExternalStore(authStore.subscribe.bind(authStore), authStore.getSnapshot, authStore.getSnapshot)
  if (snapshot.status === 'authenticated') return <Navigate to="/" replace />
  return <Outlet />
}

const router = createBrowserRouter([
  {
    element: <AuthBootstrap />,
    children: [
      {
        element: <PublicRoute />,
        children: [{ path: '/login', element: <LoginPage /> }],
      },
      {
        element: <ProtectedRoute />,
        children: [
          { path: '/', element: <WorkbenchPage /> },
          { path: '*', element: <PermissionRoutes /> },
        ],
      },
      { path: '*', element: <Navigate to="/" replace /> },
    ],
  },
])

export function CanvasRouter() {
  return <RouterProvider router={router} />
}
