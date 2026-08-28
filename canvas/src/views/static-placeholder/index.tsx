import { useLocation } from 'react-router-dom'

export default function StaticPlaceholderPage() {
  const { pathname } = useLocation()
  return <main className="grid h-full place-items-center bg-background text-sm text-stone-500 dark:text-stone-400">
    <span>{pathname === '/canvas' ? '画布即将开放' : '页面即将开放'}</span>
  </main>
}
