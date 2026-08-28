import { AppTopNav } from '@/components/layout/app-top-nav'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useEffect } from 'react'

import { useAuthStore } from '@/store/auth'

export default function Layout() {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const authenticated = useAuthStore((state) => state.authenticated)
  const isLogin = pathname === '/login'

  useEffect(() => {
    if (!isLogin && !authenticated) navigate('/login', { replace: true })
  }, [authenticated, isLogin, navigate])

  if (!isLogin && !authenticated) return null
  return (
    <div className="canvas-shell">
      {isLogin ? null : <AppTopNav />}
      <div className="canvas-content"><Outlet /></div>
    </div>
  )
}
