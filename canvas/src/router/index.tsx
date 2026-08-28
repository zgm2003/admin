import { createBrowserRouter } from 'react-router-dom'
import Layout from '@/layout'
import HomePage from '@/views/home'

export const router = createBrowserRouter([
  { element: <Layout />, children: [{ index: true, element: <HomePage /> }, { path: 'canvas', element: <HomePage /> }, { path: 'image', element: <HomePage /> }, { path: 'video', element: <HomePage /> }, { path: 'prompts', element: <HomePage /> }, { path: 'assets', element: <HomePage /> }, { path: 'config', element: <HomePage /> }, { path: '*', element: <HomePage /> }] },
])
