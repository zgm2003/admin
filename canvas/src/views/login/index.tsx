import { ArrowRight, KeyRound, Sparkles } from 'lucide-react'
import { FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'

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
    <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] [background-size:16px_16px] dark:bg-[radial-gradient(rgba(245,245,244,.18)_1px,transparent_1px)]" />
    <div className="relative mx-auto grid min-h-screen max-w-6xl items-center gap-16 px-6 py-12 lg:grid-cols-[1fr_420px] lg:px-12">
      <section className="hidden lg:block">
        <div className="mt-28 max-w-xl">
          <p className="mb-5 flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-stone-500 dark:text-stone-400"><Sparkles className="size-4 text-orange-500" /> 创作工作台</p>
          <h1 className="text-6xl font-semibold leading-[1.08] tracking-tight sm:text-7xl">把灵感<br /><span className="text-stone-400 dark:text-stone-500">放回画布。</span></h1>
          <p className="mt-8 max-w-md text-base leading-7 text-stone-500 dark:text-stone-400">登录后继续整理你的画布、提示词和视觉资产，让每一次推演都有迹可循。</p>
        </div>
      </section>
      <section className="mx-auto w-full max-w-[420px]">
        <div className="border border-stone-200 bg-white/80 p-8 shadow-[0_24px_80px_rgba(28,25,23,.08)] backdrop-blur-xl dark:border-stone-800 dark:bg-stone-950/70 dark:shadow-black/30 sm:p-10">
          <div className="mb-8"><p className="text-xs font-medium uppercase tracking-[0.16em] text-stone-500 dark:text-stone-400">Welcome back</p><h2 className="mt-3 text-2xl font-semibold tracking-tight">登录你的工作台</h2><p className="mt-2 text-sm leading-6 text-stone-500 dark:text-stone-400">继续你的无限画布创作。</p></div>
          <form className="space-y-5" onSubmit={handleSubmit}>
            <label className="block text-sm font-medium"><span className="mb-2 block text-stone-700 dark:text-stone-300">邮箱</span><input required type="email" placeholder="you@example.com" className="h-11 w-full border border-stone-300 bg-transparent px-3 text-sm outline-none transition placeholder:text-stone-400 focus:border-stone-950 dark:border-stone-700 dark:focus:border-stone-200" /></label>
            <label className="block text-sm font-medium"><span className="mb-2 block text-stone-700 dark:text-stone-300">密码</span><input required type="password" placeholder="输入密码" className="h-11 w-full border border-stone-300 bg-transparent px-3 text-sm outline-none transition placeholder:text-stone-400 focus:border-stone-950 dark:border-stone-700 dark:focus:border-stone-200" /></label>
            <div className="flex items-center justify-between text-xs text-stone-500 dark:text-stone-400"><label className="inline-flex items-center gap-2"><input type="checkbox" className="size-3.5 accent-stone-950 dark:accent-stone-100" />记住我</label><button type="button" className="transition hover:text-stone-950 dark:hover:text-stone-100">忘记密码？</button></div>
            <button type="submit" className="group flex h-11 w-full items-center justify-center gap-2 bg-stone-950 text-sm font-medium text-white transition hover:bg-stone-800 dark:bg-stone-100 dark:text-stone-950 dark:hover:bg-white">进入画布 <ArrowRight className="size-4 transition-transform group-hover:translate-x-0.5" /></button>
          </form>
          <div className="my-7 flex items-center gap-3 text-xs text-stone-400"><span className="h-px flex-1 bg-stone-200 dark:bg-stone-800" />或<span className="h-px flex-1 bg-stone-200 dark:bg-stone-800" /></div>
          <button type="button" className="flex h-11 w-full items-center justify-center gap-2 border border-stone-300 text-sm font-medium transition hover:bg-stone-50 dark:border-stone-700 dark:hover:bg-stone-900"><KeyRound className="size-4" />使用访问密钥</button>
          <p className="mt-7 text-center text-xs leading-5 text-stone-400">首次使用？ <Link to="/" className="text-stone-700 underline underline-offset-4 dark:text-stone-200">先浏览画布</Link></p>
        </div>
      </section>
    </div>
  </main>
}
