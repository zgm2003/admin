import { useEffect } from 'react'
import { Outlet } from 'react-router-dom'
import { useLocation, useNavigate } from 'react-router-dom'

import { AgentPanel } from '@/components/agent/agent-panel'
import { AnalyticsTracker } from '@/components/layout/analytics-tracker'
import { AppTopNav } from '@/components/layout/app-top-nav'
import { useAuthStore } from '@/store/auth'

export default function Layout() {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const authenticated = useAuthStore((state) => state.authenticated)
  const isLogin = pathname === '/login'

  useEffect(() => {
    if (!isLogin && !authenticated) navigate('/login', { replace: true })
  }, [authenticated, isLogin, navigate])

  if (isLogin) return <Outlet />
  if (!authenticated) return null

  return (
    <div className="flex h-dvh overflow-hidden bg-background text-foreground">
      <AnalyticsTracker />
      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <AppTopNav />
        <div className="min-h-0 flex-1 overflow-hidden">
          <Outlet />
        </div>
      </div>
      <AgentPanel />
    </div>
  )
}
