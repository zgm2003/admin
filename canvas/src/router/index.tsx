import { useEffect, useSyncExternalStore } from 'react'
import { createBrowserRouter, Navigate, Outlet, RouterProvider } from 'react-router-dom'

import { LoginPage } from '@/views/auth/login'
import { WorkbenchPage } from '@/views/workbench'
import { authStore } from '@/store'

function AuthBootstrap() {
  const snapshot = useSyncExternalStore(authStore.subscribe.bind(authStore), authStore.getSnapshot, authStore.getSnapshot)
  useEffect(() => {
    if (snapshot.status === 'unknown') void authStore.bootstrap()
  }, [snapshot.status])
  return <Outlet />
}

function ProtectedRoute() {
  const snapshot = useSyncExternalStore(authStore.subscribe.bind(authStore), authStore.getSnapshot, authStore.getSnapshot)
  if (snapshot.status === 'unknown' || snapshot.status === 'loading') {
    return <div className="route-loading" aria-label="Loading" />
  }
  if (snapshot.status !== 'authenticated') return <Navigate to="/login" replace />
  return <Outlet />
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
        children: [{ path: '/', element: <WorkbenchPage /> }],
      },
      { path: '*', element: <Navigate to="/" replace /> },
    ],
  },
])

export function CanvasRouter() {
  return <RouterProvider router={router} />
}
