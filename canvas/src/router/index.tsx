import { lazy, Suspense, type ReactNode } from 'react'
import { createBrowserRouter } from 'react-router-dom'

import Layout from '@/layout'

const HomePage = lazy(() => import('@/views/home'))
const LoginPage = lazy(() => import('@/views/login'))
const StaticPlaceholderPage = lazy(() => import('@/views/static-placeholder'))

function LazyPage({ children }: { children: ReactNode }) {
  return <Suspense fallback={null}>{children}</Suspense>
}

export const router = createBrowserRouter([
  {
    element: <Layout />,
    children: [
      { index: true, element: <LazyPage><HomePage /></LazyPage> },
      { path: 'login', element: <LazyPage><LoginPage /></LazyPage> },
      { path: 'canvas', element: <LazyPage><StaticPlaceholderPage /></LazyPage> },
      { path: 'image', element: <LazyPage><StaticPlaceholderPage /></LazyPage> },
      { path: 'video', element: <LazyPage><StaticPlaceholderPage /></LazyPage> },
      { path: 'prompts', element: <LazyPage><StaticPlaceholderPage /></LazyPage> },
      { path: 'assets', element: <LazyPage><StaticPlaceholderPage /></LazyPage> },
      { path: 'config', element: <LazyPage><StaticPlaceholderPage /></LazyPage> },
      { path: '*', element: <LazyPage><StaticPlaceholderPage /></LazyPage> },
    ],
  },
])
