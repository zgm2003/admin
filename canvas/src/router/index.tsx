import { lazy, Suspense, type ReactNode } from 'react'
import { createBrowserRouter } from 'react-router-dom'

import Layout from '@/layout'

const HomePage = lazy(() => import('@/pages/home'))
const LoginPage = lazy(() => import('@/views/login'))
const CanvasPage = lazy(() => import('@/pages/canvas'))
const CanvasProjectPage = lazy(() => import('@/pages/canvas/project'))
const ImagePage = lazy(() => import('@/pages/image'))
const VideoPage = lazy(() => import('@/pages/video'))
const PromptsPage = lazy(() => import('@/pages/prompts'))
const AssetsPage = lazy(() => import('@/pages/assets'))
const ConfigPage = lazy(() => import('@/pages/config'))
const NotFoundPage = lazy(() => import('@/pages/not-found'))

function LazyPage({ children }: { children: ReactNode }) {
  return <Suspense fallback={null}>{children}</Suspense>
}

export const router = createBrowserRouter([
  {
    element: <Layout />,
    children: [
      { index: true, element: <LazyPage><HomePage /></LazyPage> },
      { path: 'login', element: <LazyPage><LoginPage /></LazyPage> },
      { path: 'canvas', element: <LazyPage><CanvasPage /></LazyPage> },
      { path: 'canvas/:id', element: <LazyPage><CanvasProjectPage /></LazyPage> },
      { path: 'image', element: <LazyPage><ImagePage /></LazyPage> },
      { path: 'video', element: <LazyPage><VideoPage /></LazyPage> },
      { path: 'prompts', element: <LazyPage><PromptsPage /></LazyPage> },
      { path: 'assets', element: <LazyPage><AssetsPage /></LazyPage> },
      { path: 'config', element: <LazyPage><ConfigPage /></LazyPage> },
      { path: '*', element: <LazyPage><NotFoundPage /></LazyPage> },
    ],
  },
])
