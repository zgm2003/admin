import { AppTopNav } from '@/components/layout/app-top-nav'
import { Outlet } from 'react-router-dom'

export default function Layout() {
  return <div className="canvas-shell"><AppTopNav /><div className="canvas-content"><Outlet /></div></div>
}
