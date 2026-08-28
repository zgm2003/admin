import { Moon, Sun } from 'lucide-react'
import { flushSync } from 'react-dom'
import { useRef } from 'react'
import { useThemeStore } from '@/store/theme'

export function AnimatedThemeToggler() {
  const buttonRef = useRef<HTMLButtonElement>(null)
  const current = useThemeStore((state) => state.theme)
  const setTheme = useThemeStore((state) => state.setTheme)
  const toggle = () => {
    const next = current === 'dark' ? 'light' : 'dark'
    const button = buttonRef.current
    const apply = () => setTheme(next)
    if (button === null || typeof document.startViewTransition !== 'function') { apply(); return }
    const rect = button.getBoundingClientRect()
    const x = rect.left + rect.width / 2
    const y = rect.top + rect.height / 2
    const radius = Math.hypot(Math.max(x, innerWidth - x), Math.max(y, innerHeight - y))
    const transition = document.startViewTransition(() => flushSync(apply))
    void transition.ready.then(() => document.documentElement.animate(
      { clipPath: [`circle(0 at ${x}px ${y}px)`, `circle(${radius}px at ${x}px ${y}px)`] },
      { duration: 400, easing: 'ease-in-out', pseudoElement: '::view-transition-new(root)' },
    ))
  }
  return <button ref={buttonRef} type="button" className="top-icon-button" onClick={toggle} title={current === 'dark' ? '切换到浅色主题' : '切换到深色主题'}>{current === 'dark' ? <Sun size={16} /> : <Moon size={16} />}</button>
}
