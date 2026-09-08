import { ArrowRight, Sparkles } from 'lucide-react'
import { FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'

import logoUrl from '@/assets/logo.png'
import { useAuthStore } from '@/store/auth'

export default function LoginPage() {
  const navigate = useNavigate()
  const setAuthenticated = useAuthStore((state) => state.setAuthenticated)

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setAuthenticated(true)
    navigate('/')
  }

  return <main className="relative min-h-full overflow-hidden bg-background text-stone-950 dark:text-stone-100">
    <div className="pointer-events-none absolute inset-0">
      <div className="absolute inset-0 bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] [background-size:22px_22px] dark:bg-[radial-gradient(rgba(245,245,244,.14)_1px,transparent_1px)]" />
      <div className="canvas-login-orb absolute -top-28 right-[-8%] size-[460px] rounded-full bg-amber-300/40 blur-3xl dark:bg-amber-500/15" />
      <div className="canvas-login-orb canvas-login-orb--slow absolute bottom-[-14%] left-[-6%] size-[400px] rounded-full bg-orange-400/25 blur-3xl dark:bg-orange-500/10" />
    </div>

    <div className="relative mx-auto grid min-h-screen max-w-6xl items-center gap-14 px-6 py-12 lg:grid-cols-[1fr_420px] lg:px-12">
      <section className="hidden lg:block">
        <div className="max-w-xl">
          <div className="mb-9 flex items-center gap-3 animate-in fade-in-0 slide-in-from-left-3 duration-500">
            <img src={logoUrl} alt="智澜" className="h-11 w-auto object-contain" />
            <p className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-stone-500 dark:text-stone-400"><Sparkles className="size-4 text-amber-500" /> 创作工作台</p>
          </div>
          <h1 className="text-6xl font-semibold leading-[1.08] tracking-tight animate-in fade-in-0 slide-in-from-bottom-4 duration-700 sm:text-7xl">把灵感<br /><span className="ai-title-aurora">放回画布。</span></h1>
          <p className="mt-8 max-w-md text-base leading-7 text-stone-500 animate-in fade-in-0 slide-in-from-bottom-4 duration-700 delay-150 dark:text-stone-400">登录后继续整理你的画布、提示词和视觉资产，让每一次推演都有迹可循。</p>
        </div>
      </section>

      <section className="mx-auto w-full max-w-[420px] animate-in fade-in-0 slide-in-from-bottom-5 duration-700 delay-200">
        <div className="rounded-2xl border border-stone-200/80 bg-white/80 p-8 shadow-[0_24px_80px_rgba(28,25,23,.10)] backdrop-blur-xl dark:border-stone-800 dark:bg-stone-950/70 dark:shadow-black/30 sm:p-10">
          <div className="mb-7 flex items-center gap-3 lg:hidden">
            <img src={logoUrl} alt="智澜" className="h-9 w-auto object-contain" />
            <p className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.16em] text-stone-500 dark:text-stone-400"><Sparkles className="size-4 text-amber-500" /> 创作工作台</p>
          </div>
          <div className="mb-8"><p className="text-xs font-medium uppercase tracking-[0.16em] text-stone-500 dark:text-stone-400">Welcome back</p><h2 className="mt-3 text-2xl font-semibold tracking-tight">登录你的工作台</h2><p className="mt-2 text-sm leading-6 text-stone-500 dark:text-stone-400">继续你的无限画布创作。</p></div>
          <form className="space-y-5" onSubmit={handleSubmit}>
            <label className="block text-sm font-medium"><span className="mb-2 block text-stone-700 dark:text-stone-300">邮箱</span><input required type="email" placeholder="you@example.com" className="h-11 w-full rounded-lg border border-stone-300 bg-transparent px-3 text-sm outline-none transition placeholder:text-stone-400 focus:border-amber-500 focus:ring-2 focus:ring-amber-500/20 dark:border-stone-700 dark:focus:border-amber-400" /></label>
            <label className="block text-sm font-medium"><span className="mb-2 block text-stone-700 dark:text-stone-300">密码</span><input required type="password" placeholder="输入密码" className="h-11 w-full rounded-lg border border-stone-300 bg-transparent px-3 text-sm outline-none transition placeholder:text-stone-400 focus:border-amber-500 focus:ring-2 focus:ring-amber-500/20 dark:border-stone-700 dark:focus:border-amber-400" /></label>
            <div className="flex items-center justify-between text-xs text-stone-500 dark:text-stone-400"><label className="inline-flex items-center gap-2"><input type="checkbox" className="size-3.5 accent-amber-500" />记住我</label><button type="button" className="transition hover:text-stone-950 dark:hover:text-stone-100">忘记密码？</button></div>
            <button type="submit" className="group flex h-11 w-full items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-amber-500 to-orange-500 text-sm font-semibold !text-white shadow-[0_10px_24px_rgba(245,158,11,.35)] transition hover:-translate-y-0.5 hover:shadow-[0_14px_30px_rgba(245,158,11,.45)] active:translate-y-0">进入画布 <ArrowRight className="size-4 transition-transform group-hover:translate-x-0.5" /></button>
          </form>
          <p className="mt-7 text-center text-xs leading-5 text-stone-400">首次使用？ <Link to="/" className="text-stone-700 underline underline-offset-4 dark:text-stone-200">先浏览画布</Link></p>
        </div>
      </section>
    </div>
  </main>
}
