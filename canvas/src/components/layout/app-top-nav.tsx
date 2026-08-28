import { BookOpen, Bot, Menu, Settings2 } from 'lucide-react'
import { Button, Tooltip } from 'antd'
import { Link, useLocation } from 'react-router-dom'
import { useState } from 'react'
import { navigationTools, type NavigationToolSlug } from '@/constant/navigation-tools'
import { MobileNavDrawer } from './mobile-nav-drawer'
import { AnimatedThemeToggler } from '@/components/ui/animated-theme-toggler'
import { setLocale } from '@/i18n'
import { useTranslation } from 'react-i18next'
import { GithubOutlined } from '@ant-design/icons'

export function AppTopNav() {
  const { pathname } = useLocation()
  const { i18n } = useTranslation()
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const slug = pathname.split('/').filter(Boolean)[0]
  const activeToolSlug = navigationTools.some((tool) => tool.slug === slug) ? slug as NavigationToolSlug : undefined
  return <>
    <header className="sticky top-0 z-20 h-14 shrink-0 border-b border-stone-200 bg-background/90 backdrop-blur-xl dark:border-stone-800">
      <div className="mx-auto flex h-full max-w-7xl items-stretch justify-between gap-5 px-6">
        <div className="flex min-w-0 items-center">
          <Link to="/" className="flex h-full shrink-0 items-center gap-2 text-sm font-semibold leading-none tracking-tight text-stone-950 transition hover:text-stone-600 dark:text-stone-100 dark:hover:text-stone-300"><span className="size-5 shrink-0 bg-current" style={{ mask: "url(/logo.svg) center / contain no-repeat", WebkitMask: "url(/logo.svg) center / contain no-repeat" }} /><span className="text-base font-medium">无限画布</span></Link>
          <button type="button" className="ml-3 inline-flex size-8 shrink-0 items-center justify-center text-stone-600 transition hover:text-stone-950 md:hidden dark:text-stone-300 dark:hover:text-white" onClick={() => setMobileNavOpen(true)} aria-label="打开导航菜单"><Menu className="size-5" /></button>
          <nav className="hide-scrollbar ml-8 hidden h-14 min-w-0 items-center gap-7 overflow-x-auto md:flex">{navigationTools.map((tool) => { const Icon = tool.icon; const active = tool.slug === activeToolSlug; return <Link key={tool.slug} to={`/${tool.slug}`} className={`relative flex h-14 shrink-0 items-center gap-2 text-sm leading-6 transition after:absolute after:inset-x-0 after:bottom-0 after:h-px ${active ? 'font-medium text-stone-950 after:bg-stone-950 dark:text-stone-100 dark:after:bg-stone-100' : 'text-stone-500 after:bg-transparent hover:text-stone-950 dark:text-stone-400 dark:hover:text-stone-100'}`}><Icon className="size-4" /><span className="truncate">{tool.label}</span></Link> })}</nav>
        </div>
        <div className="my-auto flex h-9 min-w-0 items-center justify-end gap-2 justify-self-end whitespace-nowrap"><Tooltip title="打开 Agent"><Button type="text" shape="circle" className="!h-8 !w-8 !min-w-8" icon={<Bot className="size-4" />} /></Tooltip><a className="natural-icon" href="#docs" title="文档" aria-label="文档"><BookOpen className="size-4" /></a><button className="natural-icon" title="配置" aria-label="配置"><Settings2 className="size-4" /></button><button className="natural-icon text-[11px] font-semibold tracking-tight" onClick={() => setLocale(i18n.language === 'zh-CN' ? 'en-US' : 'zh-CN')} aria-label="切换语言">{i18n.language === 'zh-CN' ? '中' : 'EN'}</button><AnimatedThemeToggler /><span className="px-1 text-xs font-medium text-stone-500">v0.1.0</span><a className="natural-icon" href="https://github.com/basketikun/infinite-canvas" target="_blank" rel="noreferrer" title="GitHub" aria-label="GitHub"><GithubOutlined className="text-base" /></a></div>
      </div>
    </header>
    <MobileNavDrawer open={mobileNavOpen} active={activeToolSlug} onClose={() => setMobileNavOpen(false)} />
  </>
}
